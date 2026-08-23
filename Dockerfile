# ---- build stage ----
# Compiles the Go binary. CGO is enabled because the SQLite driver (mattn/
# go-sqlite3) requires it; the builder image ships a C toolchain.
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /out/api-mock ./cmd/server

# ---- runtime stage ----
# Distroless-style minimal image: just the binary plus a writable dir for the
# SQLite database file. No shell, smallest attack surface.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates libc6-compat && \
    adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/api-mock /app/api-mock

# SQLite file lives here (volume-mountable).
RUN mkdir -p /app/data && chown -R app:app /app
USER app
ENV DB_DSN=/app/data/api_mock.db

EXPOSE 8080
ENTRYPOINT ["/app/api-mock"]
