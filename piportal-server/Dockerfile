# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags "-s -w" -o piportal-server .

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/piportal-server .

# Create data directory
RUN mkdir -p /data

ENV PIPORTAL_DB=/data/piportal.db

EXPOSE 80 443

ENTRYPOINT ["./piportal-server"]
