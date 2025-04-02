VERSION ?= latest

# 声明伪目标
.PHONY: build client image generate clean


build: clean linkany management signaling turn

linkany:
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/linkany \
		-w /root/linkany \
		registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 \
		go build -v -o /root/linkany/bin/linkany/linkany \
		-v /root/linkany/cmd/linkany/main.go

management:
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/linkany \
		-w /root/linkany \
		registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 \
		go build -v -o /root/linkany/bin/management/linkany \
		-v /root/linkany/cmd/management/main.go

signaling:
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/linkany \
		-w /root/linkany \
		registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 \
		go build -v -o /root/linkany/bin/signaling/linkany \
		-v /root/linkany/cmd/signaling/main.go

turn: 
	docker run --rm \
		--env CGO_ENABLED=0 \
		--env GOPROXY=https://goproxy.cn \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		-v $(shell pwd):/root/linkany \
		-w /root/linkany \
		registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 \
		go build -v -o /root/linkany/bin/turn/linkany \
		-v /root/linkany/cmd/turn/main.go

linkany-image:
	docker build -t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest \
		-f $(shell pwd)/docker/Dockerfile $(shell pwd)/bin/linkany
	docker push registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest

management-image:
	docker build -t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-mgt:latest \
		-f $(shell pwd)/docker/Dockerfile $(shell pwd)/bin/management
	docker push registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-mgt:latest

signaling-image:
	docker build -t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-sg:latest \
		-f $(shell pwd)/docker/Dockerfile $(shell pwd)/bin/signaling
	docker push registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-sg:latest

turn-image:
	docker build -t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-turn:latest \
		-f $(shell pwd)/docker/Dockerfile $(shell pwd)/bin/turn
	docker push registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany-turn:latest

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
		signaling/grpc/signaling/signaling.proto

clean:
	rm -rf bin
