# MinIO no longer publishes images or binaries; build the last community release from source
FROM golang:1.24-alpine AS build
ARG MINIO_RELEASE=RELEASE.2025-09-07T16-13-09Z
ARG MINIO_VERSION=2025-09-07T16:13:09Z
ARG MINIO_COMMIT=07c3a429bfed433e49018cb0f78a52145d4bedeb
ARG MINIO_SHORT_COMMIT=07c3a429bfed
RUN apk add --no-cache git
RUN CGO_ENABLED=0 go install -trimpath \
    -ldflags "-s -w \
      -X github.com/minio/minio/cmd.Version=${MINIO_VERSION} \
      -X github.com/minio/minio/cmd.ReleaseTag=${MINIO_RELEASE} \
      -X github.com/minio/minio/cmd.CommitID=${MINIO_COMMIT} \
      -X github.com/minio/minio/cmd.ShortCommitID=${MINIO_SHORT_COMMIT}" \
    github.com/minio/minio@${MINIO_COMMIT}

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/minio /usr/bin/minio
EXPOSE 9000 9001
ENTRYPOINT ["minio"]
