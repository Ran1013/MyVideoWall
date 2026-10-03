# MyVideoWall · 自部署游戏高光视频站

轻量私有视频站：小圈子分享游戏精彩片段。**Go 单二进制 + 内嵌 Vue 前端**，一个容器（或一个二进制）就是一个完整的站，MySQL 存元数据，视频文件存本地磁盘。

> 为小带宽服务器（3-5Mbps）设计：上传的视频自动转码压缩 + 自动生成封面，首页零视频请求，播放秒开。

## 功能

- **视频墙**：封面卡片墙、分类筛选、最新/最热排序、搜索、分页
- **播放页**：Range 分段流式播放（拖动秒响应）、上一部/下一部、相关推荐、播放计数
- **上传**：分片上传 + 断点续传，上传完自动压缩（ffmpeg）+ 生成封面
- **管理端**（独立密码进入）：
  - 视频管理（改标题/分类、删除）
  - 分类管理（改名自动同步、合并、清空）
  - 密码管理（观看密码 / 管理密码随时改，热生效，全站踢下线）
  - 转码画质面板（码率/帧率/CRF/分辨率/单文件上限，含快捷档，热生效并持久化）
  - 本月流量统计 + 熔断（超上限自动暂停播放，防流量包爆炸）
- **安全**：登录失败限速（同 IP 1 分钟 5 次失败封锁 1 分钟）、HMAC token（改密码即全量失效）、路径穿越防护、扩展名白名单

## 架构

```
浏览器 → http://服务器IP:端口
  videowall 容器（Go + gin）
    ├─ /*      内嵌 Vue 前端（SPA fallback）
    ├─ /api/*  auth / videos / stream / upload / admin / logs + health
    └─ stream  Range 视频流（原生 206/416，出流字节计入月度流量）
  MySQL（同机 127.0.0.1:3306，视频元数据 + 访问日志）
  videos/ chunks/ data/（宿主机目录挂载：视频、分片、持久化配置）
```

## 快速开始（Docker Compose）

前置：一台服务器 + 已运行的 MySQL（同机 127.0.0.1 可连）。

```bash
git clone https://github.com/Ran1013/MyVideoWall.git
cd MyVideoWall

# 1) 准备宿主机目录
mkdir -p /opt/videowall/{videos,chunks,data}

# 2) 配置数据库与密码
export MYSQL_DSN="MyVideos:你的数据库密码@tcp(127.0.0.1:3306)/MyVideos?charset=utf8mb4&parseTime=true"
export VIEW_PASSWORD="观看密码"
export UPLOAD_PASSWORD="管理密码"

# 3) 一键部署（首次构建约 3-6 分钟）
bash deploy.sh
```

或者不用脚本：

```bash
docker compose -f server/docker-compose.prod.yml up -d --build
```

浏览器打开 `http://服务器IP:端口`（默认 8080，由 `PORT` 决定），输入观看密码进入；管理密码登录后可访问管理页。**记得在云厂商防火墙/安全组放行该端口。**

### 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | 8080 | 监听端口 |
| `MYSQL_DSN` | — | `用户:密码@tcp(127.0.0.1:3306)/库名?charset=utf8mb4&parseTime=true` |
| `VIEW_PASSWORD` | view123 | 观看密码（初始值，可在管理端改） |
| `UPLOAD_PASSWORD` | upload123 | 上传/管理密码（初始值，可在管理端改） |
| `TRAFFIC_LIMIT_GB` | 280 | 月度流量熔断上限（GB，只统计视频出流） |
| `TRANSCODE_MAXRATE` | 1200k | 转码码率上限（初始值，可在管理端改） |
| `TRANSCODE_FPS` | 30 | 转码帧率（初始值，可在管理端改） |
| `TRANSCODE_CRF` | 28 | 转码质量 CRF（初始值，可在管理端改） |
| `TRANSCODE_MAX_MB` | 4096 | 单文件转码上限 MB，超过跳过压缩原样播出（0=不限制） |
| `TOKEN_TTL_DAYS` | 30 | 登录 token 有效期 |
| `CHUNK_MB` | 4 | 上传分片大小 |

目录挂载（compose 已配好）：`videos/` 视频与封面、`chunks/` 上传分片临时目录、`data/` **必须挂载**——密码与转码参数等持久化配置都在这里，丢了会回退到环境变量初始值。

## 从源码构建

前端构建产物内嵌进二进制（`go:embed`）：

```bash
# ① 构建前端
cd web && npm install && npm run build && cd ..

# ② 前端产物拷给 Go embed
rm -rf server/web-dist && cp -r web/dist server/web-dist

# ③ 编译（交叉编译示例：Mac 上编 Linux amd64）
cd server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o videowall .
```

或者直接用 Docker 多阶段构建（推荐，自动完成以上步骤）：

```bash
# 服务器是 x86 时务必带 --platform linux/amd64
docker build --platform linux/amd64 -f server/Dockerfile -t videowall:v1 .
```

> 国内服务器构建慢的话，Dockerfile 已配置国内镜像加速（npmmirror / goproxy.cn）。

## 更新与备份

- **更新**：重新构建镜像 → 重建容器（`videos/chunks/data` 三个卷不动，数据无损）
- **备份**：`bash backup/videowall-backup.sh`（先改脚本里的服务器别名与路径）——MySQL 全量导出 + 视频/配置增量同步到本机，建议每周一次
- **迁移**：拷 `/opt/videowall/{videos,chunks,data}` + `mysqldump` 导出，新机按快速开始重来

## 安全须知

- **MySQL 只走 127.0.0.1**，切勿把 3306 暴露公网（全网爆破靶子）
- **建议改掉默认密码**：管理页「密码管理」可直接改，改完全站登录失效
- 本项目默认无 HTTPS（IP 直连场景），密码明文传输；介意就套一层反向代理（Caddy/Nginx + 域名证书）
- 上传接口与管理接口都有独立密码门槛；登录接口带失败限速

## 致谢

- [ip2region](https://github.com/lionsoul2014/ip2region) —— 离线 IP 归属地库（`server/data/ip2region.xdb`，Apache-2.0）
- [gin](https://github.com/gin-gonic/gin) / [Vue 3](https://vuejs.org/) / [Vite](https://vitejs.dev/) / [ffmpeg](https://ffmpeg.org/)
