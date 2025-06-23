package main

// Definición de estructuras de datos para money flow sync
type SyncRequest struct {
	UserID string `json:"user_id"`
	Period string `json:"period"`
}

// ApiResponse estructura estándar para respuestas de la API
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// BudgetData estructura para datos de presupuesto sincronizados
type BudgetData struct {
	UserID          string  `json:"user_id"`
	Period          string  `json:"period"`
	Date            string  `json:"date"`
	TotalAmount     float64 `json:"total_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	SpentAmount     float64 `json:"spent_amount"`
	UpcomingAmount  float64 `json:"upcoming_amount"`
	FromPrevious    float64 `json:"from_previous"`
	Percent         float64 `json:"percent"`
	TotalIncome     float64 `json:"total_income"`
}

// Bill estructura simplificada para cálculos de facturas
type Bill struct {
	Amount    float64 `json:"amount"`
	DueDate   string  `json:"due_date"`
	Paid      bool    `json:"paid"`
	Recurring bool    `json:"recurring"`
}