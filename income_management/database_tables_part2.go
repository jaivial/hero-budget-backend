package main

// CENTRALIZED SCHEMA MIGRATION:
// This file has been replaced - all DDL operations are now centralized
// in backend/database_schema.sql and managed by the centralized database
// initialization service.
//
// Previous DDL operations that were removed:
// - CREATE TABLE monthly_cash_bank_balance
// - CREATE TABLE quarterly_cash_bank_balance
// - CREATE TABLE semiannual_cash_bank_balance
// - CREATE TABLE annual_cash_bank_balance
// - createLongerPeriodTables() function
// - createSemiannualAndAnnualTables() function
//
// All schema definitions are now located in:
// /backend/database_schema.sql
