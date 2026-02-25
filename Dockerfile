FROM golang:1.24-bookworm as builder

ARG SERVICE_NAME
ENV SERVICE_NAME=${SERVICE_NAME}

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src/go/ ./src/go/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server src/go/services/${SERVICE_NAME}/main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server

# Expose generic port per Cloud Run rules
EXPOSE 8080

ENTRYPOINT ["/server"]
