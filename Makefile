install:
	go mod download
	go install

build:
	go mod download
	go build

docker_build:
	docker build -t clickhouse-bulk .
