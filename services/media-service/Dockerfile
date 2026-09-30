FROM golang:1.27-alpine3.24 AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/grpc/media/ ./pkg/grpc/media/
COPY pkg/id/ ./pkg/id/
COPY pkg/observability/ ./pkg/observability/
COPY services/media-service/ ./services/media-service/
WORKDIR /app/services/media-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.24
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
