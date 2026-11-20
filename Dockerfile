# Stage 1: Build binary
FROM golang:1.23-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/seed ./cmd/seed

# Stage 2: Runtime gọn nhẹ
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /app
COPY --from=builder /app/bin/api .
COPY --from=builder /app/bin/seed .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080
CMD ["./api"]
