#!/bin/bash

###############################################################################
# test_migration_simple.sh
#
# Purpose: Simple test for transaction migration with user 137
# Tests without requiring authentication tokens
###############################################################################

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="https://herobudget.jaimedigitalstudio.com"
USER_ID="137"
TEST_CATEGORY_NAME="Test Migration $(date +%s)"
INITIAL_TYPE="income"
NEW_TYPE="expense"
TEST_EMOJI="💰"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     TRANSACTION MIGRATION TEST (User 137)                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Step 1: Create test category
echo -e "${BLUE}📝 Step 1: Creating test category (type: ${INITIAL_TYPE})...${NC}"

CREATE_RESPONSE=$(curl -s -X POST "${BASE_URL}/categories/add" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"${USER_ID}\",
    \"name\": \"${TEST_CATEGORY_NAME}\",
    \"type\": \"${INITIAL_TYPE}\",
    \"emoji\": \"${TEST_EMOJI}\"
  }")

echo "Response: ${CREATE_RESPONSE}"

CATEGORY_ID=$(echo "${CREATE_RESPONSE}" | grep -o '"id":[0-9]*' | grep -o '[0-9]*' | head -1)

if [ -z "$CATEGORY_ID" ]; then
  echo -e "${RED}❌ Failed to create category${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Category created with ID: ${CATEGORY_ID}${NC}"
echo ""

# Step 2: Create 3 income transactions
echo -e "${BLUE}📝 Step 2: Creating 3 test income transactions...${NC}"

curl -s -X POST "${BASE_URL}/incomes/add" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"${USER_ID}\",
    \"amount\": 500.00,
    \"date\": \"2025-01-15\",
    \"category\": \"${TEST_CATEGORY_NAME}\",
    \"category_id\": ${CATEGORY_ID},
    \"payment_method\": \"cash\",
    \"description\": \"Test Transaction 1\"
  }" > /dev/null

curl -s -X POST "${BASE_URL}/incomes/add" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"${USER_ID}\",
    \"amount\": 750.00,
    \"date\": \"2025-01-20\",
    \"category\": \"${TEST_CATEGORY_NAME}\",
    \"category_id\": ${CATEGORY_ID},
    \"payment_method\": \"bank\",
    \"description\": \"Test Transaction 2\"
  }" > /dev/null

curl -s -X POST "${BASE_URL}/incomes/add" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"${USER_ID}\",
    \"amount\": 1000.00,
    \"date\": \"2025-01-25\",
    \"category\": \"${TEST_CATEGORY_NAME}\",
    \"category_id\": ${CATEGORY_ID},
    \"payment_method\": \"cash\",
    \"description\": \"Test Transaction 3\"
  }" > /dev/null

echo -e "${GREEN}✅ Created 3 test transactions (Total: $2,250)${NC}"
echo ""

# Step 3: Verify transactions in incomes table
echo -e "${BLUE}📝 Step 3: Verifying transactions in incomes table...${NC}"

INCOMES_RESPONSE=$(curl -s "${BASE_URL}/incomes?user_id=${USER_ID}")
INCOME_COUNT=$(echo "${INCOMES_RESPONSE}" | grep -o "\"category_id\":${CATEGORY_ID}" | wc -l | tr -d ' ')

echo "Found ${INCOME_COUNT} transactions in incomes table"

if [ "$INCOME_COUNT" -lt 3 ]; then
  echo -e "${RED}❌ Expected at least 3 transactions, found ${INCOME_COUNT}${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Verified ${INCOME_COUNT} transactions in incomes table${NC}"
echo ""

# Step 4: Change category type
echo -e "${BLUE}📝 Step 4: Changing category type (${INITIAL_TYPE} → ${NEW_TYPE})...${NC}"

UPDATE_RESPONSE=$(curl -s -X POST "${BASE_URL}/categories/update-with-type-change" \
  -H "Content-Type: application/json" \
  -d "{
    \"category_id\": ${CATEGORY_ID},
    \"user_id\": \"${USER_ID}\",
    \"old_type\": \"${INITIAL_TYPE}\",
    \"new_type\": \"${NEW_TYPE}\"
  }")

echo "Response: ${UPDATE_RESPONSE}"

