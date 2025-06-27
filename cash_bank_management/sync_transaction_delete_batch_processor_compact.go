package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Procesador de lotes para sincronización offline de Transaction Delete Service - Versión Compacta
// Implementa procesamiento paralelo optimizado de eliminaciones masivas
// Incluye gestión de concurrencia, detección de conflictos y validación de integridad

// TransactionDeleteBatchProcessor gestiona el procesamiento por lotes de eliminaciones
// Proporciona capacidades de procesamiento paralelo y gestión de errores optimizada
type TransactionDeleteBatchProcessor struct {
	maxConcurrency    int                              // Máximo número de goroutines concurrentes
	batchSize         int                              // Tamaño de lote para procesamiento
	timeout           time.Duration                    // Timeout para operaciones individuales
	retryAttempts     int                              // Número máximo de reintentos
	validateBalance   bool                             // Si validar balance durante procesamiento
	auditEnabled      bool                             // Si habilitar auditoría detallada
	conflictStrategy  string                           // Estrategia para manejo de conflictos
	processingQueue   chan OfflineTransactionDeletion  // Cola de procesamiento
	resultChannel     chan SyncTransactionDeleteResult // Canal de resultados
	workerWg          sync.WaitGroup                   // WaitGroup para workers
	isRunning         bool                             // Estado del procesador
	mutex             sync.RWMutex                     // Mutex para acceso thread-safe
}

// BatchProcessorMetrics contiene métricas básicas del procesador de lotes
type BatchProcessorMetrics struct {
	StartTime              time.Time      `json:"start_time"`              // Tiempo de inicio del batch
	EndTime                time.Time      `json:"end_time"`                // Tiempo de finalización
	TotalOperations        int            `json:"total_operations"`        // Total de operaciones procesadas
	SuccessfulOperations   int            `json:"successful_operations"`   // Operaciones exitosas
	FailedOperations       int            `json:"failed_operations"`       // Operaciones fallidas
	ConflictOperations     int            `json:"conflict_operations"`     // Operaciones con conflicto
	ProcessingTimeMs       int64          `json:"processing_time_ms"`      // Tiempo total de procesamiento
	ErrorsByType           map[string]int `json:"errors_by_type"`          // Errores agrupados por tipo
}

