# Build the manager binary
FROM alpine:3.7  as gaia
WORKDIR /
COPY ./cmd/bin/gaia .
ENTRYPOINT ["./gaia"]

# Build the scheduler binary
FROM alpine:3.7 as gaia-scheduler
WORKDIR /
COPY ./cmd/bin/gaia-scheduler .
ENTRYPOINT ["./gaia-scheduler"]

