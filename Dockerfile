FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o glance .

FROM alpine:3.24.1

WORKDIR /app
COPY --from=builder /app/glance .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/glance", "--config", "/app/config/glance.yml"]
