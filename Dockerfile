# Dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o orchestrator

FROM debian:bullseye-slim
WORKDIR /root/
COPY --from=builder /app/orchestrator .
EXPOSE 8080
CMD ["./orchestrator"]
