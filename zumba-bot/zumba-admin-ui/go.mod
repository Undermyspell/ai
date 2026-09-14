module github.com/michael/zumba-admin-ui

go 1.27.1

require (
	github.com/a-h/templ v0.3.1020
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.12.3
	github.com/michael/zumba-shared v0.0.0
	github.com/yuin/goldmark v1.8.6
)

replace github.com/michael/zumba-shared => ../shared
