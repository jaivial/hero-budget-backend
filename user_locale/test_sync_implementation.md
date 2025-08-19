# User Locale Sync Operations Implementation Test

## Overview
This document provides testing steps to verify that the sync operations implementation for the user_locale service is working correctly according to the SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.

## Key Implementation Features
1. ✅ Database schema updates with proper operation_type constraints
2. ✅ Operation ID generation system with timestamp-based format 
3. ✅ Enhanced addSyncOperation function with auto-generation
4. ✅ Consistent handler pattern (auto-records for all operations)
5. ✅ API service with sync parameter injection
6. ✅ Frontend service integration with sync queue

## Testing Commands

### 1. Test User Locale Update with Sync Parameters

```bash
# Test locale update with device_id to trigger sync operation recording
curl -X POST "https://herobudget.jaimedigitalstudio.com/user_locale/update" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "18",
    "locale": "es_ES",
    "device_id": "test-device-123"
  }'
```

### 2. Test User Locale Update without Sync Parameters

```bash
# Test locale update without device_id (should still work but no sync operation)
curl -X POST "https://herobudget.jaimedigitalstudio.com/user_locale/update" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "18",
    "locale": "en_US"
  }'
```

### 3. Verify Sync Operations Are Recorded

```sql
-- Check if sync operations are being recorded in the database
-- Connect to the user_locale service database and run:

SELECT 
    operation_id, 
    operation_type, 
    entity_type, 
    entity_id, 
    device_ids, 
    created_at,
    operation_data
FROM sync_operations 
WHERE user_id = "18" 
  AND entity_type = "user_locale"
ORDER BY created_at DESC 
LIMIT 5;
```

### 4. Verify Operation ID Format

```sql
-- Verify operation IDs follow timestamp_sequence format
SELECT 
    operation_id,
    CASE WHEN operation_id REGEXP '^[0-9]{13}_[0-9]{3}$' 
         THEN 'VALID' ELSE 'INVALID' END as format_check
FROM sync_operations 
WHERE entity_type = "user_locale"
ORDER BY created_at DESC 
LIMIT 5;
```

### 5. Test Incremental Sync Processing

```bash
# Test that delta sync can fetch and process user_locale operations
curl -X GET "https://herobudget.jaimedigitalstudio.com/delta-sync/fetch?user_id=18&last_operation_id=null"
```

## Expected Results

### 1. Successful Response Format
```json
{
  "success": true,
  "message": "User locale updated successfully",
  "locale": "es_ES"
}
```

### 2. Sync Operation Database Record
```
operation_id: 1737355200000_001 (timestamp_sequence format)
operation_type: update
entity_type: user_locale
entity_id: 18
device_ids: ["test-device-123"]
operation_data: {"user_id":"18","locale":"es_ES","action":"update_locale","processed_at":"2025-01-19 15:30:00"}
```

### 3. Backend Logs Should Show
```
✅ Recording sync operation for user locale update with auto-generated operation_id
Generated operation ID: 1737355200000_001
✅ SUCCESS: Successfully recorded sync operation for user locale update: user=18, locale=es_ES
```

## Validation Checklist

- [ ] **Schema Compatibility**: sync_operations table accepts 'update' operation_type for 'user_locale'
- [ ] **Operation ID Generation**: Auto-generates timestamp-based operation IDs
- [ ] **Device ID Handling**: Properly stores device_ids as JSON array
- [ ] **Consistent Recording**: Records sync operation for every update (with or without provided operation_id)
- [ ] **Error Handling**: Continues operation even if sync recording fails
- [ ] **API Integration**: Frontend can call updateUserLocale with auto-injection
- [ ] **Incremental Sync**: Delta sync endpoint returns user_locale operations

## Troubleshooting

### If Sync Operations Are Not Being Recorded:

1. **Check Schema Constraint**: Most common issue - verify operation_type constraint includes all needed types
```sql
-- Check current constraint
PRAGMA table_info(sync_operations);
```

2. **Check Service Logs**: Look for constraint violation errors
```bash
# Check service logs for errors
tail -f /path/to/user_locale.log
```

3. **Verify Database Connection**: Ensure service can write to sync_operations table
```sql
-- Test basic insert
INSERT INTO sync_operations (operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data) 
VALUES ('test_123', 'test_user', 1737355200000, 'update', 'user_locale', 'test_id', '{}');
```

## Success Criteria

The implementation is successful when:
1. ✅ All locale updates automatically record sync operations
2. ✅ Operation IDs follow timestamp-based format
3. ✅ Device IDs are properly stored and tracked
4. ✅ Incremental sync can fetch and process user_locale operations
5. ✅ No constraint violations or sync recording failures
6. ✅ Frontend can trigger sync operations through API calls

## Integration with Other Services

This implementation pattern should be replicated across all services:
- bills_management
- expense_management  
- income_management
- savings_management
- cash_bank_management
- categories_management

Each service should follow the same consistent patterns for maximum compatibility.