# Build the manager binary
FROM alpine:3.7  as gaia
WORKDIR /
COPY ./cmd/bin/gaia .
RUN apk --update add tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata && \
    rm -rf /var/cache/apk/*
ENTRYPOINT ["./gaia"]

# Build the scheduler binary
FROM alpine:3.7 as gaia-scheduler
WORKDIR /
COPY ./cmd/bin/gaia-scheduler .
ENTRYPOINT ["./gaia-scheduler"]

