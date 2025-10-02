-- =====================================================================
-- HERO BUDGET - ESQUEMA CENTRALIZADO DE BASE DE DATOS
-- VERSIÓN: 1.0.0
-- FECHA: 2025-06-23
-- =====================================================================
-- IMPORTANTE: Este es el ÚNICO archivo autorizado para definir
-- la estructura de la base de datos. Todos los servicios deben
-- usar este esquema central.
-- =====================================================================

PRAGMA encoding = "UTF-8";
PRAGMA foreign_keys = ON;

-- =====================================================================
-- TABLA PRINCIPAL: USUARIOS
-- =====================================================================
-- Centraliza TODAS las definiciones de users que estaban dispersas
-- en google_auth, signup, reset_password, apple-auth
-- =====================================================================

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    google_id TEXT,
    apple_id TEXT,
    email TEXT NOT NULL,
    password TEXT,
    name TEXT,
    given_name TEXT,
    family_name TEXT,
    picture TEXT,
    profile_image_blob TEXT,
    locale TEXT,
    verified_email BOOLEAN DEFAULT 0,
    verification_code TEXT,
    type TEXT DEFAULT 'email',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reset_token TEXT,
    reset_expires DATETIME,
    
    -- CONSTRAINTS UNIFICADOS
    UNIQUE(google_id) WHERE google_id IS NOT NULL,
    UNIQUE(apple_id) WHERE apple_id IS NOT NULL AND apple_id != '',
    UNIQUE(email, type)
);

-- Índices para users
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_type ON users(type);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- =====================================================================
-- GESTIÓN DE CATEGORÍAS
-- =====================================================================

CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY,  -- Sin AUTOINCREMENT: permite tanto auto-increment como IDs explícitos (optimistic UI)
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    emoji TEXT DEFAULT '📁',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(user_id, name, type)
);

CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories(user_id);
CREATE INDEX IF NOT EXISTS idx_categories_type ON categories(type);

-- =====================================================================
-- GESTIÓN DE INGRESOS
-- =====================================================================

CREATE TABLE IF NOT EXISTS incomes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    amount REAL NOT NULL DEFAULT 0,
    date TEXT NOT NULL,
    category TEXT,
    category_id INTEGER,
    payment_method TEXT DEFAULT 'bank',
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_incomes_user_id ON incomes(user_id);
CREATE INDEX IF NOT EXISTS idx_incomes_date ON incomes(date);
CREATE INDEX IF NOT EXISTS idx_incomes_category ON incomes(category);
CREATE INDEX IF NOT EXISTS idx_incomes_category_id ON incomes(category_id);

-- =====================================================================
-- GESTIÓN DE GASTOS
-- =====================================================================

CREATE TABLE IF NOT EXISTS expenses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    amount REAL NOT NULL DEFAULT 0,
    date TEXT NOT NULL,
    category TEXT,
    category_id INTEGER,
    payment_method TEXT DEFAULT 'bank',
    description TEXT,
    bill_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_expenses_user_id ON expenses(user_id);
CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date);
CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category);
CREATE INDEX IF NOT EXISTS idx_expenses_category_id ON expenses(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_bill_id ON expenses(bill_id);

-- =====================================================================
-- GESTIÓN DE FACTURAS
-- =====================================================================

CREATE TABLE IF NOT EXISTS bills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    amount REAL NOT NULL DEFAULT 0,
    due_date TEXT,
    start_date TEXT,
    category TEXT,
    description TEXT,
    recurrence TEXT DEFAULT 'monthly',
    is_paid BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bills_user_id ON bills(user_id);
CREATE INDEX IF NOT EXISTS idx_bills_due_date ON bills(due_date);
CREATE INDEX IF NOT EXISTS idx_bills_recurrence ON bills(recurrence);

CREATE TABLE IF NOT EXISTS bill_payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    year_month TEXT NOT NULL,
    amount REAL NOT NULL DEFAULT 0.0,
    paid_date TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(bill_id, year_month)
);

