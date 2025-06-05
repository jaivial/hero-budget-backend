module hero_budget_backend

go 1.21

require (
	github.com/chai2010/webp v1.4.0
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect

replace github.com/chai2010/webp => ../vendor/github.com/chai2010/webp
