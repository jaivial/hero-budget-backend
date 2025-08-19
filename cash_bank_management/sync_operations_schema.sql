-- =====================================================================
-- SYNC OPERATIONS SCHEMA FOR CASH BANK MANAGEMENT
-- Implementation following docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md
-- =====================================================================

-- Create or update sync_operations table with proper constraints for cash_bank operations
-- This table supports multi-device synchronization with timestamp-based operation IDs

-- SQLite doesn't support modifying CHECK constraints directly, so we recreate the table
-- Create temporary table with updated schema
CREATE TABLE IF NOT EXISTS sync_operations_new (
    operation_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete', 'transfer', 'update_cash', 'update_bank')),
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    operation_data TEXT NOT NULL,
    device_ids TEXT DEFAULT '[]',
    client_timestamp INTEGER DEFAULT 0,
    server_timestamp INTEGER DEFAULT 0
);

-- Copy existing data to new table if sync_operations already exists
INSERT OR IGNORE INTO sync_operations_new 
SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, 
       operation_data, device_ids, client_timestamp, server_timestamp 
FROM sync_operations 
WHERE EXISTS (SELECT name FROM sqlite_master WHERE type='table' AND name='sync_operations');

-- Drop old table and rename new one (only if old table exists)
DROP TABLE IF EXISTS sync_operations_old;
UPDATE sqlite_master SET name = 'sync_operations_old' WHERE name = 'sync_operations' AND type = 'table';
ALTER TABLE sync_operations_new RENAME TO sync_operations;
DROP TABLE IF EXISTS sync_operations_old;

-- Create necessary indexes for performance
CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
    ON sync_operations(operation_id);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
    ON sync_operations(user_id, operation_id);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
    ON sync_operations(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
    ON sync_operations(user_id, entity_type, entity_id);

-- Index for cash_bank specific queries
CREATE INDEX IF NOT EXISTS idx_sync_operations_cash_bank 
    ON sync_operations(user_id, entity_type) 
    WHERE entity_type IN ('cash_bank_transfers', 'cash_bank_updates');