CREATE INDEX IF NOT EXISTS idx_bill_payments_user_id ON bill_payments(user_id);
CREATE INDEX IF NOT EXISTS idx_bill_payments_year_month ON bill_payments(year_month);
CREATE INDEX IF NOT EXISTS idx_bill_payments_bill_id ON bill_payments(bill_id);

-- =====================================================================
-- GESTIÓN DE EFECTIVO Y BANCO
-- =====================================================================

CREATE TABLE IF NOT EXISTS cash_bank (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id)
);

CREATE INDEX IF NOT EXISTS idx_cash_bank_user_id ON cash_bank(user_id);

CREATE TABLE IF NOT EXISTS cash_bank_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('income', 'expense', 'transfer')),
    amount REAL NOT NULL,
    from_account TEXT,
    to_account TEXT,
    description TEXT,
    date TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_user_id ON cash_bank_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_date ON cash_bank_transactions(date);
CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_type ON cash_bank_transactions(type);

-- =====================================================================
-- BALANCES TEMPORALES - DIARIOS
-- =====================================================================

CREATE TABLE IF NOT EXISTS daily_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    date TEXT NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_user_date ON daily_cash_bank_balance(user_id, date);
CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_date ON daily_cash_bank_balance(date);

-- =====================================================================
-- BALANCES TEMPORALES - SEMANALES  
-- =====================================================================

CREATE TABLE IF NOT EXISTS weekly_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    week INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, week, year)
);

CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_user_date ON weekly_cash_bank_balance(user_id, year, week);
CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_year ON weekly_cash_bank_balance(year);

-- =====================================================================
-- BALANCES TEMPORALES - MENSUALES
-- =====================================================================

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

CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_user_date ON monthly_cash_bank_balance(user_id, year, month);
CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_year ON monthly_cash_bank_balance(year);

-- =====================================================================
-- BALANCES TEMPORALES - TRIMESTRALES
-- =====================================================================

CREATE TABLE IF NOT EXISTS quarterly_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    quarter INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, quarter, year)
);

CREATE INDEX IF NOT EXISTS idx_quarterly_cash_bank_user_date ON quarterly_cash_bank_balance(user_id, year, quarter);
CREATE INDEX IF NOT EXISTS idx_quarterly_cash_bank_year ON quarterly_cash_bank_balance(year);

-- =====================================================================
-- BALANCES TEMPORALES - SEMESTRALES
-- =====================================================================

CREATE TABLE IF NOT EXISTS semiannual_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    semester INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, semester, year)
);

CREATE INDEX IF NOT EXISTS idx_semiannual_cash_bank_user_date ON semiannual_cash_bank_balance(user_id, year, semester);
CREATE INDEX IF NOT EXISTS idx_semiannual_cash_bank_year ON semiannual_cash_bank_balance(year);

-- =====================================================================
-- BALANCES TEMPORALES - ANUALES
-- =====================================================================

CREATE TABLE IF NOT EXISTS annual_cash_bank_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    year INTEGER NOT NULL,
    cash_amount REAL DEFAULT 0,
    bank_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, year)
);

CREATE INDEX IF NOT EXISTS idx_annual_cash_bank_user_date ON annual_cash_bank_balance(user_id, year);
CREATE INDEX IF NOT EXISTS idx_annual_cash_bank_year ON annual_cash_bank_balance(year);

-- =====================================================================
-- GESTIÓN DE PRESUPUESTO
-- =====================================================================

CREATE TABLE IF NOT EXISTS budget (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    month INTEGER NOT NULL,
    year INTEGER NOT NULL,
    total_income REAL NOT NULL DEFAULT 0,
    total_expenses REAL NOT NULL DEFAULT 0,
    savings_goal REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, month, year)
);

CREATE INDEX IF NOT EXISTS idx_budget_user_id ON budget(user_id);
CREATE INDEX IF NOT EXISTS idx_budget_month_year ON budget(month, year);

-- =====================================================================
-- GESTIÓN DE AHORROS
-- =====================================================================

CREATE TABLE IF NOT EXISTS savings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    amount REAL NOT NULL DEFAULT 0,
    goal REAL DEFAULT 0,
    description TEXT,
    period TEXT NOT NULL DEFAULT 'monthly',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_savings_user_id ON savings(user_id);
