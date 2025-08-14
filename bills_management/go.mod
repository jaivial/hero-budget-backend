module backend/bills_management

go 1.21

require (
	github.com/herobudget/backend/common v0.0.0
	github.com/joho/godotenv v1.5.1
	github.com/mattn/go-sqlite3 v1.14.27
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
)

replace github.com/herobudget/backend/common => ../common