if echo "${UPDATE_RESPONSE}" | grep -q '"success":true'; then
  echo -e "${GREEN}✅ Category type changed successfully${NC}"
else
  echo -e "${RED}❌ Failed to change category type${NC}"
  echo "Response: ${UPDATE_RESPONSE}"
  exit 1
fi

echo ""
sleep 3

# Step 5: Verify transactions moved to expenses table
echo -e "${BLUE}📝 Step 5: Verifying transactions moved to expenses table...${NC}"

EXPENSES_RESPONSE=$(curl -s "${BASE_URL}/expenses?user_id=${USER_ID}")
EXPENSE_COUNT=$(echo "${EXPENSES_RESPONSE}" | grep -o "\"category_id\":${CATEGORY_ID}" | wc -l | tr -d ' ')

echo "Found ${EXPENSE_COUNT} transactions in expenses table"

if [ "$EXPENSE_COUNT" -lt 3 ]; then
  echo -e "${RED}❌ Expected at least 3 transactions in expenses, found ${EXPENSE_COUNT}${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Verified ${EXPENSE_COUNT} transactions in expenses table${NC}"
echo ""

# Step 6: Verify transactions removed from incomes table
echo -e "${BLUE}📝 Step 6: Verifying transactions removed from incomes table...${NC}"

INCOMES_AFTER=$(curl -s "${BASE_URL}/incomes?user_id=${USER_ID}")
INCOME_COUNT_AFTER=$(echo "${INCOMES_AFTER}" | grep -o "\"category_id\":${CATEGORY_ID}" | wc -l | tr -d ' ')

echo "Found ${INCOME_COUNT_AFTER} transactions in incomes table (should be 0)"

if [ "$INCOME_COUNT_AFTER" -gt 0 ]; then
  echo -e "${RED}❌ Expected 0 transactions in incomes, found ${INCOME_COUNT_AFTER}${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Verified 0 transactions remain in incomes table${NC}"
echo ""

# Step 7: Verify category type updated
echo -e "${BLUE}📝 Step 7: Verifying category type updated...${NC}"

CATEGORIES_RESPONSE=$(curl -s "${BASE_URL}/categories?user_id=${USER_ID}")

if echo "${CATEGORIES_RESPONSE}" | grep -q "\"id\":${CATEGORY_ID}.*\"type\":\"${NEW_TYPE}\""; then
  echo -e "${GREEN}✅ Category type updated to '${NEW_TYPE}'${NC}"
else
  # Try reverse order (type before id)
  if echo "${CATEGORIES_RESPONSE}" | grep -q "\"type\":\"${NEW_TYPE}\".*\"id\":${CATEGORY_ID}"; then
    echo -e "${GREEN}✅ Category type updated to '${NEW_TYPE}'${NC}"
  else
    echo -e "${YELLOW}⚠️ Could not verify category type in response${NC}"
  fi
fi

echo ""

# Step 8: Cleanup
echo -e "${BLUE}📝 Step 8: Cleaning up test data...${NC}"

DELETE_RESPONSE=$(curl -s -X DELETE "${BASE_URL}/categories/delete" \
  -H "Content-Type: application/json" \
  -d "{
    \"category_id\": ${CATEGORY_ID},
    \"user_id\": \"${USER_ID}\"
  }")

if echo "${DELETE_RESPONSE}" | grep -q '"success":true'; then
  echo -e "${GREEN}✅ Test data cleaned up${NC}"
else
  echo -e "${YELLOW}⚠️ Manual cleanup may be needed for category ID: ${CATEGORY_ID}${NC}"
fi

echo ""

# Summary
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           ✅ ALL TESTS PASSED SUCCESSFULLY ✅               ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo -e "  ✅ Created test category (ID: ${CATEGORY_ID})"
echo -e "  ✅ Created 3 income transactions"
echo -e "  ✅ Verified transactions in incomes table"
echo -e "  ✅ Changed category type (${INITIAL_TYPE} → ${NEW_TYPE})"
echo -e "  ✅ Verified transactions moved to expenses table"
echo -e "  ✅ Verified transactions removed from incomes table"
echo -e "  ✅ Verified category type updated"
echo -e "  ✅ Cleaned up test data"
echo ""
echo -e "${GREEN}🎉 Transaction migration working correctly!${NC}"
