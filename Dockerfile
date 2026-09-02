# --- Build stage -----------------------------------------------------------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependency downloads separately from source changes.
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# --- Run stage ---------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /server /app/server
# Swagger UI + openapi.yaml, served at /docs (see cmd/server/main.go).
COPY docs ./docs

# Railway injects PORT at runtime — main.go already reads it via
# config.Get().Port, which falls back to 8080 if unset (e.g. local `go run`).
EXPOSE 8080

CMD ["/app/server"]
