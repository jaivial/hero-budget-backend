package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	
	// Estructura para la solicitud de creación de factura
	var req struct {
		UserID         string  `json:"user_id"`
		Name           string  `json:"name"`
		Amount         float64 `json:"amount"`
		DueDate        string  `json:"due_date"`
		Category       string  `json:"category"`
		Icon           string  `json:"icon"`
		StartDate      string  `json:"start_date"`
		PaymentDay     int     `json:"payment_day"`
		DurationMonths int     `json:"duration_months"`
		Regularity     string  `json:"regularity"`
		PaymentMethod  string  `json:"payment_method"`
	}
	
	// Validar datos de la solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Name == "" || req.Amount <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Insertar factura en la base de datos
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?)", req.UserID, req.Name, req.Amount, req.DueDate, req.Category, req.Icon, req.StartDate, req.PaymentDay, req.DurationMonths, req.Regularity, req.PaymentMethod)
	if err != nil {
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}
	
	// Obtener ID de la factura creada
	billID, _ := result.LastInsertId()
	
	// CORREGIDO: Aplicar la factura usando lógica acumulativa
	err = addBillToCashBankBalanceCumulative(db, req.UserID, req.Amount, req.StartDate, req.DurationMonths, req.PaymentMethod)
	if err != nil {
		log.Printf("Error adding bill to monthly_cash_bank_balance: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error adding bill to cash bank balance: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Crear registros de pago para la factura
	createBillPaymentRecords(db, int(billID), req.UserID, req.StartDate, req.DurationMonths, req.PaymentMethod)
	
	// Invalidate bills cache for this user since a new bill was added
	invalidateBillsCache(req.UserID)
	
	sendSuccessResponse(w, "Bill added successfully", map[string]interface{}{
		"id": billID, "user_id": req.UserID, "name": req.Name, "amount": req.Amount,
	})
}

// handlePayBill maneja las solicitudes POST para procesar pagos de facturas
// Utiliza la función markBillPaid para procesar el pago de manera transaccional
func handlePayBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Estructura para la solicitud de pago
	var req struct {
		UserID        string `json:"user_id"`
		BillID        int    `json:"bill_id"`
		YearMonth     string `json:"year_month"`
		PaymentDate   string `json:"payment_date"`
		PaymentMethod string `json:"payment_method"`
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
