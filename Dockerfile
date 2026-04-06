FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pdf-service ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    wkhtmltopdf \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/pdf-service .
COPY settings.yaml .
EXPOSE 8080
CMD ["./pdf-service"]
