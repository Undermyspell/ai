# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f wrapped/Dockerfile -t stammtisch-wrapped .
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

COPY shared/ shared/
COPY wrapped/go.mod wrapped/go.sum wrapped/
WORKDIR /app/wrapped
RUN go mod download

COPY wrapped/ .

# Generate templ code and build the application
RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final Stage
FROM alpine:3.24.1

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder (static assets are embedded via assets/embed.go)
COPY --from=builder /app/wrapped/server .

EXPOSE 8080

CMD ["./server"]
