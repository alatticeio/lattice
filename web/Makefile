TAG ?= dev

build:
	docker build -t ghcr.io/wireflowio/wireflow-web:v0.0.1 .

docker: build
	docker push ghcr.io/wireflowio/wireflow-web:v0.0.1