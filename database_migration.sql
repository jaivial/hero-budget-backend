-- Migración de base de datos Hero Budget
-- Crear tabla faltante: monthly_cash_bank_balance
-- Fecha: 2025-06-03

-- Crear tabla monthly_cash_bank_balance si no existe
CREATE TABLE IF NOT EXISTS monthly_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    month INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, month, year)
);

-- Crear índices para optimizar consultas
CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_user_date 
ON monthly_cash_bank_balance(user_id, year, month);

-- Insertar datos iniciales para usuario de prueba
INSERT OR IGNORE INTO monthly_cash_bank_balance 
(user_id, month, year, cash_amount, bank_amount) 
VALUES 
('36', 6, 2025, 500.0, 500.0),
('test_user', 6, 2025, 1000.0, 1000.0);

-- Verificar que la tabla fue creada
SELECT name FROM sqlite_master WHERE type='table' AND name='monthly_cash_bank_balance'; 