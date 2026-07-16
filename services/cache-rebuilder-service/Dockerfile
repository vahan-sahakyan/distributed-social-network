FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/cache/ ./pkg/cache/
COPY pkg/database/ ./pkg/database/
COPY pkg/grpc/cache_rebuilder/ ./pkg/grpc/cache_rebuilder/
COPY pkg/grpc/posts/ ./pkg/grpc/posts/
COPY pkg/grpc/users/ ./pkg/grpc/users/
COPY services/cache-rebuilder-service/ ./services/cache-rebuilder-service/
WORKDIR /app/services/cache-rebuilder-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