// TransactionDeleteBatchConfig estructura de configuración compacta
type TransactionDeleteBatchConfig struct {
	MaxConcurrency   int    `json:"max_concurrency"`
	BatchSize        int    `json:"batch_size"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	RetryAttempts    int    `json:"retry_attempts"`
	ValidateBalance  bool   `json:"validate_balance"`
	AuditEnabled     bool   `json:"audit_enabled"`
	ConflictStrategy string `json:"conflict_strategy"`
}

// NewTransactionDeleteBatchProcessor crea un nuevo procesador compacto
func NewTransactionDeleteBatchProcessor(config TransactionDeleteBatchConfig) *TransactionDeleteBatchProcessor {
	// Set default values
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 3
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 25
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 30
	}
	if config.RetryAttempts < 0 {
		config.RetryAttempts = 1
	}

	return &TransactionDeleteBatchProcessor{
		maxConcurrency:   config.MaxConcurrency,
		batchSize:        config.BatchSize,
		timeout:          time.Duration(config.TimeoutSeconds) * time.Second,
		retryAttempts:    config.RetryAttempts,
		validateBalance:  config.ValidateBalance,
		auditEnabled:     config.AuditEnabled,
		conflictStrategy: config.ConflictStrategy,
		processingQueue:  make(chan OfflineTransactionDeletion, config.BatchSize),
		resultChannel:    make(chan SyncTransactionDeleteResult, config.BatchSize),
		isRunning:        false,
	}
}

// ProcessBatch procesa un lote completo de eliminaciones de transacciones
func (p *TransactionDeleteBatchProcessor) ProcessBatch(deletions []OfflineTransactionDeletion) (SyncTransactionDeleteBatchResponse, error) {
	p.mutex.Lock()
	if p.isRunning {
		p.mutex.Unlock()
		return SyncTransactionDeleteBatchResponse{}, fmt.Errorf("processor is already running")
	}
	p.isRunning = true
	p.mutex.Unlock()

	defer func() {
		p.mutex.Lock()
		p.isRunning = false
		p.mutex.Unlock()
	}()

	startTime := time.Now()
	response := SyncTransactionDeleteBatchResponse{
		Success:          false,
		ProcessedOps:     0,
		SuccessfulOps:    0,
		FailedOps:        0,
		Results:          make([]SyncTransactionDeleteResult, 0, len(deletions)),
		Conflicts:        make([]TransactionDeleteConflictResolution, 0),
		BalanceImpacts:   make([]TransactionDeleteBalanceImpact, 0),
		ValidationErrors: make([]string, 0),
		LastSync:         time.Now().Format(time.RFC3339),
	}

	// Start workers
	for i := 0; i < p.maxConcurrency; i++ {
		p.workerWg.Add(1)
		go p.deletionWorker(i)
	}

	// Start result collector
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go p.resultCollector(&response, &collectorWg)

	// Feed deletions to queue
	go func() {
		defer close(p.processingQueue)
		for _, deletion := range deletions {
			select {
			case p.processingQueue <- deletion:
				// Queued successfully
			case <-time.After(p.timeout):
				log.Printf("Timeout queuing deletion: %s", deletion.LocalID)
			}
		}
	}()

	// Wait for completion
	p.workerWg.Wait()
	close(p.resultChannel)
	collectorWg.Wait()

	// Calculate metrics
	processingTime := time.Since(startTime).Milliseconds()
	response.Success = response.FailedOps == 0 && len(response.Conflicts) == 0

	if response.Success {
		response.Message = fmt.Sprintf("Batch processed successfully: %d deletions in %dms", 
			response.SuccessfulOps, processingTime)
	} else {
		response.Message = fmt.Sprintf("Batch completed with %d errors, %d conflicts", 
			response.FailedOps, len(response.Conflicts))
	}

	return response, nil
}

// deletionWorker procesa eliminaciones individuales
func (p *TransactionDeleteBatchProcessor) deletionWorker(workerID int) {
	defer p.workerWg.Done()

	for deletion := range p.processingQueue {
		result := SyncTransactionDeleteResult{
			LocalID:          deletion.LocalID,
			TransactionID:    deletion.TransactionID,
			Action:           deletion.Action,
			Status:           "processing",
			SyncTimestamp:    time.Now().Format(time.RFC3339),
			ValidationPassed: false,
		}

		// Process deletion with retry
		success := p.processDeletionWithRetry(deletion, &result)
		
		if success {
			result.Status = "success"
			result.ValidationPassed = true
		} else if result.Status != "conflict" {
			result.Status = "error"
		}

		// Send result
		select {
		case p.resultChannel <- result:
			// Sent successfully
		case <-time.After(time.Second * 2):
			log.Printf("Timeout sending result from worker %d", workerID)
		}
	}
}

// processDeletionWithRetry procesa eliminación con reintentos
func (p *TransactionDeleteBatchProcessor) processDeletionWithRetry(deletion OfflineTransactionDeletion, result *SyncTransactionDeleteResult) bool {
	var lastError error

	for attempt := 0; attempt <= p.retryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}

		// Validate deletion
		if err := validateTransactionDeletionConsistency(deletion); err != nil {
			result.Error = fmt.Sprintf("Validation failed: %v", err)
			return false
		}

		// Check if can be deleted
		exists, canDelete, err := checkTransactionCanBeDeleted(deletion.TransactionID, deletion.UserID)
		if err != nil {
			lastError = err
			continue
		}

		if !exists {
			p.handleDeletionConflict(deletion, result, "not_found", "Transaction not found")
			return false
		}

		if !canDelete {
			p.handleDeletionConflict(deletion, result, "protected", "Transaction protected")
			return false
		}

		// Calculate balance impact
		balanceImpact, err := calculateBalanceImpactForDeletion(deletion.TransactionID, deletion.UserID)
		if err != nil {
			lastError = err
			continue
		}

		// Execute deletion
		err = executeTransactionDeletion(deletion.TransactionID, deletion.UserID, deletion.DeletionReason)
		if err != nil {
			lastError = err
			continue
		}

		result.BalanceAdjustment = balanceImpact.AdjustmentMade
		return true
	}

	result.Error = fmt.Sprintf("All attempts failed: %v", lastError)
	return false
}

// resultCollector recolecta resultados
func (p *TransactionDeleteBatchProcessor) resultCollector(response *SyncTransactionDeleteBatchResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	for result := range p.resultChannel {
		response.Results = append(response.Results, result)
		response.ProcessedOps++

		switch result.Status {
		case "success":
			response.SuccessfulOps++
		case "error":
			response.FailedOps++
		case "conflict":
			// Conflicts are tracked separately
		}

		if !result.ValidationPassed && result.Error != "" {
			response.ValidationErrors = append(response.ValidationErrors, result.Error)
		}
	}
}

// handleDeletionConflict maneja conflictos
func (p *TransactionDeleteBatchProcessor) handleDeletionConflict(deletion OfflineTransactionDeletion, result *SyncTransactionDeleteResult, conflictType, description string) {
	result.Status = "conflict"
	result.ConflictType = conflictType
	result.RequiresAction = true
	result.Error = description

	log.Printf("Deletion conflict: %s for transaction %s", conflictType, deletion.TransactionID)
}

// IsRunning verifica si está ejecutándose
func (p *TransactionDeleteBatchProcessor) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.isRunning
}