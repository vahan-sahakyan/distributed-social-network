FROM golang:1.27-alpine3.24 AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/broker/ ./pkg/broker/
COPY pkg/database/ ./pkg/database/
COPY pkg/events/ ./pkg/events/
COPY pkg/grpc/notifications/ ./pkg/grpc/notifications/
COPY pkg/grpc/posts/ ./pkg/grpc/posts/
COPY pkg/id/ ./pkg/id/
COPY pkg/observability/ ./pkg/observability/
COPY pkg/retry/ ./pkg/retry/
COPY services/notification-service/ ./services/notification-service/
WORKDIR /app/services/notification-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.24
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
