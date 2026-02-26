FROM golang:1.25-bookworm as builder

ARG SERVICE_NAME
ENV SERVICE_NAME=${SERVICE_NAME}

WORKDIR /app

COPY src/go/go.mod src/go/go.sum ./src/go/
RUN cd src/go && go mod download

COPY src/go/ ./src/go/

RUN cd src/go && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server ./services/${SERVICE_NAME}/main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server

# Expose generic port per Cloud Run rules
EXPOSE 8080

ENTRYPOINT ["/server"]
