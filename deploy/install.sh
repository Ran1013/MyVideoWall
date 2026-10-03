#!/bin/bash
# ============================================================
# 视频站 · 服务器安装脚本（把本目录上传到服务器后执行）
# 用法：sudo bash install.sh
# 做的事：装到 /opt/videowall、生成 .env、注册 systemd 服务、启动
# ============================================================
set -e

APP_DIR=/opt/videowall
HERE="$(cd "$(dirname "$0")" && pwd)"

if [ "$(id -u)" != "0" ]; then
  echo "请用 root 执行：sudo bash install.sh"
  exit 1
fi

echo "[1/5] 安装文件到 $APP_DIR ..."
mkdir -p "$APP_DIR/videos" "$APP_DIR/chunks" "$APP_DIR/data"
cp "$HERE/videowall" "$APP_DIR/videowall"
chmod +x "$APP_DIR/videowall"
[ -f "$HERE/ip2region.xdb" ] && cp "$HERE/ip2region.xdb" "$APP_DIR/data/ip2region.xdb"

echo "[2/5] 生成配置 .env ..."
if [ -f "$APP_DIR/.env" ]; then
  echo "  已存在 $APP_DIR/.env，保留原配置（如需改密码请编辑该文件后 restart）"
else
  cp "$HERE/env.example" "$APP_DIR/.env"
  echo "  ⚠️  已生成 $APP_DIR/.env —— 请先编辑它（改两个密码与数据库信息），"
  echo "     改完重新执行本脚本，或执行：systemctl restart videowall"
fi

echo "[3/5] 注册 systemd 服务 ..."
cp "$HERE/videowall.service" /etc/systemd/system/videowall.service
systemctl daemon-reload
systemctl enable videowall >/dev/null 2>&1 || true

echo "[4/5] 启动服务 ..."
systemctl restart videowall

echo "[5/5] 健康检查 ..."
for i in $(seq 1 10); do
  sleep 2
  if curl -fs "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
    PORT=$(grep -E '^PORT=' "$APP_DIR/.env" | cut -d= -f2)
    echo ""
    echo "========================================"
    echo "  部署完成 ✓"
    echo "  访问地址: http://服务器公网IP:${PORT:-8080}"
    echo "  别忘了: 云厂商安全组放行该端口"
    echo "  看日志: journalctl -u videowall -f"
    echo "========================================"
    exit 0
  fi
done

echo ""
echo "健康检查未通过，可能原因："
echo "  - .env 里的 MYSQL_DSN 数据库信息不对（先用 mysql 命令验证连接）"
echo "  - 端口被占用"
echo "看详细日志：journalctl -u videowall -n 50 --no-pager"
exit 1
