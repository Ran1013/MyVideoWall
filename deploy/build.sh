#!/bin/bash
# ============================================================
# 视频站 · 本机构建脚本（在你自己的电脑上跑）
# 用法：bash deploy/build.sh
# 产出：videowall-deploy.tar.gz（上传到服务器解压后 sudo bash install.sh）
# 依赖：本机有 Docker（用于编译 Go）+ Node（用于构建前端）
# ============================================================
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist-deploy"
PKG="$OUT/videowall-pkg"

echo "== [1/5] 构建前端（Vue）=="
cd "$ROOT/web"
if [ ! -d node_modules ]; then npm install; fi
npm run build
rm -rf "$ROOT/server/web-dist" && mkdir -p "$ROOT/server/web-dist"
cp -r "$ROOT/web/dist/." "$ROOT/server/web-dist/"

echo "== [2/5] 编译 Go（Docker，嵌入前端）=="
docker run --rm -v "$ROOT/server:/src" -w /src \
  -e GOPROXY=https://goproxy.cn,direct \
  golang:1.22-alpine sh -c '
    apk add --no-cache git >/dev/null 2>&1
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out-binary .
    mv /out-binary /src/videowall-linux-amd64
  '

echo "== [3/5] 组装部署包 =="
rm -rf "$PKG" && mkdir -p "$PKG"
cp "$ROOT/server/videowall-linux-amd64" "$PKG/videowall"
cp "$ROOT/deploy/install.sh" "$ROOT/deploy/env.example" "$ROOT/deploy/videowall.service" "$PKG/"
mkdir -p "$ROOT/api/data"
if [ -f "$ROOT/api/data/ip2region.xdb" ]; then
  cp "$ROOT/api/data/ip2region.xdb" "$PKG/ip2region.xdb"
else
  echo "  ⚠️  未找到 api/data/ip2region.xdb，访客归属地将不可用"
fi

echo "== [4/5] 打包 =="
tar -czf "$OUT/videowall-deploy.tar.gz" -C "$OUT" videowall-pkg

echo "== [5/5] 完成 =="
ls -lh "$OUT/videowall-deploy.tar.gz"
echo ""
echo "下一步："
echo "  1. 上传到服务器：scp $OUT/videowall-deploy.tar.gz root@服务器IP:/root/"
echo "  2. 服务器上执行："
echo "       tar xzf videowall-deploy.tar.gz && cd videowall-pkg"
echo "       sudo bash install.sh     # 自动安装到 /opt/videowall 并注册 systemd 服务"
echo "  3. 云厂商安全组放行 8080 端口，浏览器开 http://服务器IP:8080"
echo ""
echo "（密码/数据库信息在 videowall-pkg/env.example 里，如需修改请改该文件后再上传）"
rm -f "$ROOT/server/videowall-linux-amd64"
