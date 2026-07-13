FROM golang:1.26-alpine AS builder

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

FROM alpine:latest

RUN apk add --no-cache make

COPY --from=builder /go/bin/goose /usr/local/bin/goose

WORKDIR /app

COPY db/ db/
COPY Makefile .

CMD ["make", "migrate"]
