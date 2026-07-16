FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/database/ ./pkg/database/
COPY pkg/grpc/users/ ./pkg/grpc/users/
COPY pkg/id/ ./pkg/id/
COPY services/users-service/ ./services/users-service/
WORKDIR /app/services/users-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
