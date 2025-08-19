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
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
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
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// DeleteIncomeRequest estructura para eliminar ingresos
type DeleteIncomeRequest struct {
	UserID   string `json:"user_id"`
	IncomeID int    `json:"income_id"`
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// SyncOperation estructura para registrar operaciones de sincronización
type SyncOperation struct {
	ID           int    `json:"id"`
	UserID       string `json:"user_id"`
	OperationID  string `json:"operation_id"`
	Action       string `json:"action"`        // "create", "update", "delete"
	TableName    string `json:"table_name"`    // "incomes", "expenses", etc.
	RecordID     string `json:"record_id"`     // ID del registro afectado
	Data         string `json:"data"`          // JSON con los datos de la operación
	DeviceID     string `json:"device_id"`
	ClientTimestamp int64 `json:"client_timestamp"`
	ServerTimestamp int64 `json:"server_timestamp"`
	CreatedAt    string `json:"created_at"`
}

// ApiResponse estructura estándar para respuestas de la API
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}