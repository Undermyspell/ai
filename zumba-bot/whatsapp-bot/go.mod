module github.com/michael/zumba-whatsapp-bot

go 1.26.5

require (
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.12.3
)

require github.com/michael/zumba-shared v0.0.0

replace github.com/michael/zumba-shared => ../shared
