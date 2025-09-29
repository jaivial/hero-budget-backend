package common

import "fmt"

// KeyBuilder construye keys consistentes para el cache
type KeyBuilder struct{}

// BuildUserKey construye key para datos de usuario
func (kb *KeyBuilder) BuildUserKey(userID string) string {
	return fmt.Sprintf("user:%s", userID)
}

// BuildIncomeKey construye key para datos de ingresos
func (kb *KeyBuilder) BuildIncomeKey(userID, period string) string {
	return fmt.Sprintf("income:%s:%s", userID, period)
}

// BuildExpenseKey construye key para datos de gastos
func (kb *KeyBuilder) BuildExpenseKey(userID, period string) string {
	return fmt.Sprintf("expense:%s:%s", userID, period)
}

// BuildBillsKey construye key para datos de facturas
func (kb *KeyBuilder) BuildBillsKey(userID, period string) string {
	return fmt.Sprintf("bills:%s:%s", userID, period)
}

// BuildDashboardKey construye key para datos del dashboard
func (kb *KeyBuilder) BuildDashboardKey(userID, period string) string {
	return fmt.Sprintf("dashboard:%s:%s", userID, period)
}

// BuildSavingsKey construye key para datos de ahorros
func (kb *KeyBuilder) BuildSavingsKey(userID string) string {
	return fmt.Sprintf("savings:%s", userID)
}

// BuildCashBankKey construye key para distribución cash/bank
func (kb *KeyBuilder) BuildCashBankKey(userID string) string {
	return fmt.Sprintf("cashbank:%s", userID)
}

// Instancia global del KeyBuilder
var KB = &KeyBuilder{}
