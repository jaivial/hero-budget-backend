#!/bin/bash

echo "🧪 Testing Operation ID System for Bills Management"
echo "================================================="

# Test variables
USER_ID="test_user_123"
BASE_URL="http://localhost:8091"

echo "📝 1. Testing adding a bill with operation_id generation..."

# Create a test bill
RESPONSE=$(curl -s -X POST "$BASE_URL/bills/add" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "'$USER_ID'",
    "name": "Test Bill - Operation ID",
    "amount": 100.50,
    "due_date": "2025-01-15",
    "category": "utilities",
    "icon": "💡",
    "start_date": "2025-01-01",
    "payment_day": 15,
    "duration_months": 12,
    "regularity": "monthly",
    "payment_method": "bank",
    "operation_id": "",
    "device_id": "test_device_001",
    "timestamp": '$(($(date +%s)*1000))'
  }')

echo "Response: $RESPONSE"
echo ""

echo "📋 2. Testing sync changes endpoint with operation_id..."

# Test fetching operations for the user
SYNC_RESPONSE=$(curl -s "$BASE_URL/bills/sync/changes?user_id=$USER_ID&limit=10")

echo "Sync Response: $SYNC_RESPONSE"
echo ""

echo "✅ Operation ID system test completed!"
echo "Check the logs to see operation_id generation in action."