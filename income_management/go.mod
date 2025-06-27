module backend/income_management

go 1.21

require (
	github.com/mattn/go-sqlite3 v1.14.27
	github.com/joho/godotenv v1.5.1
	github.com/herobudget/backend/common v0.0.0
)

replace github.com/herobudget/backend/common => ../common
