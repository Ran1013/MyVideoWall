# 多架构构建说明：
#   构建命令必须带 --platform linux/amd64（服务器是 x86）
#   BUILDPLATFORM：前端构建与 Go 编译在本机原生机跑（快）；TARGETARCH 交叉编译出服务器架构的二进制
# ---------- 阶段 1：构建 Vue 前端 ----------
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com && (npm ci || npm install)
COPY web/ ./
ARG VITE_API_BASE=""
ENV VITE_API_BASE=${VITE_API_BASE}
RUN npm run build

# ---------- 阶段 2：编译 Go（前端 dist 嵌入二进制） ----------
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS backend-builder
ENV GOPROXY=https://goproxy.cn,direct
ARG TARGETARCH=amd64
RUN apk add --no-cache git
WORKDIR /src
COPY server/go.mod server/go.sum* ./
RUN go mod download
COPY server/ ./
RUN rm -rf web-dist && mkdir web-dist
COPY --from=frontend-builder /web/dist ./web-dist/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/server .

# ---------- 阶段 3：运行镜像（自包含：程序 + 前端 + IP 归属地库） ----------
FROM alpine:3.20
# ffmpeg：上传后自动压缩视频（720p/H.264/faststart）
RUN apk add --no-cache ca-certificates tzdata ffmpeg
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=backend-builder /out/server ./server
COPY server/data/ip2region.xdb ./data/ip2region.xdb
ENV PORT=8080 \
    VIDEO_DIR=/app/videos \
    CHUNKS_DIR=/app/chunks \
    IP2REGION_XDB=/app/data/ip2region.xdb
EXPOSE 8080
CMD ["./server"]
