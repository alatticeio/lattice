VERSION ?= latest

# 声明伪目标
.PHONY: build


build: clean
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/wireflow \
		-w /root/wireflow \
		registry.cn-hangzhou.aliyuncs.com/wireflow-io/golang:1.25.2 \
		go build -v -o /root/wireflow/bin/wireflow \
		-v /root/wireflow/main.go

build-image:
	docker build \
		-t registry.cn-hangzhou.aliyuncs.com/wireflow-io/wireflow:latest \
		-f deploy/docker/Dockerfile . \
		--push

generate:
	protoc --proto_path=internal/proto \
		--go_out=internal/grpc \
		--go_opt=paths=source_relative \
		--go-grpc_out=internal/grpc \
		--go-grpc_opt=paths=source_relative drp.proto management.proto
clean:
	rm -rf bin
