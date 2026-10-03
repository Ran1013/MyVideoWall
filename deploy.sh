#!/bin/bash
# ============================================================
# 视频站 videowall-next · 一键部署脚本（服务器上执行）
# 用法：bash deploy.sh
# 前置：MySQL 已在宿主机运行（宝塔），库名/密码见下方配置区
# ============================================================
set -e

# ---------- 配置区 ----------
MYSQL_DSN="MyVideos:你的数据库密码@tcp(127.0.0.1:3306)/MyVideos?charset=utf8mb4&parseTime=true"
VIEW_PASSWORD="view123"
UPLOAD_PASSWORD="upload123"
ALLOW_ORIGIN=""            # 同源部署留空即可；以后加 CF Pages 前端再填其域名
VITE_API_BASE=""           # 同源部署留空（前端 embed 在容器里，直接用服务器 IP:端口 访问）
# ------------------------

cd "$(dirname "$0")"

echo "[1/4] 检查 Docker..."
if ! command -v docker >/dev/null 2>&1; then
  echo "  安装 Docker..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
else
  echo "  Docker 已安装 ✓"
fi

# 国内拉镜像加速（国内服务器建议）
ACCEL="/etc/docker/daemon.json"
if [ ! -f "$ACCEL" ]; then
  echo "  配置镜像加速器..."
  mkdir -p /etc/docker
  cat > "$ACCEL" <<'JSON'
{ "registry-mirrors": ["https://docker.m.daocloud.io", "https://dockerproxy.net"] }
JSON
  systemctl daemon-reload 2>/dev/null || true
  systemctl restart docker 2>/dev/null || true
fi

echo "[2/4] 生成 .env..."
cat > .env <<EOF
MYSQL_DSN=$MYSQL_DSN
VIEW_PASSWORD=$VIEW_PASSWORD
UPLOAD_PASSWORD=$UPLOAD_PASSWORD
ALLOW_ORIGIN=$ALLOW_ORIGIN
VITE_API_BASE=$VITE_API_BASE
EOF

echo "[3/4] 构建并启动（首次构建约 3-6 分钟，需下载 node/go 基础镜像）..."
docker compose -f server/docker-compose.prod.yml --env-file .env up -d --build

echo "[4/4] 健康检查..."
for i in $(seq 1 15); do
  sleep 2
  if curl -fs "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
    echo ""
    echo "========================================"
    echo "  部署完成 ✓"
    echo "  访问地址: http://服务器公网IP:8080"
    echo "  别忘了: 云厂商安全组放行 8080 端口"
    echo "========================================"
    exit 0
  fi
done
echo "健康检查未通过，查看日志：docker compose -f server/docker-compose.prod.yml logs --tail 50"
exit 1
