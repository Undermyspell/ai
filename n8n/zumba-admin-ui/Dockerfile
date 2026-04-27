FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
