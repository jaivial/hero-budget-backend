-- Fix bill_payments table schema
-- Add missing columns for proper bill payment tracking

-- First, check if user_id column exists
ALTER TABLE bill_payments ADD COLUMN user_id TEXT;

-- Add amount column to track payment amounts
ALTER TABLE bill_payments ADD COLUMN amount REAL DEFAULT 0.0;

-- Update existing records to have proper user_id values
-- This will need to be done by joining with bills table
UPDATE bill_payments 
SET user_id = (
    SELECT b.user_id 
    FROM bills b 
    WHERE b.id = bill_payments.bill_id
)
WHERE user_id IS NULL;

-- Update existing records to have proper amount values
-- This will use the bill amount as default
UPDATE bill_payments 
SET amount = (
    SELECT b.amount 
    FROM bills b 
    WHERE b.id = bill_payments.bill_id
)
WHERE amount = 0.0;

-- Create index for better performance
CREATE INDEX IF NOT EXISTS idx_bill_payments_user_id ON bill_payments(user_id);
CREATE INDEX IF NOT EXISTS idx_bill_payments_year_month ON bill_payments(year_month);