module backend/dashboard_data

go 1.21

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/herobudget/backend/common v0.0.0
	github.com/mattn/go-sqlite3 v1.14.27
)

replace github.com/herobudget/backend/common => ../common
