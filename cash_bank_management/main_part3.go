package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// Handlers para transferencias entre efectivo y banco
// Implementan lógica de movimiento de fondos con validación de saldos
// Mantienen integridad de datos y trazabilidad de operaciones

// handleCashToBankTransfer maneja transferencias de efectivo a banco
// Valida saldo suficiente en efectivo antes de realizar la transferencia
// Actualiza ambos saldos atomicamente para mantener consistencia
func handleCashToBankTransfer(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST allowed for transfers
	// Valida método HTTP - solo POST permitido para transferencias
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate transfer request JSON
	// Parsea y valida petición de transferencia JSON
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields in transfer request
	// Valida campos requeridos en petición de transferencia
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate transfer amount is positive
	// Valida que cantidad de transferencia sea positiva
	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Get current distribution to check cash availability
	// Obtiene distribución actual para verificar disponibilidad de efectivo
	distribution, err := fetchCashBankDistribution(transferRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Check if there's enough cash to transfer
	// Verifica si hay suficiente efectivo para transferir
	// Previene transferencias que dejarían saldo negativo
	if transferRequest.Amount > distribution.CashAmount {
		sendErrorResponse(w, "Not enough cash to transfer", http.StatusBadRequest)
		return
	}

	// Update amounts atomically - subtract from cash, add to bank
	// Actualiza cantidades atomicamente - resta de efectivo, suma a banco
	distribution.CashAmount -= transferRequest.Amount
	distribution.BankAmount += transferRequest.Amount

	// Recalculate percentages after transfer
	// Recalcula porcentajes después de la transferencia
	// Total permanece igual, solo cambia la distribución
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Save the updated distribution to database
	// Guarda distribución actualizada en base de datos
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating distribution after transfer: %v", err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Add transaction to history for audit trail
	// Añade transacción al historial para trazabilidad
	err = addTransaction(transferRequest.UserID, "cash_to_bank", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error - no bloquea operación principal
	}

	// Invalidate caches since transfer affects distribution
	// Invalida caches ya que la transferencia afecta la distribución
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(transferRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", transferRequest.UserID, err)
		}
		
		// Also invalidate dashboard cache
		// También invalida cache del dashboard
		err = cacheManager.InvalidateDashboardCache(transferRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", transferRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", transferRequest.UserID)
	}

	// Return success response with updated distribution
	// Retorna respuesta exitosa con distribución actualizada
	sendSuccessResponse(w, "Cash to bank transfer successful", distribution)
}

// handleBankToCashTransfer maneja transferencias de banco a efectivo
// Funcionalidad simétrica a cash-to-bank pero en dirección opuesta
// Incluye mismas validaciones y controles de integridad
func handleBankToCashTransfer(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method
	// Valida método HTTP
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse transfer request
	// Parsea petición de transferencia
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	// Valida campos requeridos
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Get current distribution to check bank balance
	// Obtiene distribución actual para verificar saldo bancario
	distribution, err := fetchCashBankDistribution(transferRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Check if there's enough bank balance to transfer
	// Verifica si hay suficiente saldo bancario para transferir
	if transferRequest.Amount > distribution.BankAmount {
		sendErrorResponse(w, "Not enough bank balance to transfer", http.StatusBadRequest)
		return
	}

	// Update amounts - subtract from bank, add to cash
	// Actualiza cantidades - resta de banco, suma a efectivo
	distribution.BankAmount -= transferRequest.Amount
	distribution.CashAmount += transferRequest.Amount

	// Recalculate percentages
	// Recalcula porcentajes
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Save updated distribution
	// Guarda distribución actualizada
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating distribution after transfer: %v", err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	// Añade transacción al historial
	err = addTransaction(transferRequest.UserID, "bank_to_cash", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
	}

	// Invalidate caches
	// Invalida caches
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(transferRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", transferRequest.UserID, err)
		}
		
		err = cacheManager.InvalidateDashboardCache(transferRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", transferRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", transferRequest.UserID)
	}

	// Return success response
	// Retorna respuesta exitosa
	sendSuccessResponse(w, "Bank to cash transfer successful", distribution)
}

// Funciones de utilidad para respuestas HTTP estructuradas
// Proporcionan formato consistente para todas las respuestas de la API

// sendSuccessResponse envía respuesta exitosa con datos JSON estructurados
// Establece headers apropiados y serializa datos en formato ApiResponse
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	// Set appropriate content type for JSON response
	// Establece tipo de contenido apropiado para respuesta JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Encode response using standard ApiResponse structure
	// Codifica respuesta usando estructura ApiResponse estándar
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// sendErrorResponse envía respuesta de error con código de estado HTTP apropiado
// Proporciona mensajes de error consistentes y códigos de estado estándar
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	// Set JSON content type for error responses
	// Establece tipo de contenido JSON para respuestas de error
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Send structured error response without data field
	// Envía respuesta de error estructurada sin campo de datos
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}

// Función addTransaction está definida en database_functions_part2.go