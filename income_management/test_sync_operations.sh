#!/bin/bash

# Test script to verify sync operations are properly recorded for income management
# Based on docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md validation tests

set -e

# Configuration
BASE_URL="http://localhost:8093"
TEST_USER_ID="test_sync_user_$(date +%s)"
TEST_DEVICE_ID="test_device_$(date +%s)"

echo "🧪 Testing sync operations for income management service"
echo "📋 Test User ID: $TEST_USER_ID"
echo "📱 Test Device ID: $TEST_DEVICE_ID"
echo "🌐 Base URL: $BASE_URL"
echo ""

# Function to make API calls with proper error handling
make_api_call() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo "📡 Testing: $description"
    echo "   Method: $method"
    echo "   Endpoint: $endpoint"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$BASE_URL$endpoint")
    fi
    
    # Extract HTTP status code (last line)
    http_code=$(echo "$response" | tail -n1)
    # Extract response body (all lines except last)
    response_body=$(echo "$response" | head -n -1)
    
    echo "   Status: $http_code"
    echo "   Response: $response_body"
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo "   ✅ Success"
    else
        echo "   ❌ Failed with status $http_code"
        return 1
    fi
    
    echo ""
    return 0
}

# Test 1: Create income with sync parameters
echo "🧪 Test 1: Create income operation with sync tracking"
create_data='{
    "user_id": "'$TEST_USER_ID'",
    "amount": 1000.50,
    "date": "2024-01-15",
    "category": "Salary",
    "payment_method": "bank",
    "description": "Test income for sync",
    "device_id": "'$TEST_DEVICE_ID'"
}'

if make_api_call "POST" "/incomes/add" "$create_data" "Create income with sync"; then
    echo "✅ Test 1 PASSED: Income creation with sync parameters"
else
    echo "❌ Test 1 FAILED: Income creation failed"
    exit 1
fi

# Test 2: Update income with sync parameters
echo "🧪 Test 2: Update income operation with sync tracking"
update_data='{
    "user_id": "'$TEST_USER_ID'",
    "income_id": 1,
    "amount": 1200.75,
    "category": "Updated Salary",
    "device_id": "'$TEST_DEVICE_ID'"
}'

if make_api_call "POST" "/incomes/update" "$update_data" "Update income with sync"; then
    echo "✅ Test 2 PASSED: Income update with sync parameters"
else
    echo "❌ Test 2 FAILED: Income update failed"
    exit 1
fi

# Test 3: Delete income with sync parameters
echo "🧪 Test 3: Delete income operation with sync tracking"
delete_data='{
    "user_id": "'$TEST_USER_ID'",
    "income_id": 1,
    "device_id": "'$TEST_DEVICE_ID'"
}'

if make_api_call "POST" "/incomes/delete" "$delete_data" "Delete income with sync"; then
    echo "✅ Test 3 PASSED: Income deletion with sync parameters"
else
    echo "❌ Test 3 FAILED: Income deletion failed"
    exit 1
fi

echo "🎉 All tests completed successfully!"
echo ""
echo "📊 Next Steps:"
echo "1. Check database to verify sync operations were recorded:"
echo "   sqlite3 /path/to/database.db \"SELECT operation_type, operation_id, entity_id FROM sync_operations WHERE user_id='$TEST_USER_ID' ORDER BY created_at DESC LIMIT 10;\""
echo ""
echo "2. Verify operation ID format (should be timestamp_sequence):"
echo "   sqlite3 /path/to/database.db \"SELECT operation_id FROM sync_operations WHERE operation_id NOT REGEXP '^[0-9]{13}_[0-9]{3}$';\""
echo ""
echo "3. Test incremental sync processing:"
echo "   Use the delta sync endpoints to fetch and process these operations on another device"
echo ""
echo "📋 Expected Results:"
echo "   - 3 sync operations recorded (create, update, delete)"
echo "   - All operation IDs follow timestamp_sequence format"
echo "   - All operations have proper device_id tracking"
echo "   - Operations are chronologically ordered by operation_id"