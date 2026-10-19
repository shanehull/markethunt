FROM golang:1.26 AS builder

WORKDIR /app

COPY . .

ARG GIT_TAG=dev

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/shanehull/markethunt/internal/buildinfo.GitTag=${GIT_TAG}" \
    -o /app/markethunt ./cmd/markethunt

RUN mkdir -p /cache && touch /cache/.keep

FROM scratch

COPY --from=builder /app/markethunt /usr/local/bin/markethunt
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chown=65534:65534 /cache /cache

USER 65534:65534

ENV MARKETHUNT_ADDR=:8080
ENV MARKETHUNT_CACHE=/cache
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/markethunt"]
