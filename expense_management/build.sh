#!/bin/bash

echo "Building expense_management service..."
go build -o expense_management main_part1.go main_part2_tables.go main_part3_handlers.go main_part4_database.go main_part5_balance_updates.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful"
    echo "Starting service with --produccion flag..."
    ./expense_management --produccion
else
    echo "❌ Build failed"
    exit 1
fi