FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ARG TARGET=api
COPY . .
RUN case "$TARGET" in \
      api|admin|im|scheduler|game|support|migrate|admin-bootstrap|support-agent-bootstrap) ;; \
      *) echo "unsupported build target: $TARGET" >&2; exit 1 ;; \
    esac \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
      -o /out/service "./cmd/${TARGET}"

FROM alpine:3.22
RUN addgroup -S claw && adduser -S -G claw claw \
    && apk add --no-cache ca-certificates tzdata
COPY --from=build /out/service /usr/local/bin/service
USER claw
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/service"]
