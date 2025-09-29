package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// updateBillDurationLogic maneja cambios de duración y fecha de inicio
// Esta función es el punto de entrada principal para actualizaciones de duración de facturas
// Analiza cambios en duración y fechas de inicio, calculando diferencias entre periodos
func updateBillDurationLogic(db *sql.DB, updateData BillUpdateData) error {
	durationChanged := updateData.OldDurationMonths != updateData.NewDurationMonths
	startDateChanged := updateData.OldStartDate != updateData.NewStartDate

	// Si no hay cambios en duración ni fecha de inicio, no hacer nada
	if !durationChanged && !startDateChanged {
		return nil
	}

	// Calcular meses del periodo antiguo y nuevo
	// CORREGIDO: Usar meses consecutivos correctamente
	oldMonths, err := calculateMonthsFromDuration(updateData.OldStartDate, updateData.OldDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating old months: %v", err)
	}

	newMonths, err := calculateMonthsFromDuration(updateData.NewStartDate, updateData.NewDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating new months: %v", err)
	}

	// Identificar meses eliminados, añadidos y que permanecen
	removedMonths := findDifferenceMonths(oldMonths, newMonths)
	addedMonths := findDifferenceMonths(newMonths, oldMonths)
	remainingMonths := findCommonMonths(oldMonths, newMonths)

	// Procesar meses eliminados del periodo
	if len(removedMonths) > 0 {
		if err := processRemovedMonths(db, updateData, removedMonths); err != nil {
			return fmt.Errorf("error processing removed months: %v", err)
		}
	}

	// Procesar meses añadidos al periodo
	if len(addedMonths) > 0 {
		if err := processAddedMonths(db, updateData, addedMonths); err != nil {
			return fmt.Errorf("error processing added months: %v", err)
		}
	}

	// Procesar cambios de importe en meses que permanecen
	if len(remainingMonths) > 0 && updateData.OldAmount != updateData.NewAmount {
		amountDiff := updateData.NewAmount - updateData.OldAmount
		if err := processRemainingMonthsAmountChange(db, updateData, remainingMonths, amountDiff); err != nil {
			return fmt.Errorf("error processing amount changes: %v", err)
		}
	}

	return nil
}

// calculateMonthsFromDuration calcula meses afectados por duración
// CORREGIDO: Calcula meses consecutivos correctamente
// Si start_date es enero y duration_months es 5, devuelve: enero, febrero, marzo, abril, mayo
func calculateMonthsFromDuration(startDate string, durationMonths int) ([]string, error) {
	var parsedDate time.Time
	var err error

	// Intentar parsear diferentes formatos de fecha compatibles
	if strings.Contains(startDate, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		parsedDate, err = time.Parse("2006-01-02T15:04:05Z", startDate)
	} else {
		// Formato solo fecha: "2025-01-15"
		parsedDate, err = time.Parse("2006-01-02", startDate)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid start date %s: %v", startDate, err)
	}

	// Generar lista de meses consecutivos desde la fecha de inicio
	// CORRECCIÓN CRÍTICA: Los meses son consecutivos desde el mes de inicio
	var months []string
	for i := 0; i < durationMonths; i++ {
		// Calcular mes consecutivo sumando i meses a la fecha de inicio
		monthDate := parsedDate.AddDate(0, i, 0)
		months = append(months, monthDate.Format("2006-01"))
	}
	return months, nil
}

// findDifferenceMonths encuentra meses que están en list1 pero no en list2
// Útil para identificar meses eliminados o añadidos entre periodos
func findDifferenceMonths(list1, list2 []string) []string {
	// Crear mapa de list2 para búsqueda eficiente
	list2Map := make(map[string]bool)
	for _, month := range list2 {
		list2Map[month] = true
	}

	// Encontrar elementos de list1 no presentes en list2
	var result []string
	for _, month := range list1 {
		if !list2Map[month] {
			result = append(result, month)
		}
	}
	return result
}

// findCommonMonths encuentra meses comunes entre dos listas
// Identifica meses que permanecen entre el periodo antiguo y nuevo
func findCommonMonths(list1, list2 []string) []string {
	// Crear mapa de list1 para búsqueda eficiente
	list1Map := make(map[string]bool)
	for _, month := range list1 {
		list1Map[month] = true
	}

	// Encontrar elementos comunes
	var result []string
	for _, month := range list2 {
		if list1Map[month] {
			result = append(result, month)
		}
	}
	return result
}

// processRemovedMonths maneja meses eliminados del periodo
// Restaura balances eliminando efectos de la factura en estos meses
func processRemovedMonths(db *sql.DB, updateData BillUpdateData, removedMonths []string) error {
	// Obtener meses que tienen expenses asociados al bill
	expenseMonths := getExpenseMonths(db, updateData.BillID, updateData.UserID)

	// Procesar cada mes eliminado
	for _, month := range removedMonths {
		if expenseMonths[month] {
			// Si el mes tenía expense, restar del expense_amount y restaurar balance
			subtractExpenseAmountForBill(db, updateData.BillID, updateData.UserID, month, updateData.OldAmount)
			updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "expense", -1)
		} else {
			// Si no tenía expense, restar del bill_amount
			updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "bill", -1)
		}
		// Restaurar balance principal (liberar dinero comprometido)
		updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "main", 1)
	}
	return nil
}

// processAddedMonths maneja meses añadidos al periodo
// Aplica efectos de la factura en los nuevos meses del periodo
func processAddedMonths(db *sql.DB, updateData BillUpdateData, addedMonths []string) error {
	for _, month := range addedMonths {
		// Asegurar que existe fila para el mes en monthly_balance
		db.Exec("INSERT OR IGNORE INTO monthly_balance (user_id, year_month) VALUES (?, ?)", updateData.UserID, month)

		// Añadir al bill_amount del mes
		updateBalanceColumns(db, updateData.UserID, month, updateData.NewAmount, updateData.NewPaymentMethod, "bill", 1)

		// Comprometer dinero del balance principal
		updateBalanceColumns(db, updateData.UserID, month, updateData.NewAmount, updateData.NewPaymentMethod, "main", -1)
	}
	return nil
}

// processRemainingMonthsAmountChange maneja cambios de importe en meses existentes
// Actualiza importes en meses que permanecen en ambos periodos
func processRemainingMonthsAmountChange(db *sql.DB, updateData BillUpdateData, remainingMonths []string, amountDiff float64) error {
	// Obtener meses que tienen expenses asociados
	expenseMonths := getExpenseMonths(db, updateData.BillID, updateData.UserID)

	for _, month := range remainingMonths {
		if expenseMonths[month] {
			// Si tiene expense, actualizar tabla expenses y expense_amount
			updateExpenseAmountForBillDifference(db, updateData.BillID, updateData.UserID, month, amountDiff)
			updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "expense", 1)
		} else {
			// Si no tiene expense, actualizar bill_amount
			updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "bill", 1)
		}
		// Ajustar balance principal según diferencia de importe
		updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "main", -1)
	}
	return nil
}

// getExpenseMonths obtiene meses con expenses para un bill específico
// Identifica qué meses del periodo tienen gastos registrados
func getExpenseMonths(db *sql.DB, billID int, userID string) map[string]bool {
	expenseMonths := make(map[string]bool)

	// Consultar expenses asociados al bill agrupados por mes
	rows, err := db.Query("SELECT DISTINCT strftime('%Y-%m', date) FROM expenses WHERE bill_id = ? AND user_id = ?", billID, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				expenseMonths[month] = true
			}
		}
	}
	return expenseMonths
}
