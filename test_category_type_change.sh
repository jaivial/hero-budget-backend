#!/bin/bash

# Test script for Category Type Change with Cascade Re-calculation
# Tests the two new endpoints:
# 1. GET /categories/check-transactions
# 2. POST /categories/update-with-type-change

echo "🧪 ============================================================"
echo "🧪 Testing Category Type Change with Cascade Re-calculation"
echo "🧪 ============================================================"
echo ""

# Configuration
BASE_URL="http://localhost:8096"
USER_ID="test_user_123"
TEST_CATEGORY_ID=1

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print test section
print_section() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📋 $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# Function to print test result
print_result() {
    local status=$1
    local message=$2

    if [ "$status" = "pass" ]; then
        echo -e "${GREEN}✅ PASS${NC}: $message"
    elif [ "$status" = "fail" ]; then
        echo -e "${RED}❌ FAIL${NC}: $message"
    elif [ "$status" = "warn" ]; then
        echo -e "${YELLOW}⚠️  WARN${NC}: $message"
    else
        echo "ℹ️  INFO: $message"
    fi
}

# Function to test endpoint with GET method
test_get_endpoint() {
    local endpoint=$1
    local params=$2
    local description=$3

    echo ""
    echo "🔍 Testing: $description"
    echo "   Endpoint: GET $endpoint"
    echo "   Params: $params"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint?$params" 2>/dev/null)
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    echo "   Status Code: $status_code"
    echo "   Response Body:"
    echo "$body" | jq '.' 2>/dev/null || echo "$body"

    if [ "$status_code" = "200" ]; then
        print_result "pass" "Endpoint responded successfully"
        return 0
    else
        print_result "fail" "Expected 200, got $status_code"
        return 1
    fi
}

# Function to test endpoint with POST method
test_post_endpoint() {
    local endpoint=$1
    local data=$2
    local description=$3

    echo ""
    echo "🔍 Testing: $description"
    echo "   Endpoint: POST $endpoint"
    echo "   Request Body:"
    echo "$data" | jq '.' 2>/dev/null || echo "$data"

    response=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d "$data" \
        "$BASE_URL$endpoint" 2>/dev/null)

    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    echo "   Status Code: $status_code"
    echo "   Response Body:"
    echo "$body" | jq '.' 2>/dev/null || echo "$body"

    if [ "$status_code" = "200" ]; then
        print_result "pass" "Endpoint responded successfully"
        return 0
    else
        print_result "fail" "Expected 200, got $status_code"
        return 1
    fi
}

# ============================================================================
# TEST SUITE
# ============================================================================

print_section "Test 1: Check if categories service is running"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" 2>/dev/null)
if [ "$response" = "200" ] || [ "$response" = "404" ]; then
    print_result "pass" "Categories service is reachable on port 8096"
else
    print_result "fail" "Categories service not reachable (status: $response)"
    print_result "warn" "Make sure the categories service is running on port 8096"
    exit 1
fi

# ============================================================================
print_section "Test 2: Check Category Transactions Endpoint (GET)"

# Test 2.1: Valid request with category that has no transactions
test_get_endpoint \
    "/categories/check-transactions" \
    "category_id=999&user_id=$USER_ID" \
    "Check transactions for category with ID 999 (likely no transactions)"

# Test 2.2: Valid request with category that might have transactions
test_get_endpoint \
    "/categories/check-transactions" \
    "category_id=$TEST_CATEGORY_ID&user_id=$USER_ID" \
    "Check transactions for category with ID $TEST_CATEGORY_ID"

# Test 2.3: Missing category_id parameter
echo ""
echo "🔍 Testing: Check transactions with missing category_id (should fail)"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/categories/check-transactions?user_id=$USER_ID" 2>/dev/null)
status_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)
echo "   Status Code: $status_code"
echo "   Response: $body"
if [ "$status_code" != "200" ]; then
    print_result "pass" "Correctly rejected request with missing category_id"
else
    print_result "warn" "Endpoint should validate required parameters"
fi

# Test 2.4: Missing user_id parameter
echo ""
echo "🔍 Testing: Check transactions with missing user_id (should fail)"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/categories/check-transactions?category_id=$TEST_CATEGORY_ID" 2>/dev/null)
status_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)
echo "   Status Code: $status_code"
echo "   Response: $body"
if [ "$status_code" != "200" ]; then
    print_result "pass" "Correctly rejected request with missing user_id"
else
    print_result "warn" "Endpoint should validate required parameters"
fi

# ============================================================================
print_section "Test 3: Update Category With Type Change Endpoint (POST)"

# Test 3.1: Type change without transactions (expense → income)
request_data=$(cat <<EOF
{
  "user_id": "$USER_ID",
  "category_id": 999,
  "old_type": "expense",
  "new_type": "income",
  "name": "Test Category Updated",
  "emoji": "🎯",
  "operation_id": "test_op_$(date +%s)",
  "device_id": "test_device_123",
  "timestamp": $(date +%s)
}
EOF
)

