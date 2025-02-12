# Turn

## Auth
As turn server needs add authHandler, default use mem to cache users, you can use redis when in production.

## Docker Install
```bash
docker run -d --net=host registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest linkany turn --public-ip 81.68.109.143
```

## Using redis
```bash
docker run -d --net=host registry.cn-hangzhou.aliyuncs.com/linkany-io/linkany:latest linkany turn --redis-host xx.x.xx.xx --redis-port 6379 --redis-password xxx
```