VERSION ?= latest

build: any

any:
	docker run --rm --env CGO_ENABLED=0 --env GOPROXY=https://goproxy.cn  --env GOOS=linux --env GOARCH=amd64 -v $(shell pwd):/root/linkany -w /root/linkany registry.cn-hangzhou.aliyuncs.com/linkany-io/golang:1.23.0 go build -v -o /root/linkany/bin/linkany -v main.go

image:
	docker build -t registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest -f ${shell pwd}/docker/Dockerfile ${shell pwd}/bin
	docker push registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest

clean:
	rm -rf bin