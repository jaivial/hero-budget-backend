package main

// Definición de estructuras de datos para income management
type Income struct {
	ID            int     `json:"id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"` // "cash" o "bank"
	Description   string  `json:"description,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

// AddIncomeRequest estructura para agregar nuevos ingresos
type AddIncomeRequest struct {
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description,omitempty"`
}

// UpdateIncomeRequest estructura para actualizar ingresos existentes
type UpdateIncomeRequest struct {
	UserID        string  `json:"user_id"`
	IncomeID      int     `json:"income_id"`
	Amount        float64 `json:"amount,omitempty"`
	Date          string  `json:"date,omitempty"`
	Category      string  `json:"category,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	Description   string  `json:"description,omitempty"`
}

// DeleteIncomeRequest estructura para eliminar ingresos
type DeleteIncomeRequest struct {
	UserID   string `json:"user_id"`
	IncomeID int    `json:"income_id"`
}

// ApiResponse estructura estándar para respuestas de la API
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}