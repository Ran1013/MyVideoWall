#!/bin/bash
# 视频站备份：服务器 -> 本机
#
# 用法:  bash videowall-backup.sh          （建议每周跑一次，可配合 cron/计划任务）
# 前置:  ssh <服务器别名> 可直连（~/.ssh/config 已配置）；服务器已装 rsync
# 内容:  ① MySQL 全量导出（.gz，带时间戳，保留 90 天）
#        ② videos/（视频+封面）与 data/（密码配置、流量计数）rsync 增量同步
# 说明:  备份走服务器出口带宽，会计入云厂商流量包；
#        视频为增量同步（服务器上删掉的视频在本机备份里保留，防误删）。
set -euo pipefail

HOST="your-server"                              # ~/.ssh/config 里配置的服务器别名，改成你自己的
REMOTE_DIR="/opt/videowall"                     # 服务器上的部署目录
DEST="$HOME/videowall-backups"                  # 本机备份存放位置
STAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "$DEST/db" "$DEST/videos" "$DEST/data"

echo "==> ① 导出 MySQL（容器内业务账号，密码不落盘）"
# 注意 \$ 转义：变量必须进到 mysql 容器内部才展开，服务器宿主 shell 不展开
ssh "$HOST" 'docker exec mysql sh -c "exec mysqldump --no-tablespaces -u\$MYSQL_USER -p\"\$MYSQL_PASSWORD\" \$MYSQL_DATABASE" | gzip' \
  > "$DEST/db/MyVideos-$STAMP.sql.gz"
echo "    db/MyVideos-$STAMP.sql.gz ($(du -h "$DEST/db/MyVideos-$STAMP.sql.gz" | cut -f1))"

echo "==> ② 同步视频与配置（增量）"
rsync -a "$HOST:$REMOTE_DIR/videos/" "$DEST/videos/"
rsync -a "$HOST:$REMOTE_DIR/data/"   "$DEST/data/"

echo "==> ③ 清理 90 天前的数据库导出"
find "$DEST/db" -name "MyVideos-*.sql.gz" -mtime +90 -print -delete

echo "==> 完成，备份位于 $DEST"
du -sh "$DEST/db" "$DEST/videos" "$DEST/data" 2>/dev/null
