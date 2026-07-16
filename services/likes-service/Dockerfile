FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/broker/ ./pkg/broker/
COPY pkg/database/ ./pkg/database/
COPY pkg/grpc/likes/ ./pkg/grpc/likes/
COPY pkg/id/ ./pkg/id/
COPY services/likes-service/ ./services/likes-service/
WORKDIR /app/services/likes-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
