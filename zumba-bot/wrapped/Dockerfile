# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f wrapped/Dockerfile -t stammtisch-wrapped .
# Für den Pi (arm64) lässt es sich auch auf einem amd64-Rechner bauen – ohne
# Emulator, Go kompiliert quer:
#   docker buildx build --platform linux/arm64 -f wrapped/Dockerfile -t zumba-wrapped:<tag> --load .
FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app

RUN apk --no-cache add git ca-certificates
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

COPY shared/ shared/
COPY wrapped/go.mod wrapped/go.sum wrapped/
WORKDIR /app/wrapped
RUN go mod download

COPY wrapped/ .

# Generate templ code and build the application
RUN templ generate
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o server ./cmd/server

# Final Stage
FROM alpine:3.24.2

# Zertifikate aus der Build-Stufe: die Endstufe braucht so kein RUN und damit
# beim Quer-Bauen keinen arm64-Emulator.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

WORKDIR /root/

# Copy binary from builder (static assets are embedded via assets/embed.go)
COPY --from=builder /app/wrapped/server .

EXPOSE 8080

CMD ["./server"]
