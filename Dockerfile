FROM golang:1.26 AS builder

WORKDIR /app

COPY . .

ARG GIT_TAG=dev

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/shanehull/markethunt/internal/buildinfo.GitTag=${GIT_TAG}" \
    -o /app/markethunt ./cmd/markethunt

FROM scratch

COPY --from=builder /app/markethunt /usr/local/bin/markethunt
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER 65534:65534

ENV MARKETHUNT_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/markethunt"]
