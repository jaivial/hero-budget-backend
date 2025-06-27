module backend/profile_management

go 1.21

require (
	github.com/mattn/go-sqlite3 v1.14.27
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/joho/godotenv v1.5.1
	github.com/herobudget/backend/common v0.0.0
)

replace github.com/herobudget/backend/common => ../common
