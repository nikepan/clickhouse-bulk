FROM golang:1.10.1 as builder
WORKDIR /go/src/github.com/nikepan/clickhouse-bulk
RUN go get -u github.com/golang/dep/cmd/dep
ADD . ./
RUN go get
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build
FROM alpine:latest
WORKDIR /app
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/config.sample.json .
COPY --from=builder /go/src/github.com/nikepan/clickhouse-bulk/clickhouse-bulk .
EXPOSE 8123
ENTRYPOINT ["./clickhouse-bulk", "-config=config.sample.json"]