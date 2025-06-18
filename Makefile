VERSION ?= latest

# 声明伪目标
.PHONY: build


build: clean
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/linkany \
		-w /root/linkany \
		registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 \
		go build -v -o /root/linkany/bin/linkany \
		-v /root/linkany/main.go

build-image:
	cd $(shell pwd)/bin && docker build \
		-t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest \
		-f /root/docker/maven/build/linkany/docker/Dockerfile . \
		--push

generate:
	protoc --go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		management/grpc/mgt/management.proto
	protoc --go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		drp/grpc/drp.proto

clean:
	rm -rf bin
