# Category ID Migration - Deployment Guide

## Overview
This guide documents the deployment process for the category ID migration that enables automatic category name synchronization in transactions.

## Migration Summary
- **Purpose**: Add category_id foreign key relationships between categories and transactions (incomes/expenses)
- **Services Affected**: `categories_management`, `income_management`, `expense_management`
- **Database Changes**: Added `category_id INTEGER` columns to `incomes` and `expenses` tables
- **Deployment Method**: Automatic migration on service startup

## Pre-Deployment Checklist

### 1. Code Verification
All services include the category_id migration:
- ✅ Categories Management: Transaction update functionality added
- ✅ Income Management: category_id field support added to all CRUD operations
- ✅ Expense Management: category_id field support added to all CRUD operations
- ✅ Database Schema: category_id columns and indexes defined in `database_schema.sql`
- ✅ Migration Script: Population logic included in `database_migration.sql`

### 2. Build Verification
All services compile successfully:
```bash
cd backend/categories_management && go build .
cd backend/income_management && go build .
cd backend/expense_management && go build .
```

## Deployment Process

### Automatic Migration (Recommended)
The migration runs automatically when services start up. Each service includes:

1. **Migration on Startup**: All three services execute migration scripts on initialization
2. **Idempotent Operations**: Migration can be run multiple times safely
3. **Error Handling**: Services handle "column already exists" errors gracefully
4. **Verification**: Services log migration statistics and success/failure status

### Using Existing Deployment Script
The standard deployment process using `deploybackend.sh` will work correctly:

1. Select services to deploy (10, 11, 13 for income, expense, categories)
2. Services will be compiled and restarted
3. Migration will run automatically on service startup
4. Verify endpoints are working correctly

### Manual Migration (If Needed)
If manual migration is required, execute the database migration script directly:

```bash
sqlite3 /opt/hero_budget/backend/budget_data.db < database_migration.sql
```

## Migration Details

### Database Schema Changes
```sql
-- Add category_id columns
ALTER TABLE incomes ADD COLUMN category_id INTEGER;
ALTER TABLE expenses ADD COLUMN category_id INTEGER;

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_incomes_category_id ON incomes(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_category_id ON expenses(category_id);

-- Populate existing records
UPDATE incomes SET category_id = (
    SELECT c.id FROM categories c
    WHERE c.name = incomes.category
    AND c.user_id = incomes.user_id
    AND c.type = 'income' LIMIT 1
) WHERE category_id IS NULL AND category IS NOT NULL;

UPDATE expenses SET category_id = (
    SELECT c.id FROM categories c
    WHERE c.name = expenses.category
    AND c.user_id = expenses.user_id
    AND c.type = 'expense' LIMIT 1
) WHERE category_id IS NULL AND category IS NOT NULL;
```

### Service-Level Changes

#### Categories Management
- Added `updateTransactionsCategoryName()` function
- Integrated automatic transaction updates in category update handler
- Dual update strategy: primary via category_id, fallback via category name

#### Income Management
- Updated `Income` struct to include `CategoryID *int`
- Updated all database operations (INSERT, UPDATE, SELECT) to handle category_id
- Updated sync operation data to include category_id

#### Expense Management
- Updated `Expense` struct to include `CategoryID *int`
- Updated all database operations (INSERT, UPDATE, SELECT) to handle category_id
- Updated sync operation data to include category_id

## Post-Deployment Verification

### 1. Service Health Checks
Verify all services start successfully and migration completes:
```bash
# Check service logs for migration success
journalctl -u income_management -f
journalctl -u expense_management -f
journalctl -u categories_management -f
```

Look for migration success messages:
- `✅ Database migration completed successfully`
- Migration statistics showing records processed

### 2. Endpoint Testing
Test category update functionality:
```bash
# Test category update
curl -X POST http://localhost:8096/categories/update \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user",
    "category_id": 1,
    "name": "Updated Category Name"
  }'
```

### 3. Database Verification
Check that category_id columns exist and are populated:
```sql
-- Verify schema changes
.schema incomes
.schema expenses

-- Check data population
SELECT COUNT(*) as total, COUNT(category_id) as with_category_id
FROM incomes;

SELECT COUNT(*) as total, COUNT(category_id) as with_category_id
FROM expenses;
```

## Rollback Plan

### If Migration Fails
1. **Stop affected services**
2. **Restore database backup** (if available)
3. **Deploy previous version** of services
4. **Investigate migration errors** in service logs

### If Services Don't Start
1. **Check compilation errors** in deployment logs
2. **Verify database connectivity**
3. **Check file permissions** on database files
4. **Review service configuration** and environment variables

## Feature Benefits

### Automatic Category Name Synchronization
- When a category name is updated, all related transactions automatically update
- Maintains data consistency across incomes and expenses
- Dual update strategy ensures compatibility with existing data

### Improved Data Integrity
- Foreign key relationships between categories and transactions
- Better query performance with indexed category_id columns
- Consistent category management across all transaction types

### Backward Compatibility
- Existing category name fields are preserved
- Fallback logic handles records without category_id
- Gradual migration approach minimizes disruption

## Technical Notes

### Migration Safety Features
- **Idempotent**: Can be run multiple times without errors
- **Non-destructive**: Preserves all existing data
- **Graceful Error Handling**: Logs warnings instead of failing
- **Verification**: Provides statistics on migration results

### Performance Considerations
- **Indexed Columns**: category_id columns are automatically indexed
- **Efficient Queries**: Uses category_id for faster lookups where available
- **Minimal Impact**: Migration runs only on service startup

## Contact Information
For deployment issues or questions, refer to the main deployment documentation or check service logs for detailed error information.

---
**Generated**: 2025-09-29
**Migration Version**: 1.0
**Services**: categories_management, income_management, expense_management