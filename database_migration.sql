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

-- =====================================================================
-- Migración: Agregar columnas category_id a incomes y expenses
-- Fecha: 2025-09-29
-- Propósito: Permitir enlaces directos entre categorías y transacciones
-- =====================================================================

-- Agregar category_id a la tabla incomes
ALTER TABLE incomes ADD COLUMN category_id INTEGER;

-- Agregar category_id a la tabla expenses
ALTER TABLE expenses ADD COLUMN category_id INTEGER;

-- Crear índices para las nuevas columnas category_id
CREATE INDEX IF NOT EXISTS idx_incomes_category_id ON incomes(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_category_id ON expenses(category_id);

-- Poblar category_id existentes basándose en el nombre de categoría
-- Actualizar incomes con category_id basado en category name
UPDATE incomes
SET category_id = (
    SELECT c.id
    FROM categories c
    WHERE c.name = incomes.category
    AND c.user_id = incomes.user_id
    AND c.type = 'income'
    LIMIT 1
)
WHERE category_id IS NULL AND category IS NOT NULL;

-- Actualizar expenses con category_id basado en category name
UPDATE expenses
SET category_id = (
    SELECT c.id
    FROM categories c
    WHERE c.name = expenses.category
    AND c.user_id = expenses.user_id
    AND c.type = 'expense'
    LIMIT 1
)
WHERE category_id IS NULL AND category IS NOT NULL;

-- Verificar la migración
SELECT
    'incomes' as table_name,
    COUNT(*) as total_records,
    COUNT(category_id) as records_with_category_id,
    COUNT(*) - COUNT(category_id) as records_without_category_id
FROM incomes
UNION ALL
SELECT
    'expenses' as table_name,
    COUNT(*) as total_records,
    COUNT(category_id) as records_with_category_id,
    COUNT(*) - COUNT(category_id) as records_without_category_id
FROM expenses; 