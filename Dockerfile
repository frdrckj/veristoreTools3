# ---- Build stage ----
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o migrate ./cmd/migrate

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Jakarta

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrate .
COPY --from=builder /app/static ./static
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config.yaml ./config.yaml

EXPOSE 8080

CMD ["./server"]
