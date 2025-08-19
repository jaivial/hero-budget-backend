# Cash Bank Management - Sync Operations Testing & Validation Guide

This guide provides comprehensive testing steps to validate that sync operations are being recorded correctly for all cash bank operations.

## Pre-Testing Setup

### 1. Database Schema Verification
First, ensure the sync_operations table exists with proper constraints:

```bash
# Connect to your database and run:
sqlite3 /path/to/your/database.db
```

```sql
-- Check if sync_operations table exists
.tables

-- Verify table schema includes cash_bank operation types
.schema sync_operations

-- Should show operation_type CHECK constraint including: 'transfer', 'update_cash', 'update_bank'
```

### 2. Backend Service Status
Ensure the cash_bank_management service is running:

```bash
# Check if service is running
ps aux | grep cash_bank_management

# Start service if not running (adjust path as needed)
cd /path/to/backend/cash_bank_management
go run .
```

## Testing Plan - Backend API Endpoints

### Test 1: Cash-to-Bank Transfer with Sync

```bash
# Test cash-to-bank transfer with sync parameters
curl -X POST http://localhost:8090/transfer/cash-to-bank \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_123",
    "amount": 100.0,
    "date": "2025-01-19",
    "operation_id": "1737355200000_001",
    "device_id": "test-device-123",
    "timestamp": 1737355200000
  }'

# Expected Response:
# {"success": true, "message": "Cash to bank transfer successful", "data": {...}}
```

**Validation Query:**
```sql
-- Check if sync operation was recorded
SELECT operation_id, operation_type, entity_type, entity_id, device_ids, created_at 
FROM sync_operations 
WHERE user_id = "test_user_123" 
  AND operation_type = "transfer" 
  AND entity_type = "cash_bank_transfers"
ORDER BY created_at DESC LIMIT 1;

-- Should return one record with operation_type = "transfer"
```

### Test 2: Bank-to-Cash Transfer with Sync

```bash
# Test bank-to-cash transfer with sync parameters
curl -X POST http://localhost:8090/transfer/bank-to-cash \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_123",
    "amount": 50.0,
    "date": "2025-01-19",
    "operation_id": "1737355300000_001",
    "device_id": "test-device-123",
    "timestamp": 1737355300000
  }'
```

**Validation Query:**
```sql
-- Check if sync operation was recorded
SELECT operation_id, operation_type, entity_type, entity_id, device_ids 
FROM sync_operations 
WHERE user_id = "test_user_123" 
  AND operation_type = "transfer" 
  AND entity_type = "cash_bank_transfers"
ORDER BY created_at DESC LIMIT 1;
```

### Test 3: Cash Amount Update with Sync

```bash
# Test cash amount update with sync parameters
curl -X POST http://localhost:8090/cash-bank/cash/update \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_123",
    "amount": 250.0,
    "date": "2025-01-19",
    "operation_id": "1737355400000_001",
    "device_id": "test-device-123",
    "timestamp": 1737355400000
  }'
```

**Validation Query:**
```sql
-- Check if sync operation was recorded
SELECT operation_id, operation_type, entity_type, entity_id, device_ids 
FROM sync_operations 
WHERE user_id = "test_user_123" 
  AND operation_type = "update_cash" 
  AND entity_type = "cash_bank_updates"
ORDER BY created_at DESC LIMIT 1;
```

### Test 4: Bank Amount Update with Sync

```bash
# Test bank amount update with sync parameters
curl -X POST http://localhost:8090/cash-bank/bank/update \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_123",
    "amount": 500.0,
    "date": "2025-01-19",
    "operation_id": "1737355500000_001",
    "device_id": "test-device-123",
    "timestamp": 1737355500000
  }'
```

**Validation Query:**
```sql
-- Check if sync operation was recorded
SELECT operation_id, operation_type, entity_type, entity_id, device_ids 
FROM sync_operations 
WHERE user_id = "test_user_123" 
  AND operation_type = "update_bank" 
  AND entity_type = "cash_bank_updates"
ORDER BY created_at DESC LIMIT 1;
```

## Critical Test: Auto-Generation Pattern

### Test 5: Operations WITHOUT Provided operation_id (Auto-Generation Test)

This tests the critical pattern from the implementation guide where operations should ALWAYS be recorded even without sync parameters.

```bash
# Test cash-to-bank transfer WITHOUT operation_id - should auto-generate
curl -X POST http://localhost:8090/transfer/cash-to-bank \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_456",
    "amount": 75.0,
    "date": "2025-01-19",
    "device_id": "test-device-456"
  }'

# Test cash update WITHOUT any sync parameters - should still record
curl -X POST http://localhost:8090/cash-bank/cash/update \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_456",
    "amount": 300.0,
    "date": "2025-01-19",
    "device_id": "test-device-456"
  }'
```

