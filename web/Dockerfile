# ---- build stage ----
FROM node:20-alpine AS build
WORKDIR /app

# 先拷贝依赖清单，利用缓存
COPY package.json package-lock.json ./
RUN npm ci

# 再拷贝源码并构建
COPY . .
RUN npm run build

# ---- runtime stage ----
FROM nginx:1.27-alpine
# 覆盖默认站点配置，支持 SPA fallback
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 拷贝构建产物（Vite 默认 dist）
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]