test_post_endpoint \
    "/categories/update-with-type-change" \
    "$request_data" \
    "Update category type from expense to income (no transactions)"

# Test 3.2: Type change with transactions (income → expense)
request_data=$(cat <<EOF
{
  "user_id": "$USER_ID",
  "category_id": $TEST_CATEGORY_ID,
  "old_type": "income",
  "new_type": "expense",
  "name": "Salary Category",
  "emoji": "💰",
  "operation_id": "test_op_$(date +%s)",
  "device_id": "test_device_123",
  "timestamp": $(date +%s)
}
EOF
)

test_post_endpoint \
    "/categories/update-with-type-change" \
    "$request_data" \
    "Update category type from income to expense (might have transactions)"

# Test 3.3: No type change (same type - should use simple update flow)
request_data=$(cat <<EOF
{
  "user_id": "$USER_ID",
  "category_id": $TEST_CATEGORY_ID,
  "old_type": "expense",
  "new_type": "expense",
  "name": "Updated Name Only",
  "emoji": "📝",
  "operation_id": "test_op_$(date +%s)",
  "device_id": "test_device_123",
  "timestamp": $(date +%s)
}
EOF
)

test_post_endpoint \
    "/categories/update-with-type-change" \
    "$request_data" \
    "Update category without type change (should skip cascade)"

# Test 3.4: Missing required fields
echo ""
echo "🔍 Testing: Update with missing user_id (should fail)"
request_data=$(cat <<EOF
{
  "category_id": $TEST_CATEGORY_ID,
  "old_type": "expense",
  "new_type": "income"
}
EOF
)
response=$(curl -s -w "\n%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$request_data" \
    "$BASE_URL/categories/update-with-type-change" 2>/dev/null)
status_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)
echo "   Status Code: $status_code"
echo "   Response: $body"
if [ "$status_code" != "200" ]; then
    print_result "pass" "Correctly rejected request with missing user_id"
else
    print_result "warn" "Endpoint should validate required parameters"
fi

# Test 3.5: Invalid JSON
echo ""
echo "🔍 Testing: Update with invalid JSON (should fail)"
response=$(curl -s -w "\n%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "{invalid json}" \
    "$BASE_URL/categories/update-with-type-change" 2>/dev/null)
status_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)
echo "   Status Code: $status_code"
echo "   Response: $body"
if [ "$status_code" != "200" ]; then
    print_result "pass" "Correctly rejected invalid JSON"
else
    print_result "warn" "Endpoint should validate JSON format"
fi

# ============================================================================
print_section "Test 4: Sync Operation Recording Verification"

echo ""
echo "📝 After running the type change operations above, verify that:"
echo "   1. Sync operations were recorded in sync_operations table"
echo "   2. Operation type is 'update_with_type_change'"
echo "   3. Operation data contains old_type and new_type"
echo "   4. Device ID and timestamp were recorded correctly"
echo ""
print_result "info" "Check database manually: SELECT * FROM sync_operations WHERE operation_type = 'update_with_type_change' ORDER BY created_at DESC LIMIT 5;"

# ============================================================================
print_section "Test 5: Monthly Balance Cascade Verification"

echo ""
echo "📊 After running type change operations, verify monthly_cash_bank_balance table:"
echo "   1. Income/expense columns were updated correctly"
echo "   2. Cumulative balance columns were recalculated"
echo "   3. Previous balance columns reflect cascade changes"
echo "   4. All months from first transaction onwards were updated"
echo ""
print_result "info" "Check database manually: SELECT year_month, income_bank_amount, expense_bank_amount, total_balance FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID' ORDER BY year_month;"

# ============================================================================
print_section "Test Summary"

echo ""
echo "🎯 All endpoint connectivity tests completed!"
echo ""
echo "📋 Manual Verification Checklist:"
echo "   [ ] Backend endpoints are accessible and responding"
echo "   [ ] Check transactions endpoint returns correct counts"
echo "   [ ] Type change endpoint executes without errors"
echo "   [ ] Sync operations are recorded in database"
echo "   [ ] Monthly balances are recalculated correctly"
echo "   [ ] Frontend API methods work correctly"
echo "   [ ] Incremental sync service handles the new operation type"
echo "   [ ] Cross-device sync works (test on two devices)"
echo ""
echo "🔍 Next Steps:"
echo "   1. Review test output above for any failures"
echo "   2. Check backend service logs for detailed execution flow"
echo "   3. Verify database state matches expected results"
echo "   4. Test the feature from the mobile app UI"
echo "   5. Test cross-device synchronization"
echo ""
echo "✅ Backend API testing script completed!"
echo ""