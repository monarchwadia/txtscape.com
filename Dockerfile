FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /txtscape ./cmd/txtscape

FROM alpine:3.20

RUN apk --no-cache add ca-certificates postgresql-client

COPY --from=builder /txtscape /txtscape
COPY migrations/ /migrations/

EXPOSE 8080

ENTRYPOINT ["/txtscape"]
