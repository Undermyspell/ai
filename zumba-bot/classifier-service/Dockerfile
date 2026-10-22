# Eigenständiges Modul – Build-Kontext ist classifier-service/ selbst:
#   docker build -t zumba-classifier .
# Für den Pi (arm64) auch auf amd64 ohne Emulator, Go kompiliert quer:
#   docker buildx build --platform linux/arm64 -t zumba-classifier:<tag> --load .
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:3.24.3

# Kein RUN in der Endstufe: so braucht das Quer-Bauen keinen arm64-Emulator.
# Der Service spricht nur HTTP im Cluster, Zertifikate/Zeitzonen braucht er nicht.
WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
