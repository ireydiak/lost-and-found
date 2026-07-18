FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/serve ./cmd/serve \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3

# ca-certificates: outbound HTTPS (Nominatim geocoding, OAuth providers)
RUN apk add --no-cache ca-certificates

WORKDIR /app
# migrate reads migration SQL from disk; serve has its assets embedded
COPY --from=builder /src/db ./db
COPY --from=builder /out/serve /out/migrate /usr/local/bin/

EXPOSE 8080
CMD ["serve"]
