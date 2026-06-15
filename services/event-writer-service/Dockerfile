FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY pkg/ ./pkg/
COPY services/event-writer-service/ ./services/event-writer-service/
WORKDIR /app/services/event-writer-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
