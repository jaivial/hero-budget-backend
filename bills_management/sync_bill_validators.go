package main

import (
	"fmt"
	"time"
)

// Validadores para sincronización offline de facturas
// Implementa validaciones específicas para garantizar integridad de datos

// Validate valida la estructura de una solicitud de sincronización por lotes de facturas
// Asegura que todos los campos requeridos están presentes y son válidos
func (req *SyncBillBatchRequest) Validate() error {
	// Validar campos básicos requeridos
	if req.UserID == "" {
		return fmt.Errorf("user_id es requerido para sincronización de facturas")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id es requerido para evitar duplicados")
	}
	if len(req.Bills) == 0 {
		return fmt.Errorf("no hay facturas para sincronizar")
	}
	
	// Validar límite de lote para evitar sobrecarga del servidor
	if len(req.Bills) > 100 {
		return fmt.Errorf("el lote excede el límite máximo de 100 facturas")
	}
	
	// Validar cada factura individual en el lote
	for i, bill := range req.Bills {
		if err := bill.Validate(); err != nil {
			return fmt.Errorf("factura %d inválida: %v", i, err)
		}
	}
	
	// Validar formato de timestamps si están presentes
	if req.LastSync != "" {
		if _, err := time.Parse(time.RFC3339, req.LastSync); err != nil {
			return fmt.Errorf("formato de last_sync inválido: debe ser RFC3339")
		}
	}
	
	return nil
}

// Validate valida una factura offline individual
// Verifica que contiene la información mínima requerida según la acción
func (bill *OfflineBill) Validate() error {
	// Validar campos básicos siempre requeridos
	if bill.LocalID == "" {
		return fmt.Errorf("local_id es requerido para identificación única")
	}
	if bill.UserID == "" {
		return fmt.Errorf("user_id es requerido para asociación de factura")
	}
	
	// Validar acción específica
	if !isValidBillAction(bill.Action) {
		return fmt.Errorf("action debe ser add, update o delete")
	}
	
	// Validaciones específicas según el tipo de acción
	switch bill.Action {
	case "add":
		return validateAddBillAction(bill)
	case "update":
		return validateUpdateBillAction(bill)
	case "delete":
		return validateDeleteBillAction(bill)
	default:
		return fmt.Errorf("acción no reconocida: %s", bill.Action)
	}
}

// isValidBillAction verifica si la acción es válida para facturas
func isValidBillAction(action string) bool {
	validActions := []string{"add", "update", "delete"}
	for _, validAction := range validActions {
		if action == validAction {
			return true
		}
	}
	return false
}

// validateAddBillAction valida los datos requeridos para agregar una nueva factura
func validateAddBillAction(bill *OfflineBill) error {
	if bill.Name == "" {
		return fmt.Errorf("name es requerido para nueva factura")
	}
	if bill.Amount <= 0 {
		return fmt.Errorf("amount debe ser mayor que 0")
	}
	if bill.StartDate == "" {
		return fmt.Errorf("start_date es requerido para nueva factura")
	}
	if bill.PaymentDay <= 0 || bill.PaymentDay > 31 {
		return fmt.Errorf("payment_day debe estar entre 1 y 31")
	}
	if bill.DurationMonths <= 0 {
		return fmt.Errorf("duration_months debe ser mayor que 0")
	}
	if bill.PaymentMethod != "cash" && bill.PaymentMethod != "bank" {
		return fmt.Errorf("payment_method debe ser cash o bank")
	}
	
	// Validar formato de fechas
	if _, err := time.Parse("2006-01-02T15:04:05Z", bill.StartDate); err != nil {
		return fmt.Errorf("formato de start_date inválido: %v", err)
	}
	
	return nil
}

// validateUpdateBillAction valida los datos para actualización de factura existente
func validateUpdateBillAction(bill *OfflineBill) error {
	if bill.ServerID <= 0 {
		return fmt.Errorf("server_id es requerido para actualizar factura existente")
	}
	
	// Para updates, los campos pueden ser opcionales, pero si están presentes deben ser válidos
	if bill.Amount != 0 && bill.Amount < 0 {
		return fmt.Errorf("amount no puede ser negativo")
	}
	if bill.PaymentDay != 0 && (bill.PaymentDay < 1 || bill.PaymentDay > 31) {
		return fmt.Errorf("payment_day debe estar entre 1 y 31")
	}
	if bill.DurationMonths != 0 && bill.DurationMonths < 0 {
		return fmt.Errorf("duration_months no puede ser negativo")
	}
	if bill.PaymentMethod != "" && bill.PaymentMethod != "cash" && bill.PaymentMethod != "bank" {
		return fmt.Errorf("payment_method debe ser cash o bank")
	}
	
	return nil
}

// validateDeleteBillAction valida los datos para eliminación de factura
func validateDeleteBillAction(bill *OfflineBill) error {
	if bill.ServerID <= 0 {
		return fmt.Errorf("server_id es requerido para eliminar factura existente")
	}
	
	// Para deletes, solo necesitamos IDs básicos
	return nil
}