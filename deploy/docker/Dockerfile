FROM registry.cn-hangzhou.aliyuncs.com/wireflow-io/alpine:3.15
LABEL authors="winston"

# base deps, timezone, and dynamic linker compat in a single layer
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk update \
    && apk add --no-cache iptables tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && mkdir -p /lib64 \
    && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

WORKDIR /app

# copy binary and frontend assets from builder and source
COPY bin/wireflow /app/wireflow
COPY templates/ /app/templates/
COPY static/ /app/static/

CMD ["/app/wireflow","login"]
