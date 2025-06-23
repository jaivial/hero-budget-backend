package main

// CENTRALIZED SCHEMA MIGRATION:
// This file has been replaced - all DDL operations are now centralized
// in backend/database_schema.sql and managed by the centralized database
// initialization service.
//
// Previous DDL operations that were removed:
// - CREATE TABLE IF NOT EXISTS budget
// - CREATE TABLE IF NOT EXISTS savings
// - ALTER TABLE savings ADD COLUMN period TEXT
// - CREATE TABLE IF NOT EXISTS cash_bank
// - All DDL operations that were in createTablesIfNotExist()
//
// All schema definitions are now located in:
// /backend/database_schema.sql