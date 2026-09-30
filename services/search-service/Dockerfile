FROM golang:1.27-alpine3.24 AS builder
WORKDIR /app
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg/broker/ ./pkg/broker/
COPY pkg/events/ ./pkg/events/
COPY pkg/grpc/search/ ./pkg/grpc/search/
COPY pkg/observability/ ./pkg/observability/
COPY pkg/validate/ ./pkg/validate/
COPY services/search-service/ ./services/search-service/
WORKDIR /app/services/search-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.24
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
