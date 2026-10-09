# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f whatsapp-bot/Dockerfile -t zumba-whatsapp-bot .
# Für den Pi (arm64) lässt es sich auch auf einem amd64-Rechner bauen – ohne
# Emulator, Go kompiliert quer:
#   docker buildx build --platform linux/arm64 -f whatsapp-bot/Dockerfile -t zumba-whatsapp-bot:<tag> --load .
FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app

RUN apk --no-cache add git ca-certificates tzdata

COPY shared/ shared/
COPY whatsapp-bot/go.mod whatsapp-bot/go.sum whatsapp-bot/
WORKDIR /app/whatsapp-bot
RUN go mod download

COPY whatsapp-bot/ .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:3.24.2

# Zertifikate und Zeitzonen aus der Build-Stufe: die Endstufe braucht so kein
# RUN und damit beim Quer-Bauen keinen arm64-Emulator.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /root/

COPY --from=builder /app/whatsapp-bot/server .

EXPOSE 8080

CMD ["./server"]
