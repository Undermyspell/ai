# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f zumba-admin-ui/Dockerfile -t zumba-admin-ui .
# Für den Pi (arm64) lässt es sich auch auf einem amd64-Rechner bauen – ohne
# Emulator, Go kompiliert quer:
#   docker buildx build --platform linux/arm64 -f zumba-admin-ui/Dockerfile -t zumba-admin-ui:<tag> --load .
FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app

RUN apk --no-cache add git ca-certificates tzdata
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

COPY shared/ shared/
COPY zumba-admin-ui/go.mod zumba-admin-ui/go.sum zumba-admin-ui/
WORKDIR /app/zumba-admin-ui
RUN go mod download

COPY zumba-admin-ui/ .

RUN templ generate
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:3.24.2

# Zertifikate und Zeitzonen aus der Build-Stufe: die Endstufe braucht so kein
# RUN und damit beim Quer-Bauen keinen arm64-Emulator.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /root/

COPY --from=builder /app/zumba-admin-ui/server .

EXPOSE 8080

CMD ["./server"]
