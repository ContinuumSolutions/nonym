# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Download dependencies first (cached layer unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# modernc.org/sqlite is pure Go — CGO is not needed
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ek1 ./cmd/ek1


# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates: HTTPS calls to third-party APIs (Gmail, Plaid, etc.)
# tzdata: IANA timezone support (profile connection settings)
RUN apk add --no-cache ca-certificates tzdata

# /data is the working directory — SQLite writes ek1.db here.
# Mount a named volume here to persist the database across restarts.
RUN mkdir /data

COPY --from=builder /ek1 /usr/local/bin/ek1

WORKDIR /data
EXPOSE 3000

CMD ["ek1"]
