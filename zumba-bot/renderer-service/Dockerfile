# Build-Kontext ist renderer-service/ selbst (kein shared-Modul nötig):
#   docker build -f renderer-service/Dockerfile renderer-service/
FROM golang:1.26.5-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Debian statt Alpine: chromium + fonts-noto-color-emoji sind hier auf
# amd64 UND arm64 (Pi) verlässlich paketiert.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        chromium \
        fonts-noto-core \
        fonts-noto-color-emoji \
        ca-certificates \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

ENV TZ=Europe/Berlin \
    CHROME_PATH=/usr/bin/chromium

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
