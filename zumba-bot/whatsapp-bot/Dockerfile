# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f whatsapp-bot/Dockerfile -t zumba-whatsapp-bot .
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git

COPY shared/ shared/
COPY whatsapp-bot/go.mod whatsapp-bot/go.sum whatsapp-bot/
WORKDIR /app/whatsapp-bot
RUN go mod download

COPY whatsapp-bot/ .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:3.24.1

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/whatsapp-bot/server .

EXPOSE 8080

CMD ["./server"]
