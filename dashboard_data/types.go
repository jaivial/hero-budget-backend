package main

// Definición de estructuras de datos para el dashboard
type DashboardData struct {
	Period           string          `json:"period"`
	Date             string          `json:"date"`
	BudgetOverview   BudgetOverview  `json:"budget_overview"`
	SavingsOverview  SavingsOverview `json:"savings_overview"`
	CashDistribution CashBank        `json:"cash_distribution"`
	FinanceMetrics   FinanceMetrics  `json:"finance_metrics"`
	UpcomingBills    []Bill          `json:"upcoming_bills"`
}

// BudgetOverview estructura para vista general del presupuesto
type BudgetOverview struct {
	MoneyFlow       MoneyFlow `json:"money_flow"`
	RemainingAmount float64   `json:"remaining_amount"`
	TotalAmount     float64   `json:"total_amount"`
	SpentAmount     float64   `json:"spent_amount"`
	UpcomingAmount  float64   `json:"upcoming_amount"`
	CombinedExpense float64   `json:"combined_expense"`
	ExpensePercent  float64   `json:"expense_percent"`
	DailyRate       float64   `json:"daily_rate"`
	HighSpending    bool      `json:"high_spending"`
	TotalIncome     float64   `json:"total_income"`
}

// MoneyFlow estructura para flujo de dinero entre períodos
type MoneyFlow struct {
	Percent      float64 `json:"percent"`
	FromPrevious float64 `json:"from_previous"`
}

// SavingsOverview estructura para vista general de ahorros
type SavingsOverview struct {
	Percent     float64 `json:"percent"`
	Available   float64 `json:"available"`
	Goal        float64 `json:"goal"`
	Period      string  `json:"period"`
	NeedToSave  float64 `json:"need_to_save"`
	DailyTarget float64 `json:"daily_target"`
}

// CashBank estructura para distribución de efectivo y banco
type CashBank struct {
	Month        string  `json:"month"`
	CashAmount   float64 `json:"cash_amount"`
	CashPercent  float64 `json:"cash_percent"`
	BankAmount   float64 `json:"bank_amount"`
	BankPercent  float64 `json:"bank_percent"`
	MonthlyTotal float64 `json:"monthly_total"`
}

// FinanceMetrics estructura para métricas financieras
type FinanceMetrics struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Bills    float64 `json:"bills"`
}

// Bill estructura para representar facturas pendientes
type Bill struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Amount      float64 `json:"amount"`
	DueDate     string  `json:"due_date"`
	Paid        bool    `json:"paid"`
	Overdue     bool    `json:"overdue"`
	OverdueDays int     `json:"overdue_days"`
	Recurring   bool    `json:"recurring"`
	Category    string  `json:"category"`
	Icon        string  `json:"icon"`
}