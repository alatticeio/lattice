#!/bin/bash
set -e

# 如果用户没传参数，则通过 API 获取最新版
if [ -z "$1" ]; then
  echo "未指定版本，正在获取最新版本..."
  TAG=$(curl -s https://api.github.com/repos/wireflowio/wireflow/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
else
  TAG=$1
fi

# 去掉版本号开头的 'v' 得到文件名用的版本号
VERSION=${TAG#v}

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

FILE_NAME="wireflow_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/wireflowio/wireflow/releases/download/${TAG}/${FILE_NAME}"

echo "正在从 $URL 下载版本 $TAG..."

curl -fSL "$URL" | tar -xz
sudo mv wireflow wfctl /usr/local/bin/
chmod +x /usr/local/bin/wireflow /usr/local/bin/wfctl

echo "wireflow 安装成功"