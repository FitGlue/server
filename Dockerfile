FROM golang:1.25-bookworm as builder

ARG SERVICE_NAME
ENV SERVICE_NAME=${SERVICE_NAME}

WORKDIR /app

# Copy root module files first for caching
COPY src/go/go.mod src/go/go.sum ./src/go/

# Copy all source (includes service-level go.mod if present)
COPY src/go/ ./src/go/

# Download deps and build from root module
RUN cd src/go && go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server ./services/${SERVICE_NAME}

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server

# Expose generic port per Cloud Run rules
EXPOSE 8080

ENTRYPOINT ["/server"]
