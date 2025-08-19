-- =====================================================================
-- SYNC OPERATIONS SCHEMA FOR INCOME MANAGEMENT
-- Implementation following docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md
-- =====================================================================

-- Create sync_operations table with proper constraints for income management
CREATE TABLE IF NOT EXISTS sync_operations (
    operation_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete')),
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    operation_data TEXT NOT NULL,
    device_ids TEXT DEFAULT '[]',
    client_timestamp INTEGER DEFAULT 0,
    server_timestamp INTEGER DEFAULT 0
);

-- Create necessary indexes for performance
CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
    ON sync_operations(operation_id);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
    ON sync_operations(user_id, operation_id);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
    ON sync_operations(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
    ON sync_operations(user_id, entity_type, entity_id);