CREATE INDEX IF NOT EXISTS idx_savings_period ON savings(period);

-- =====================================================================
-- MÉTRICAS FINANCIERAS
-- =====================================================================

CREATE TABLE IF NOT EXISTS finance_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL DEFAULT 0,
    period TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, metric_name, period)
);

CREATE INDEX IF NOT EXISTS idx_finance_metrics_user_id ON finance_metrics(user_id);
CREATE INDEX IF NOT EXISTS idx_finance_metrics_name ON finance_metrics(metric_name);

-- =====================================================================
-- BALANCES GENERALES (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS balances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    current_cash REAL DEFAULT 0,
    current_bank REAL DEFAULT 0,
    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id)
);

CREATE INDEX IF NOT EXISTS idx_balances_user_id ON balances(user_id);

-- =====================================================================
-- BALANCES DIARIOS (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS daily_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    date TEXT NOT NULL,
    cash_balance REAL DEFAULT 0,
    bank_balance REAL DEFAULT 0,
    total_balance REAL DEFAULT 0,
    daily_expenses REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_daily_balance_user_date ON daily_balance(user_id, date);
CREATE INDEX IF NOT EXISTS idx_daily_balance_date ON daily_balance(date);

-- =====================================================================
-- BALANCES SEMANALES (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS weekly_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    week_start TEXT NOT NULL,
    week_end TEXT NOT NULL,
    week_number INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_balance REAL DEFAULT 0,
    bank_balance REAL DEFAULT 0,
    total_balance REAL DEFAULT 0,
    weekly_expenses REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, week_number, year)
);

CREATE INDEX IF NOT EXISTS idx_weekly_balance_user_date ON weekly_balance(user_id, year, week_number);
CREATE INDEX IF NOT EXISTS idx_weekly_balance_year ON weekly_balance(year);

-- =====================================================================
-- BALANCES TRIMESTRALES (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS quarterly_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    quarter INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_balance REAL DEFAULT 0,
    bank_balance REAL DEFAULT 0,
    total_balance REAL DEFAULT 0,
    quarterly_expenses REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, quarter, year)
);

CREATE INDEX IF NOT EXISTS idx_quarterly_balance_user_date ON quarterly_balance(user_id, year, quarter);
CREATE INDEX IF NOT EXISTS idx_quarterly_balance_year ON quarterly_balance(year);

-- =====================================================================
-- BALANCES SEMESTRALES (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS semiannual_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    semester INTEGER NOT NULL,
    year INTEGER NOT NULL,
    cash_balance REAL DEFAULT 0,
    bank_balance REAL DEFAULT 0,
    total_balance REAL DEFAULT 0,
    semiannual_expenses REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, semester, year)
);

CREATE INDEX IF NOT EXISTS idx_semiannual_balance_user_date ON semiannual_balance(user_id, year, semester);
CREATE INDEX IF NOT EXISTS idx_semiannual_balance_year ON semiannual_balance(year);

-- =====================================================================
-- BALANCES ANUALES (PARA EXPENSES)
-- =====================================================================

CREATE TABLE IF NOT EXISTS annual_balance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    year INTEGER NOT NULL,
    cash_balance REAL DEFAULT 0,
    bank_balance REAL DEFAULT 0,
    total_balance REAL DEFAULT 0,
    annual_expenses REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, year)
);

CREATE INDEX IF NOT EXISTS idx_annual_balance_user_date ON annual_balance(user_id, year);
CREATE INDEX IF NOT EXISTS idx_annual_balance_year ON annual_balance(year);

-- =====================================================================
-- VERSIÓN DEL ESQUEMA
-- =====================================================================

CREATE TABLE IF NOT EXISTS schema_version (
    id INTEGER PRIMARY KEY,
    version TEXT NOT NULL,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR REPLACE INTO schema_version (id, version) VALUES (1, '1.0.0');

-- =====================================================================
-- FIN DEL ESQUEMA CENTRALIZADO
-- =====================================================================