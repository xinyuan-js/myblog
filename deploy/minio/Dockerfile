# syntax=docker/dockerfile:1.7
FROM golang:1.26.5-alpine AS build

# Pin immutable upstream commits. GOTOOLCHAIN=local prevents an older toolchain
# directive in either upstream module from downgrading the patched Go compiler.
ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
ARG MC_COMMIT=77f82e18b5401a65958f1619df6ebb994634bd88
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install -tags kqueue "github.com/minio/minio@${MINIO_COMMIT}" \
    && go install "github.com/minio/mc@${MC_COMMIT}"

FROM alpine:3.24.1

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata

COPY --from=build /go/bin/minio /usr/local/bin/minio
COPY --from=build /go/bin/mc /usr/local/bin/mc

ENV MC_CONFIG_DIR=/tmp/mc
EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["minio"]
