-- Asegurar que las operaciones de SQLite usen UTF-8
PRAGMA encoding = "UTF-8";

-- Recrear tabla de categorías con soporte explícito para UTF-8
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    emoji TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Asegurarse de que la columna emoji permita almacenar caracteres unicode
PRAGMA table_info(categories); 