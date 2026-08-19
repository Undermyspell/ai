# Build-Kontext ist zumba-bot/ (wegen des shared-Moduls):
#   docker build -f zumba-admin-ui/Dockerfile -t zumba-admin-ui .
FROM golang:1.27.0-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

COPY shared/ shared/
COPY zumba-admin-ui/go.mod zumba-admin-ui/go.sum zumba-admin-ui/
WORKDIR /app/zumba-admin-ui
RUN go mod download

COPY zumba-admin-ui/ .

RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:3.24.1

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/zumba-admin-ui/server .

EXPOSE 8080

CMD ["./server"]
