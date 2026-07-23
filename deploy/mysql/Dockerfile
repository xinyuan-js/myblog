# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS gosu-build

ARG GOSU_COMMIT=6456aaa0f3c854d199d0f037f068eb97515b7513
RUN apk add --no-cache curl tar
RUN curl --fail --location --retry 8 --retry-all-errors --proto '=https' --tlsv1.2 \
      "https://codeload.github.com/tianon/gosu/tar.gz/${GOSU_COMMIT}" \
      --output /tmp/gosu.tar.gz \
    && mkdir -p /src/gosu \
    && tar -xzf /tmp/gosu.tar.gz --strip-components=1 -C /src/gosu
WORKDIR /src/gosu
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=1.19" -o /go/bin/gosu .

# MySQL 8.4.10 LTS. The official image also bundles MySQL Shell and its Python
# environment even though the server entrypoint does not use them. Remove that
# attack surface and replace gosu with the same source revision built by the
# current patched Go toolchain.
FROM mysql:8.4@sha256:5700b0892591a760c4caef7a0024c887afd46317d73dd420801706e661c4db56

USER root
RUN microdnf remove -y mysql-shell \
    && microdnf clean all \
    && rm -rf /var/cache/dnf /var/cache/yum
COPY --from=gosu-build /go/bin/gosu /usr/local/bin/gosu
COPY --from=gosu-build /src/gosu/LICENSE /licenses/gosu/LICENSE
