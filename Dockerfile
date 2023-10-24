FROM 122.96.144.180:30080/ci/golang:alpine AS builder

WORKDIR /build

COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o _output/bin/gaia cmd/gaia-controllers/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o _output/bin/gaia-scheduler cmd/gaia-scheduler/main.go

FROM 122.96.144.180:30080/ci/ubuntu:18.04 as gaia
WORKDIR /
COPY  --from=builder /build/_output/bin/gaia .
ENTRYPOINT ["./gaia"]

# Build the scheduler binary
FROM 122.96.144.180:30080/ci/ubuntu:18.04 as gaia-scheduler
WORKDIR /
COPY  --from=builder /build/_output/bin/gaia-scheduler .
ENTRYPOINT ["./gaia-scheduler"]

