# Build the manager binary
FROM alpine:3.7
WORKDIR /
COPY ./cmd/bin/gaia .
ENTRYPOINT ["./gaia"]

