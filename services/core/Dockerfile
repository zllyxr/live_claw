FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/claw-core .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S claw \
    && adduser -S -G claw claw
COPY --from=build /out/claw-core /usr/local/bin/claw-core

USER claw
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/claw-core"]
