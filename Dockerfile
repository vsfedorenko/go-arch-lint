FROM golang:1.25-alpine

COPY go-arch-lint /usr/local/bin/go-arch-lint

ENTRYPOINT ["go-arch-lint"]
