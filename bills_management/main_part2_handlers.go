package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// invalidateBillsCache removes cached bills data for a specific user
// Called whenever bills are added, updated, or deleted to ensure cache consistency
func invalidateBillsCache(userID string) {
	if cacheManager != nil {
		err := cacheManager.InvalidateBillsCache(userID, "monthly", "daily", "weekly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate bills cache for user %s: %v", userID, err)
		}

		// Also invalidate dashboard cache since bills affect dashboard calculations
		err = cacheManager.InvalidateDashboardCache(userID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", userID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (bills and dashboard)", userID)
	} else {
		log.Printf("⚠️ Cache manager not available for user: %s", userID)
	}
}

// handleFetchBills maneja las solicitudes GET para obtener facturas
// Soporta filtrado por periodo y fecha para consultas específicas
func handleFetchBills(w http.ResponseWriter, r *http.Request) {
	// Validar que se proporcione user_id
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Verificar si se solicita un periodo específico
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")

	if period != "" && date != "" {
		// Try cache first for period-specific bills
		cacheKey := fmt.Sprintf("period_%s_%s", period, date)
		if cacheManager != nil {
			var cachedBills []Bill
			err := cacheManager.GetBillsData(userID, cacheKey, &cachedBills)
			if err == nil {
				log.Printf("✅ Cache HIT: bills for user %s period %s date %s", userID, period, date)
				sendSuccessResponse(w, "Bills fetched successfully from cache", cachedBills)
				return
			}
			log.Printf("🔍 Cache MISS: bills for user %s period %s date %s", userID, period, date)
		}

		// Obtener facturas para un periodo específico
		billsWithStatus, err := fetchBillsForPeriod(userID, period, date)
		if err != nil {
			sendErrorResponse(w, "Error fetching bills for period", http.StatusInternalServerError)
			return
		}
		// Convertir a formato Bill estándar
		var bills []Bill
		for _, billWithStatus := range billsWithStatus {
			bills = append(bills, convertBillWithPeriodStatusToBill(billWithStatus))
		}

		// Cache the result for future requests
		if cacheManager != nil {
			err = cacheManager.CacheBillsData(userID, cacheKey, bills)
			if err != nil {
				log.Printf("Warning: Failed to cache bills data for user %s: %v", userID, err)
			}
		}

		sendSuccessResponse(w, "Bills fetched successfully", bills)
		return
	}

	// Try cache first for all bills
	if cacheManager != nil {
		var cachedBills []Bill
		err := cacheManager.GetBillsData(userID, "all", &cachedBills)
		if err == nil {
			log.Printf("✅ Cache HIT: all bills for user %s", userID)
			sendSuccessResponse(w, "Bills fetched successfully from cache", cachedBills)
			return
		}
		log.Printf("🔍 Cache MISS: all bills for user %s", userID)
	}

	// Obtener todas las facturas del usuario
	bills, err := fetchBills(userID)
	if err != nil {
		sendErrorResponse(w, "Error fetching bills", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheBillsData(userID, "all", bills)
		if err != nil {
			log.Printf("Warning: Failed to cache bills data for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Bills fetched successfully", bills)
}

// handleAddBill maneja las solicitudes POST para crear nuevas facturas
// CORREGIDO: Valida los datos y actualiza AMBAS tablas (monthly_balance y monthly_cash_bank_balance)
func handleAddBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for bill data
	var addRequest AddBillRequest
	if err := json.NewDecoder(r.Body).Decode(&addRequest); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request parameters
	if addRequest.UserID == "" || addRequest.Name == "" || addRequest.Amount <= 0 {
		sendErrorResponse(w, "Missing required fields: user_id, name, amount", http.StatusBadRequest)
		return
	}

	// Look up server category_id based on category name and user_id
	// This ensures we use the server's category ID, not the frontend's local ID
	var serverCategoryID *int
	if addRequest.Category != "" {
		var categoryID int
		err := db.QueryRow(`
			SELECT id FROM categories
			WHERE user_id = ? AND name = ? AND type = 'expense'
			LIMIT 1
		`, addRequest.UserID, addRequest.Category).Scan(&categoryID)

		if err == nil {
			serverCategoryID = &categoryID
			log.Printf("✅ Resolved category '%s' to server category_id: %d", addRequest.Category, categoryID)
		} else if err == sql.ErrNoRows {
			log.Printf("⚠️ Category '%s' not found for user %s, bill will be created without category_id", addRequest.Category, addRequest.UserID)
		} else {
			log.Printf("❌ Error looking up category: %v", err)
		}
	}

	// Insertar factura en la base de datos
	var result sql.Result
	var err error
	var billID int64

	if addRequest.BillID > 0 {
		// Use client-provided bill ID for sync consistency
		// Use serverCategoryID (resolved from category name) instead of client's category_id
		result, err = db.Exec("INSERT INTO bills (id, user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, category_id, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?, ?)", addRequest.BillID, addRequest.UserID, addRequest.Name, addRequest.Amount, addRequest.DueDate, addRequest.Category, serverCategoryID, addRequest.Icon, addRequest.StartDate, addRequest.PaymentDay, addRequest.DurationMonths, addRequest.Regularity, addRequest.PaymentMethod)
		billID = int64(addRequest.BillID)
	} else {
		// Auto-generate bill ID (legacy behavior)
		// Use serverCategoryID (resolved from category name) instead of client's category_id
		result, err = db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, category_id, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?, ?)", addRequest.UserID, addRequest.Name, addRequest.Amount, addRequest.DueDate, addRequest.Category, serverCategoryID, addRequest.Icon, addRequest.StartDate, addRequest.PaymentDay, addRequest.DurationMonths, addRequest.Regularity, addRequest.PaymentMethod)
		if err == nil {
			billID, _ = result.LastInsertId()
		}
	}

	if err != nil {
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}

	// Always record sync operation with auto-generated operation_id
	log.Printf("Recording sync operation for bill creation with auto-generated operation_id")

	// Create sync operation data matching the bill structure
	syncData := map[string]interface{}{
		"id":              int(billID),
		"user_id":         addRequest.UserID,
		"name":            addRequest.Name,
		"amount":          addRequest.Amount,
		"due_date":        addRequest.DueDate,
		"category":        addRequest.Category,
		"icon":            addRequest.Icon,
		"start_date":      addRequest.StartDate,
		"payment_day":     addRequest.PaymentDay,
		"duration_months": addRequest.DurationMonths,
		"regularity":      addRequest.Regularity,
		"payment_method":  addRequest.PaymentMethod,
		"created_at":      time.Now().Format("2006-01-02 15:04:05"),
		"updated_at":      time.Now().Format("2006-01-02 15:04:05"),
	}

	// Add sync operation record to database with device_id and timestamp from request
	err = addSyncOperation(
		addRequest.UserID,
		addRequest.OperationID, // Use operation_id from request if provided, otherwise auto-generated
		"create",
		"bills",
		strconv.FormatInt(billID, 10),
		syncData,
		addRequest.DeviceID,  // Use device_id from request
		addRequest.Timestamp, // Use timestamp from request
	)

	if err != nil {
		log.Printf("Warning: Failed to record sync operation: %v", err)
		// Don't fail the bill creation for sync errors, just log warning
	} else {
		log.Printf("Successfully recorded sync operation for bill ID: %d", billID)
	}

	// CORREGIDO: Aplicar la factura usando lógica acumulativa
	err = addBillToCashBankBalanceCumulative(db, addRequest.UserID, addRequest.Amount, addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)
	if err != nil {
		log.Printf("Error adding bill to monthly_cash_bank_balance: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error adding bill to cash bank balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Crear registros de pago para la factura
	createBillPaymentRecords(db, int(billID), addRequest.UserID, addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)

	// Invalidate bills cache for this user since a new bill was added
	invalidateBillsCache(addRequest.UserID)

	sendSuccessResponse(w, "Bill added successfully", map[string]interface{}{
		"id": billID, "user_id": addRequest.UserID, "name": addRequest.Name, "amount": addRequest.Amount,
	})
}

// handlePayBill maneja las solicitudes POST para procesar pagos de facturas
// Utiliza la función markBillPaid para procesar el pago de manera transaccional
func handlePayBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Estructura para la solicitud de pago con parámetros de sincronización
	var req struct {
		UserID        string `json:"user_id"`
		BillID        int    `json:"bill_id"`
		YearMonth     string `json:"year_month"`
		PaymentDate   string `json:"payment_date"`
		PaymentMethod string `json:"payment_method"`
		// Sync operation parameters for incremental synchronization
		OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
		DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
		Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
	}

	// Decodificar solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validar campos requeridos
	if req.UserID == "" || req.BillID <= 0 || req.YearMonth == "" {
		sendErrorResponse(w, "Missing required fields: user_id, bill_id, year_month", http.StatusBadRequest)
		return
	}

	// Procesar pago usando la función dedicada
	paymentResponse, err := markBillPaid(db, req.BillID, req.UserID, req.YearMonth, req.PaymentDate)
	if err != nil {
		log.Printf("Error marking bill as paid: %v", err)
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Always record sync operation with auto-generated operation_id (like add/update handlers)
	log.Printf("Recording sync operation for bill payment with auto-generated operation_id")

	// Create sync operation data for bill payment
	syncData := map[string]interface{}{
		"user_id":        req.UserID,
		"bill_id":        req.BillID,
		"year_month":     req.YearMonth,
		"payment_date":   req.PaymentDate,
		"payment_method": req.PaymentMethod,
		"payment_status": "paid",
		"processed_at":   time.Now().Format("2006-01-02 15:04:05"),
	}

	log.Printf("🔄 Calling addSyncOperation with auto-generation: user=%s, action=pay, table=bills, record=%d",
		req.UserID, req.BillID)

	// Add sync operation record to database with device_id from request
	err = addSyncOperation(
		req.UserID,
		"", // Empty operation_id will trigger auto-generation
		"pay",
		"bills",
		strconv.Itoa(req.BillID),
		syncData,
		req.DeviceID, // Use device_id from request
		0,            // Timestamp will be auto-generated
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for bill payment: %v", err)
		log.Printf("❌ ERROR: Sync operation details - user_id=%s, bill_id=%d, device_id=%s",
			req.UserID, req.BillID, req.DeviceID)
		// Don't fail the bill payment for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for bill payment: bill_id=%d, year_month=%s", req.BillID, req.YearMonth)
	}

	// Invalidate bills cache for this user since bill payment status changed
	invalidateBillsCache(req.UserID)

	sendSuccessResponse(w, "Bill payment processed successfully", paymentResponse)
}

// handleDebugAddBill endpoint de depuración para probar funcionalidades
// CORREGIDO: Ahora prueba ambas funciones (monthly_balance y monthly_cash_bank_balance)
func handleDebugAddBill(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔥 DEBUG: handleDebugAddBill llamada")

	// Parámetros fijos para depuración
	userID := "14"
	amount := 100.0
	startDate := "2025-01-15T00:00:00Z"
	durationMonths := 6
	paymentMethod := "bank"

	log.Printf("🔥 DEBUG: Llamando a addBillToCashBankBalanceCumulative (lógica acumulativa)...")
	err := addBillToCashBankBalanceCumulative(db, userID, amount, startDate, durationMonths, paymentMethod)
	if err != nil {
		log.Printf("🔥 DEBUG: Error en addBillToCashBankBalanceCumulative: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error in cash_bank_balance: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("🔥 DEBUG: addBillToCashBankBalance completada sin errores")
	sendSuccessResponse(w, "Debug completed - monthly_cash_bank_balance updated", map[string]interface{}{
		"user_id":         userID,
		"amount":          amount,
		"duration_months": durationMonths,
		"payment_method":  paymentMethod,
		"tables_updated":  []string{"monthly_cash_bank_balance"},
	})
}
