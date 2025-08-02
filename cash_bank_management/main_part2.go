package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Handlers HTTP principales para operaciones de efectivo y banco
// Estos handlers implementan la lógica de negocio para operaciones CRUD
// Incluyen validación de entrada, manejo de cache y respuestas estructuradas

// handleFetchDistribution maneja peticiones GET para obtener distribución de efectivo/banco
// Proporciona datos actuales de distribución con soporte de cache para optimización
// Responde con información completa incluyendo cantidades y porcentajes calculados
func handleFetchDistribution(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only GET requests allowed
	// Valida método HTTP - solo permite peticiones GET para consultas
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract and validate user ID from query parameters
	// Extrae y valida ID de usuario desde parámetros de consulta
	// El user_id es obligatorio para identificar los datos del usuario
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Try cache first for performance optimization
	// Intenta obtener datos del cache primero para optimizar rendimiento
	// Si encuentra datos en cache, los retorna inmediatamente
	if cacheManager != nil {
		var cachedDistribution CashBankDistribution
		err := cacheManager.GetCashBankData(userID, &cachedDistribution)
		if err == nil {
			log.Printf("✅ Cache HIT: cash bank distribution for user %s", userID)
			sendSuccessResponse(w, "Cash bank distribution fetched successfully from cache", cachedDistribution)
			return
		}
		log.Printf("🔍 Cache MISS: cash bank distribution for user %s", userID)
	}

	// Extract year_month parameter, default to current month if not provided
	// Extrae parámetro year_month, usa mes actual si no se proporciona
	yearMonth := r.URL.Query().Get("year_month")
	if yearMonth == "" {
		// Default to current month for backwards compatibility
		// Por defecto usa mes actual para compatibilidad
		yearMonth = time.Now().Format("2006-01")
	}
	
	// Fetch cash bank distribution data from database
	// Obtiene datos de distribución desde la base de datos
	// Incluye cálculo de porcentajes y validación de datos
	distribution, err := fetchCashBankDistribution(userID, yearMonth)
	if err != nil {
		log.Printf("Error fetching cash bank distribution: %v", err)
		sendErrorResponse(w, "Error fetching cash bank distribution", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests optimization
	// Cachea el resultado para optimizar futuras peticiones
	// Mejora significativamente el rendimiento en consultas frecuentes
	if cacheManager != nil {
		err = cacheManager.CacheCashBankData(userID, distribution)
		if err != nil {
			log.Printf("Warning: Failed to cache cash bank distribution for user %s: %v", userID, err)
		}
	}

	// Return cash bank distribution data as structured JSON response
	// Retorna datos de distribución como respuesta JSON estructurada
	sendSuccessResponse(w, "Cash bank distribution fetched successfully", distribution)
}

// handleUpdateCash maneja peticiones POST para actualizar cantidad de efectivo
// Permite modificar directamente el saldo de efectivo del usuario
// Recalcula automáticamente porcentajes y actualiza todas las tablas periódicas
func handleUpdateCash(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST requests allowed for updates
	// Valida método HTTP - solo permite POST para actualizaciones
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate JSON request body
	// Parsea y valida el cuerpo de la petición JSON
	// Estructura esperada: UpdateAmountRequest con user_id, amount y date
	var updateRequest UpdateAmountRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields in the request
	// Valida campos requeridos en la petición
	// User ID es obligatorio para identificar la cuenta
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate amount is non-negative
	// Valida que la cantidad no sea negativa
	// Permite cero para resetear saldo a cero
	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Extract year_month from date or use current month
	// Extrae year_month de la fecha o usa mes actual
	yearMonth := updateRequest.Date[:7] // "2025-05-01" -> "2025-05"
	if len(updateRequest.Date) < 7 {
		yearMonth = time.Now().Format("2006-01")
	}
	
	// Get current distribution to maintain bank amount
	// Obtiene distribución actual para mantener cantidad de banco
	// Necesario para recalcular el total y porcentajes correctamente
	distribution, err := fetchCashBankDistribution(updateRequest.UserID, yearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Update cash amount and recalculate totals
	// Actualiza cantidad de efectivo y recalcula totales
	// Mantiene la cantidad de banco intacta
	distribution.CashAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages based on new amounts
	// Recalcula porcentajes basados en las nuevas cantidades
	// Evita división por cero si el total es cero
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save the updated distribution to all period tables
	// Guarda la distribución actualizada en todas las tablas periódicas
	// Mantiene consistencia en datos diarios, semanales, mensuales, etc.
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating cash amount: %v", err)
		sendErrorResponse(w, "Error updating cash amount", http.StatusInternalServerError)
		return
	}

	// Add transaction to history for audit trail
	// Añade transacción al historial para trazabilidad
	// Importante para auditoría y seguimiento de cambios
	err = addTransaction(updateRequest.UserID, "cash_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error - no impide la operación principal
	}

	// Invalidate related caches since cash amount was updated
	// Invalida caches relacionados ya que la cantidad de efectivo cambió
	// Asegura que próximas consultas obtengan datos actualizados
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(updateRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", updateRequest.UserID, err)
		}
		
		// Also invalidate dashboard cache since cash/bank affects dashboard
		// También invalida cache del dashboard ya que efectivo/banco afecta el dashboard
		err = cacheManager.InvalidateDashboardCache(updateRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", updateRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", updateRequest.UserID)
	}

	// Return success response with updated distribution
	// Retorna respuesta exitosa con distribución actualizada
	sendSuccessResponse(w, "Cash amount updated successfully", distribution)
}

// handleUpdateBank maneja peticiones POST para actualizar cantidad de banco
// Permite modificar directamente el saldo bancario del usuario
// Funcionalidad simétrica a handleUpdateCash pero para cuentas bancarias
func handleUpdateBank(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST requests allowed
	// Valida método HTTP - solo permite POST para modificaciones
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate JSON request body
	// Parsea y valida el cuerpo de petición JSON
	var updateRequest UpdateAmountRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	// Valida campos requeridos en la petición
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate amount is non-negative
	// Valida que la cantidad sea no negativa
	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Extract year_month from date or use current month
	// Extrae year_month de la fecha o usa mes actual
	yearMonth := updateRequest.Date[:7] // "2025-05-01" -> "2025-05"
	if len(updateRequest.Date) < 7 {
		yearMonth = time.Now().Format("2006-01")
	}
	
	// Get current distribution to maintain cash amount
	// Obtiene distribución actual para mantener cantidad de efectivo
	distribution, err := fetchCashBankDistribution(updateRequest.UserID, yearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Update bank amount and recalculate totals
	// Actualiza cantidad bancaria y recalcula totales
	distribution.BankAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages based on new totals
	// Recalcula porcentajes basados en nuevos totales
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save updated distribution to database
	// Guarda distribución actualizada en base de datos
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating bank amount: %v", err)
		sendErrorResponse(w, "Error updating bank amount", http.StatusInternalServerError)
		return
	}

	// Add transaction to history for tracking
	// Añade transacción al historial para seguimiento
	err = addTransaction(updateRequest.UserID, "bank_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// Invalidate caches since bank amount was updated
	// Invalida caches ya que la cantidad bancaria cambió
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(updateRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", updateRequest.UserID, err)
		}
		
		// Also invalidate dashboard cache
		// También invalida cache del dashboard
		err = cacheManager.InvalidateDashboardCache(updateRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", updateRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", updateRequest.UserID)
	}

	// Return success response with updated data
	// Retorna respuesta exitosa con datos actualizados
	sendSuccessResponse(w, "Bank amount updated successfully", distribution)
}