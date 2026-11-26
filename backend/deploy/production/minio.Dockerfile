# The upstream Docker Hub and Quay release images are no longer available.
# Build the existing pinned release from its verified upstream commit instead.
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src
ARG MINIO_COMMIT=0d7408fc9969caf07de6a8c3a84f9fbb10a6739e
RUN git init . \
    && git remote add origin https://github.com/minio/minio.git \
    && git fetch --depth 1 origin ${MINIO_COMMIT} \
    && git checkout --detach FETCH_HEAD \
    && test "$(git rev-parse HEAD)" = "${MINIO_COMMIT}"
ENV MINIO_RELEASE=RELEASE CGO_ENABLED=0 GOFLAGS=-p=2
RUN go build -tags kqueue -trimpath \
    -ldflags="$(go run buildscripts/gen-ldflags.go 2025-04-22T22:12:26Z)" \
    -o /out/minio .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates curl tzdata \
    && addgroup -S minio && adduser -S -G minio minio \
    && mkdir /data && chown minio:minio /data
COPY --from=build /out/minio /usr/local/bin/minio
COPY --from=build /src/LICENSE /usr/share/licenses/minio/LICENSE
LABEL org.opencontainers.image.source="https://github.com/minio/minio" \
      org.opencontainers.image.revision="0d7408fc9969caf07de6a8c3a84f9fbb10a6739e" \
      org.opencontainers.image.version="RELEASE.2025-04-22T22-12-26Z" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
USER minio
EXPOSE 9000 9001
ENTRYPOINT ["/usr/local/bin/minio"]
