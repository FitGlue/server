FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy the entire go source tree
COPY src/go/ ./src/go/

# Build the service
WORKDIR /app/src/go
RUN go build -o /app/bin/service services/registry/main.go

# Minimal runner image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/service /app/service

# Run as non-root
USER 1000:1000

ENV PORT=8083
EXPOSE 8083

CMD ["/app/service"]
