package main

// CENTRALIZED SCHEMA MIGRATION:
// This file has been replaced - all DDL operations are now centralized
// in backend/database_schema.sql and managed by the centralized database
// initialization service.
//
// Previous DDL operations that were removed:
// - CREATE TABLE daily_cash_bank_balance
// - CREATE TABLE weekly_cash_bank_balance
// - createDailyBalanceIndices()
// - createWeeklyBalanceIndices()
// - createMonthlyAndLongerPeriodTables() call
//
// All schema definitions are now located in:
// /backend/database_schema.sql