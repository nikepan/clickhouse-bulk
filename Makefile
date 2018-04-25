install:
	go get
	go install

build:
	go get
	go build

docker_build:
	docker build -t clickhouse-bulk .
