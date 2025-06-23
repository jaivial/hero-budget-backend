package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// UpdateBillInMonthlyCashBankBalance estructura para actualizar facturas en monthly_cash_bank_balance
// CORREGIDO: Enfoque específico para monthly_cash_bank_balance con lógica coherente
type BillCashBankUpdateData struct {
	BillID            int
	UserID            string
	OldAmount         float64
	NewAmount         float64
	OldDurationMonths int
	NewDurationMonths int
	OldStartDate      string
	NewStartDate      string
	OldPaymentMethod  string
	NewPaymentMethod  string
	OldPaymentDay     int
	NewPaymentDay     int
}

// parseFlexibleDateCashBank parsea fechas manejando múltiples formatos
// Función auxiliar para compatibilidad con formatos ISO y estándar
func parseFlexibleDateCashBank(dateStr string) (time.Time, error) {
	if strings.Contains(dateStr, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		if parsed, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
			return parsed, nil
		}
		if parsed, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			return parsed, nil
		}
	}
	// Formato solo fecha: "2025-01-15"
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		return parsed, nil
	}
	// Formato año-mes: "2025-01" (agregar día 01)
	if parsed, err := time.Parse("2006-01", dateStr); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// calculateMonthsFromDurationCashBank calcula los meses afectados por una factura
// Genera lista de meses en formato YYYY-MM basado en start_date y duration_months
func calculateMonthsFromDurationCashBank(startDateStr string, durationMonths int) ([]string, error) {
	startDate, err := parseFlexibleDateCashBank(startDateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing start date: %v", err)
	}

	var months []string
	for i := 0; i < durationMonths; i++ {
		monthDate := startDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		months = append(months, yearMonth)
	}
	return months, nil
}

// recalculateAllSubsequentMonthsBalance recalcula total_balance para todos los meses posteriores
// NUEVO: Función corregida que recalcula balance total considerando previous_amounts actualizados
func recalculateAllSubsequentMonthsBalance(db *sql.DB, userID, startMonth string) error {
	log.Printf("🔄 Recalculando total_balance para todos los meses >= %s", startMonth)
	
	// Obtener todos los meses desde startMonth en adelante
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month >= ? ORDER BY year_month", userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months for balance recalculation: %v", err)
	}
	defer rows.Close()
	
	var months []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			months = append(months, month)
		}
	}
	
	// Recalcular total_balance para cada mes considerando previous_amounts
	for _, month := range months {
		// Obtener valores actuales del mes
		var cashAmount, bankAmount, prevCash, prevBank float64
		err := db.QueryRow(`
			SELECT COALESCE(cash_amount, 0), COALESCE(bank_amount, 0),
			       COALESCE(previous_cash_amount, 0), COALESCE(previous_bank_amount, 0)
			FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month = ?
		`, userID, month).Scan(&cashAmount, &bankAmount, &prevCash, &prevBank)
		
		if err != nil {
			log.Printf("Error obteniendo datos para mes %s: %v", month, err)
			continue
		}
		
		// CORREGIDO: total_balance = (previous_amounts + current_amounts)
		newTotalBalance := (prevCash + prevBank) + (cashAmount + bankAmount)
		newTotalPrevious := prevCash + prevBank
		
		// Actualizar total_balance y total_previous_balance
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET total_balance = ?, total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, newTotalBalance, newTotalPrevious, userID, month)
		
		if err != nil {
			log.Printf("Error actualizando total_balance para mes %s: %v", month, err)
		} else {
			log.Printf("✅ Mes %s: total_balance=%.2f, total_previous_balance=%.2f", month, newTotalBalance, newTotalPrevious)
		}
	}
	
	return nil
}

// ensureMonthRecordExists asegura que existe un registro para el mes especificado
// Función auxiliar para crear registros de meses cuando sea necesario
func ensureMonthRecordExists(db *sql.DB, userID, yearMonth string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", userID, yearMonth)
	if err != nil {
		return fmt.Errorf("error creating monthly record for %s: %v", yearMonth, err)
	}
	return nil
}