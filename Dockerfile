FROM golang:1.12.4 as builder

ARG GOPROXY
ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0 \
    GO111MODULE=on

WORKDIR /go/src/github.com/nikepan/clickhouse-bulk

# cache dependencies
ADD go.* ./
RUN go mod download

ADD . ./
RUN go build -v

FROM alpine:latest
RUN apk add ca-certificates
WORKDIR /app
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/config.sample.json .
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/clickhouse-bulk .
EXPOSE 8123
ENTRYPOINT ["./clickhouse-bulk", "-config=config.sample.json"]