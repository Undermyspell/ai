module github.com/michael/stammtisch-wrapped

go 1.26.5

require (
	github.com/a-h/templ v0.3.1020
	github.com/lib/pq v1.12.3
)

require github.com/michael/zumba-shared v0.0.0

replace github.com/michael/zumba-shared => ../shared
