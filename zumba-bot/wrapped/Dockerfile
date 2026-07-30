# Build Stage
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate templ code and build the application
RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final Stage
FROM alpine:3.24.1

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder (static assets are embedded via assets/embed.go)
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
