# Blueprint: Docker-based Bootstrapper

## 目标
实现真正的“零依赖”初始化，用户只需拥有 Docker 即可拉起完整 K8s 实验环境。

## 逻辑约束
1. **环境检测**: 脚本启动时必须先执行 `docker info` 确保映射的 socket 可用。
2. **幂等部署**: 如果检测到名为 `wireflow` 的 k3d 集群已存在，应询问用户是“重置”还是“增量更新”。
3. **架构适配**: 镜像需支持多架构（buildx），确保 (ARM64) 和普通开发机 (AMD64) 均可运行。
4. **清理机制**: 提供 `CLEANUP=true` 环境变量，运行同一镜像即可一键销毁整个集群。
5. 你可以创建一个专门用于初始化的镜像 wireflow/installer：
6. 使用下边的命令就可以完成control plane的初始化: docker run --rm \
   -v /var/run/docker.sock:/var/run/docker.sock \
   -v ~/.kube:/root/.kube \
   wireflow/installer:latest