FROM registry.cn-hangzhou.aliyuncs.com/linkany-io/alpine:3.15
LABEL authors="winston"
## ADD, resolve: sh: xxx not found
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && apk update && apk add --no-cache iptables
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
RUN apk add --no-cache tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone
WORKDIR /app
COPY linkany /usr/local/bin/linkany

CMD ["linkany", "login"]