**Critical Validation:**
```sql
-- Verify BOTH operations were recorded despite missing sync parameters
SELECT operation_id, operation_type, entity_type, created_at
FROM sync_operations 
WHERE user_id = "test_user_456"
ORDER BY created_at DESC;

-- Should return TWO records:
-- 1. operation_type = "update_cash", entity_type = "cash_bank_updates"
-- 2. operation_type = "transfer", entity_type = "cash_bank_transfers"
-- Both should have auto-generated operation_id in format: TIMESTAMP_001
```

## Operation ID Format Validation

### Test 6: Verify Operation ID Format

```sql
-- Check that all operation IDs follow the timestamp format: {timestamp_ms}_{sequence}
SELECT operation_id, 
       CASE WHEN operation_id REGEXP '^[0-9]{13}_[0-9]{3}$' 
            THEN 'VALID FORMAT' 
            ELSE 'INVALID FORMAT' 
       END as format_check
FROM sync_operations 
WHERE user_id IN ("test_user_123", "test_user_456")
ORDER BY created_at DESC;

-- ALL records should show 'VALID FORMAT'
```

### Test 7: Chronological Ordering Test

```sql
-- Verify operation IDs are in chronological order
SELECT operation_id, created_at, operation_type
FROM sync_operations 
WHERE user_id = "test_user_123"
ORDER BY operation_id ASC;

-- operation_id values should increase chronologically
-- Earlier operations should have smaller operation_id values
```

## Complete Sync Operations Summary

### Final Validation Query

```sql
-- Get complete summary of all sync operations for test users
SELECT 
  user_id,
  operation_type,
  entity_type,
  COUNT(*) as operation_count,
  MIN(created_at) as first_operation,
  MAX(created_at) as last_operation
FROM sync_operations 
WHERE user_id IN ("test_user_123", "test_user_456")
GROUP BY user_id, operation_type, entity_type
ORDER BY user_id, operation_type;

-- Expected Results:
-- test_user_123: 2x transfer (cash_bank_transfers), 1x update_cash, 1x update_bank
-- test_user_456: 1x transfer (cash_bank_transfers), 1x update_cash
```

## Frontend Integration Testing

### Test 8: React Native Service Integration

If testing with the React Native app:

1. **Make a cash transfer using the app UI**
2. **Check that it appears in sync queue:**
   ```javascript
   // In React Native app console/debug:
   import { StandardSyncQueueService } from './src/services/standardSyncQueueService';
   
   // Check sync queue
   const queueItems = await StandardSyncQueueService.getQueuedOperations();
   console.log('Cash bank operations in queue:', 
     queueItems.filter(item => item.table_name === 'cash_bank'));
   ```

3. **Process the sync queue:**
   ```javascript
   // Trigger sync processing
   await StandardSyncQueueService.processQueue();
   
   // Verify operation was sent to server and recorded in sync_operations table
   ```

## Troubleshooting Common Issues

### Issue 1: Sync Operations Not Being Recorded
- **Check**: Backend logs for error messages during operation recording
- **Verify**: Database schema has proper operation_type constraints
- **Solution**: Ensure all handlers are using the updated versions with sync recording

### Issue 2: Invalid Operation ID Format
- **Check**: Operation IDs in database match pattern `^\d{13}_\d{3}$`
- **Verify**: `generateNextOperationId` function is working correctly
- **Solution**: Check operation ID generation utilities implementation

### Issue 3: Missing Device IDs
- **Check**: Frontend is sending device_id parameter
- **Verify**: Device ID service is working correctly
- **Solution**: Ensure sync parameter injection is working in API layer

## Success Criteria

✅ **All tests pass when:**
1. ALL CRUD operations (transfer, update_cash, update_bank) record sync operations
2. Operations without sync parameters still record sync operations (auto-generation)
3. All operation IDs follow the timestamp-based format
4. Chronological ordering is maintained
5. Frontend sync queue processes cash_bank operations correctly

## Critical Rules Verified

1. **🚨 Consistent Pattern**: ALL handlers follow the same sync recording pattern
2. **🚨 Auto-Generation**: Operations are recorded even without provided sync parameters
3. **🚨 ID Format**: All operation IDs follow timestamp-based format for ordering
4. **🚨 Entity Types**: Proper entity_type classification (cash_bank_transfers vs cash_bank_updates)

---

**Note**: Run all tests in sequence and ensure EVERY validation query returns expected results. Any missing sync operations indicate implementation gaps that must be fixed.