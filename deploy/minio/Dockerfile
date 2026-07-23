# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build

# Pin immutable upstream commits. GOTOOLCHAIN=local prevents an older toolchain
# directive in either upstream module from downgrading the patched Go compiler.
ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
ARG MINIO_SHORT_COMMIT=9e49d5e7a648
ARG MINIO_VERSION=2025-10-15T17:29:55Z
ARG MC_COMMIT=77f82e18b5401a65958f1619df6ebb994634bd88
ARG MC_SHORT_COMMIT=77f82e18b540
ARG MC_VERSION=2025-11-06T16:25:29Z
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

RUN apk add --no-cache curl tar

RUN --mount=type=cache,target=/root/.cache/upstream \
    set -eux; \
    curl --fail --location --retry 8 --retry-all-errors --proto '=https' --tlsv1.2 \
      "https://codeload.github.com/minio/minio/tar.gz/${MINIO_COMMIT}" \
      --output /root/.cache/upstream/minio.tar.gz; \
    curl --fail --location --retry 8 --retry-all-errors --proto '=https' --tlsv1.2 \
      "https://codeload.github.com/minio/mc/tar.gz/${MC_COMMIT}" \
      --output /root/.cache/upstream/mc.tar.gz; \
    mkdir -p /src/minio /src/mc; \
    tar -xzf /root/.cache/upstream/minio.tar.gz --strip-components=1 -C /src/minio; \
    tar -xzf /root/.cache/upstream/mc.tar.gz --strip-components=1 -C /src/mc

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd /src/minio \
    && go get \
      github.com/apache/thrift@v0.23.0 \
      github.com/buger/jsonparser@v1.1.2 \
      github.com/go-jose/go-jose/v4@v4.1.4 \
      github.com/prometheus/prometheus@v0.311.3 \
      go.opentelemetry.io/otel/sdk@v1.43.0 \
      golang.org/x/crypto@v0.52.0 \
      golang.org/x/net@v0.55.0 \
      google.golang.org/grpc@v1.82.1 \
    && go build -mod=mod -tags kqueue -trimpath \
      -ldflags="-s -w -X github.com/minio/minio/cmd.Version=${MINIO_VERSION} -X github.com/minio/minio/cmd.CopyrightYear=2025 -X github.com/minio/minio/cmd.ReleaseTag=RELEASE.2025-10-15T17-29-55Z -X github.com/minio/minio/cmd.CommitID=${MINIO_COMMIT} -X github.com/minio/minio/cmd.ShortCommitID=${MINIO_SHORT_COMMIT}" \
      -o /go/bin/minio . \
    && cd /src/mc \
    && go get \
      github.com/prometheus/prometheus@v0.311.3 \
      golang.org/x/crypto@v0.52.0 \
      golang.org/x/net@v0.55.0 \
      google.golang.org/grpc@v1.82.1 \
    && go build -mod=mod -trimpath \
      -ldflags="-s -w -X github.com/minio/mc/cmd.Version=${MC_VERSION} -X github.com/minio/mc/cmd.CopyrightYear=2025 -X github.com/minio/mc/cmd.ReleaseTag=RELEASE.2025-11-06T16-25-29Z -X github.com/minio/mc/cmd.CommitID=${MC_COMMIT} -X github.com/minio/mc/cmd.ShortCommitID=${MC_SHORT_COMMIT}" \
      -o /go/bin/mc .

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata

COPY --from=build /go/bin/minio /usr/local/bin/minio
COPY --from=build /go/bin/mc /usr/local/bin/mc
COPY --from=build /src/minio/LICENSE /licenses/minio/LICENSE
COPY --from=build /src/mc/LICENSE /licenses/minio-mc/LICENSE

ENV MC_CONFIG_DIR=/tmp/mc
EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["minio"]
