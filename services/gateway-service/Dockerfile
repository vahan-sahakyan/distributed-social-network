FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/grpc/cache_rebuilder/ ./pkg/grpc/cache_rebuilder/
COPY pkg/grpc/comments/ ./pkg/grpc/comments/
COPY pkg/grpc/feed/ ./pkg/grpc/feed/
COPY pkg/grpc/likes/ ./pkg/grpc/likes/
COPY pkg/grpc/media/ ./pkg/grpc/media/
COPY pkg/grpc/notifications/ ./pkg/grpc/notifications/
COPY pkg/grpc/posts/ ./pkg/grpc/posts/
COPY pkg/grpc/users/ ./pkg/grpc/users/
COPY services/gateway-service/ ./services/gateway-service/
WORKDIR /app/services/gateway-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
