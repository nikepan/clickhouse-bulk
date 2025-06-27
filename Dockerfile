FROM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY
ENV GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    CGO_ENABLED=0 \
    GO111MODULE=on

WORKDIR /go/src/github.com/nikepan/clickhouse-bulk

# cache dependencies
COPY go.* ./
RUN go mod download

COPY . ./
RUN go build

FROM alpine:3
RUN apk add ca-certificates
WORKDIR /app
RUN mkdir /app/dumps
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/config.sample.json .
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/clickhouse-bulk .
EXPOSE 8123
ENTRYPOINT ["./clickhouse-bulk", "-config=config.sample.json"]
