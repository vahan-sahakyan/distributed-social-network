FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/cache/ ./pkg/cache/
COPY pkg/grpc/feed/ ./pkg/grpc/feed/
COPY pkg/grpc/posts/ ./pkg/grpc/posts/
COPY pkg/grpc/users/ ./pkg/grpc/users/
COPY services/feed-service/ ./services/feed-service/
WORKDIR /app/services/feed-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
