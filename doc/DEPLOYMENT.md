# go-fastdfs 部署文档

本文档介绍如何将 go-fastdfs 打包为 Linux 二进制文件，并使用 Supervisor 进行进程管理。

---

## ⚠️ 重要说明：零依赖部署

**go-fastdfs 采用完全静态编译，部署时无需安装任何外部依赖！**

| 依赖项 | 是否需要安装 | 说明 |
|--------|--------------|------|
| LevelDB | ❌ **不需要** | 使用纯 Go 实现的 goleveldb，已内置 |
| Redis | ❌ **不需要** | 不依赖 Redis |
| MySQL/PostgreSQL | ❌ **不需要** | 不依赖关系型数据库 |
| 系统库 (C库) | ❌ **不需要** | CGO_ENABLED=0 静态编译 |
| Supervisor | ✅ **推荐** | 用于进程管理（可选，也可用 systemd） |

**部署只需要两个文件：**

```
/opt/go-fastdfs/
├── fileserver          # 二进制文件（静态编译，无依赖）
└── conf/
    └── cfg.json        # 配置文件
```

**一键部署：** 上传二进制 + 配置文件 → 启动即可运行！

---

## 目录

- [环境要求](#环境要求)
- [打包 Linux 二进制](#打包-linux-二进制)
- [配置文件说明](#配置文件说明)
- [Supervisor 配置](#supervisor-配置)
- [部署流程](#部署流程)
- [常用运维命令](#常用运维命令)
- [故障排查](#故障排查)

---

## 环境要求

### 开发环境

- Go 1.19+
- Git

### 生产环境

- Linux (CentOS 7+/Ubuntu 18.04+)
- Supervisor
- 足够的磁盘空间（用于文件存储）

---

## 打包 Linux 二进制

### 方式一：本地交叉编译

在 macOS 或 Windows 上编译 Linux 二进制：

```bash
# 进入项目目录
cd go-fastdfs

# 编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o fileserver_linux_amd64

# 编译 Linux arm64 (ARM 服务器)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o fileserver_linux_arm64
```

### 方式二：使用打包脚本

创建打包脚本 `build.sh`：

```bash
#!/bin/bash

# 构建信息
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
GO_VERSION=$(go version | awk '{print $3}')

# ldflags 参数
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GoVersion=${GO_VERSION}"

# 创建输出目录
mkdir -p dist

# 编译各平台版本
echo "Building for Linux amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/fileserver_linux_amd64

echo "Building for Linux arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/fileserver_linux_arm64

# 复制配置文件模板
cp -r server/conf dist/conf_template

# 创建部署包
cd dist
tar -czvf go-fastdfs-${VERSION}-linux-amd64.tar.gz fileserver_linux_amd64 conf_template
tar -czvf go-fastdfs-${VERSION}-linux-arm64.tar.gz fileserver_linux_arm64 conf_template

echo "Build complete! Output files:"
ls -la *.tar.gz
```

执行打包：

```bash
chmod +x build.sh
./build.sh
```

### 方式三：Docker 构建

创建 `Dockerfile.build`：

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o fileserver

FROM alpine:latest

WORKDIR /app
COPY --from=builder /build/fileserver .
COPY server/conf ./conf

EXPOSE 8080
CMD ["./fileserver"]
```

构建镜像：

```bash
docker build -f Dockerfile.build -t go-fastdfs:latest .
```

---

## 配置文件说明

### 配置文件位置

默认配置文件路径：`conf/cfg.json`

也可以通过命令行参数指定：

```bash
./fileserver -config /path/to/cfg.json
```

### 完整配置示例

```json
{
  "addr": ":8080",
  "host": "http://192.168.1.100:8080",
  "group": "group1",

  "peer_id": "1",
  "peers": ["http://192.168.1.101:8080"],

  "enable_https": false,

  "support_group_manage": true,
  "enable_web_upload": false,
  "show_dir": false,

  "enable_cross_origin": true,
  "enable_custom_path": true,

  "rename_file": false,
  "enable_trim_file_name_special_char": true,

  "extensions": [],
  "image_max_width": 2000,
  "image_max_height": 1000,

  "download_domain": "",
  "default_download": true,

  "admin_key": "your-admin-key-here",
  "admin_ips": ["127.0.0.1", "192.168.1.0/24"],

  "auth_url": "http://your-bff-server/api/auth/verify",
  "enable_download_auth": true,
  "download_token_expire": 300,

  "enable_hmac_auth": true,
  "download_secret_key": "your-secret-key-at-least-32-characters-long",

  "download_use_token": false,
  "enable_google_auth": false,

  "enable_tus": true,
  "enable_merge_small_file": false,

  "auto_repair": true,
  "enable_migrate": false,
  "enable_distinct_file": true,
  "read_only": false,

  "read_timeout": 600,
  "write_timeout": 600,
  "connect_timeout": true,

  "sync_worker": 200,
  "upload_worker": 16,
  "sync_delay": 60,
  "retry_count": 3,

  "alarm_receivers": [],
  "alarm_url": "",

  "enable_pprof_debug": false,
  "proxy_insecure_skip_verify": false
}
```

### 核心配置说明

| 配置项 | 说明 | 生产环境建议 |
|--------|------|--------------|
| `addr` | 监听地址 | `:8080` 或 `0.0.0.0:8080` |
| `host` | 本机外网地址 | 实际服务器地址 |
| `group` | 组名 | `group1`, `group2` 等 |
| `peers` | 集群节点列表 | 内网地址 |
| `admin_key` | 管理密钥 | 随机生成，至少32位 |
| `admin_ips` | 管理IP白名单 | 限制管理访问 |
| `download_secret_key` | HMAC签名密钥 | 随机生成，至少32位 |

### 生成随机密钥

```bash
# 生成 32 位随机密钥
openssl rand -hex 16

# 或使用 uuid
uuidgen | tr -d '-'
```

### 环境变量覆盖（可选）

如果需要通过环境变量覆盖配置，可以修改代码添加支持，或在启动脚本中处理：

```bash
#!/bin/bash

# 从环境变量获取配置
export ADMIN_KEY="${ADMIN_KEY:-default-admin-key}"
export DOWNLOAD_SECRET="${DOWNLOAD_SECRET:-default-secret}"

# 使用 jq 修改配置文件（需要安装 jq）
jq --arg key "$ADMIN_KEY" '.admin_key = $key' conf/cfg.json > conf/cfg.json.tmp
mv conf/cfg.json.tmp conf/cfg.json

# 启动服务
./fileserver
```

---

## Supervisor 配置

### 安装 Supervisor

**CentOS/RHEL:**

```bash
yum install -y supervisor
systemctl enable supervisord
systemctl start supervisord
```

**Ubuntu/Debian:**

```bash
apt-get install -y supervisor
systemctl enable supervisor
systemctl start supervisor
```

### 配置文件

创建 `/etc/supervisor/conf.d/go-fastdfs.conf`：

```ini
[program:go-fastdfs]
# 程序名称
command=/opt/go-fastdfs/fileserver
process_name=%(program_name)s
numprocs=1

# 目录设置
directory=/opt/go-fastdfs

# 用户设置（建议使用专用用户）
user=fastdfs

# 自动启动和重启
autostart=true
autorestart=true
startsecs=3
startretries=3

# 输出日志
stdout_logfile=/var/log/go-fastdfs/stdout.log
stdout_logfile_maxbytes=50MB
stdout_logfile_backups=10
stdout_capture_maxbytes=1MB

# 错误日志
stderr_logfile=/var/log/go-fastdfs/stderr.log
stderr_logfile_maxbytes=50MB
stderr_logfile_backups=10

# 环境变量
environment=PATH="/usr/local/bin:%(ENV_PATH)s"

# 优雅停止
stopasgroup=true
killasgroup=true
stopsignal=TERM
stopwaitsecs=10

# 优先级
priority=999
```

### 创建专用用户和目录

```bash
# 创建用户（不允许登录）
useradd -r -s /sbin/nologin -d /opt/go-fastdfs fastdfs

# 创建目录结构
mkdir -p /opt/go-fastdfs/{files,data,conf,log,static}

# 设置权限
chown -R fastdfs:fastdfs /opt/go-fastdfs

# 创建日志目录
mkdir -p /var/log/go-fastdfs
chown -R fastdfs:fastdfs /var/log/go-fastdfs
```

### 目录结构

```
/opt/go-fastdfs/
├── fileserver          # 二进制文件
├── conf/
│   └── cfg.json        # 配置文件
├── files/              # 文件存储目录
├── data/               # LevelDB 数据
├── log/                # 应用日志
└── static/             # 静态文件
```

---

## 部署流程

### 1. 准备服务器

```bash
# 更新系统
yum update -y  # CentOS
# apt update && apt upgrade -y  # Ubuntu

# 安装依赖
yum install -y supervisor  # CentOS
# apt install -y supervisor  # Ubuntu

# 创建用户和目录
useradd -r -s /sbin/nologin -d /opt/go-fastdfs fastdfs
mkdir -p /opt/go-fastdfs/{files,data,conf,log,static}
mkdir -p /var/log/go-fastdfs
chown -R fastdfs:fastdfs /opt/go-fastdfs
chown -R fastdfs:fastdfs /var/log/go-fastdfs
```

### 2. 上传文件

```bash
# 在本地执行
scp dist/go-fastdfs-*.tar.gz user@server:/tmp/

# 在服务器执行
cd /opt/go-fastdfs
tar -xzf /tmp/go-fastdfs-*.tar.gz
mv fileserver_linux_amd64 fileserver
chmod +x fileserver
```

### 3. 配置服务

```bash
# 复制配置文件
cp -r conf_template/* conf/

# 编辑配置文件
vim conf/cfg.json

# 生成随机密钥
echo "admin_key: $(openssl rand -hex 16)"
echo "download_secret_key: $(openssl rand -hex 16)"
```

### 4. 配置 Supervisor

```bash
# 创建配置文件
cat > /etc/supervisor/conf.d/go-fastdfs.conf << 'EOF'
[program:go-fastdfs]
command=/opt/go-fastdfs/fileserver
directory=/opt/go-fastdfs
user=fastdfs
autostart=true
autorestart=true
startsecs=3
stdout_logfile=/var/log/go-fastdfs/stdout.log
stdout_logfile_maxbytes=50MB
stderr_logfile=/var/log/go-fastdfs/stderr.log
stderr_logfile_maxbytes=50MB
stopasgroup=true
killasgroup=true
EOF

# 重新加载配置
supervisorctl reread
supervisorctl update
```

### 5. 验证部署

```bash
# 检查服务状态
supervisorctl status go-fastdfs

# 检查端口
netstat -tlnp | grep 8080

# 测试服务
curl http://localhost:8080/status
```

---

## 常用运维命令

### Supervisor 命令

```bash
# 查看所有服务状态
supervisorctl status

# 启动服务
supervisorctl start go-fastdfs

# 停止服务
supervisorctl stop go-fastdfs

# 重启服务
supervisorctl restart go-fastdfs

# 重新加载配置（不重启）
supervisorctl reread
supervisorctl update

# 查看日志
supervisorctl tail -f go-fastdfs
supervisorctl tail -f go-fastdfs stderr
```

### 服务管理命令

```bash
# 查看进程
ps aux | grep fileserver

# 查看端口
netstat -tlnp | grep 8080
ss -tlnp | grep 8080

# 查看资源使用
top -p $(pgrep fileserver)

# 查看磁盘使用
df -h /opt/go-fastdfs/files
du -sh /opt/go-fastdfs/files/*
```

### go-fastdfs API 命令

```bash
# 查看状态
curl http://localhost:8080/group1/status

# 重新加载配置
curl "http://localhost:8080/group1/reload?admin_key=your-admin-key"

# 查看统计
curl http://localhost:8080/group1/stat

# 检查文件
curl "http://localhost:8080/group1/check_file_exist?md5=file_md5"

# 获取签名 URL
curl "http://localhost:8080/group1/sign_download?path=/group1/default/file.pdf"
```

---

## 故障排查

### 服务无法启动

**检查日志：**

```bash
supervisorctl tail go-fastdfs
cat /var/log/go-fastdfs/stderr.log
```

**常见问题：**

1. **端口被占用**
   ```bash
   netstat -tlnp | grep 8080
   # 修改配置文件中的 addr
   ```

2. **权限问题**
   ```bash
   chown -R fastdfs:fastdfs /opt/go-fastdfs
   chmod +x /opt/go-fastdfs/fileserver
   ```

3. **配置文件错误**
   ```bash
   # 验证 JSON 格式
   python -m json.tool conf/cfg.json
   # 或
   jq . conf/cfg.json
   ```

### 服务运行但无法访问

**检查防火墙：**

```bash
# CentOS
firewall-cmd --list-ports
firewall-cmd --add-port=8080/tcp --permanent
firewall-cmd --reload

# Ubuntu
ufw status
ufw allow 8080/tcp
```

**检查监听地址：**

```bash
# 确保监听 0.0.0.0 而不是 127.0.0.1
netstat -tlnp | grep 8080
```

### 性能问题

**调整系统参数：**

```bash
# 增加文件描述符限制
cat >> /etc/security/limits.conf << EOF
fastdfs soft nofile 65535
fastdfs hard nofile 65535
EOF

# 调整内核参数
cat >> /etc/sysctl.conf << EOF
fs.file-max = 1000000
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
EOF

sysctl -p
```

**调整应用参数：**

修改 `conf/cfg.json`：

```json
{
  "sync_worker": 400,
  "upload_worker": 32,
  "queue_size": 20000
}
```

### 集群同步问题

**检查集群状态：**

```bash
curl http://localhost:8080/group1/status
```

**手动触发同步：**

```bash
curl "http://localhost:8080/group1/sync?date=2026-03-01&admin_key=your-key"
```

---

## 版本升级

### 平滑升级步骤

```bash
# 1. 备份
cp /opt/go-fastdfs/fileserver /opt/go-fastdfs/fileserver.bak
cp /opt/go-fastdfs/conf/cfg.json /opt/go-fastdfs/conf/cfg.json.bak

# 2. 上传新版本
scp dist/fileserver_linux_amd64 user@server:/opt/go-fastdfs/fileserver.new
chmod +x /opt/go-fastdfs/fileserver.new

# 3. 停止服务
supervisorctl stop go-fastdfs

# 4. 替换二进制
mv /opt/go-fastdfs/fileserver.new /opt/go-fastdfs/fileserver

# 5. 启动服务
supervisorctl start go-fastdfs

# 6. 验证
curl http://localhost:8080/group1/status
```

### 回滚

```bash
# 如果升级失败，快速回滚
supervisorctl stop go-fastdfs
mv /opt/go-fastdfs/fileserver.bak /opt/go-fastdfs/fileserver
supervisorctl start go-fastdfs
```

---

## 监控告警

### 健康检查脚本

```bash
#!/bin/bash
# /opt/go-fastdfs/health_check.sh

ALARM_URL="http://your-alarm-server/notify"
SERVICE_NAME="go-fastdfs"

check_service() {
    if ! supervisorctl status go-fastdfs | grep -q "RUNNING"; then
        curl -s -X POST "$ALARM_URL" -d "service=$SERVICE_NAME&message=Service not running"
        return 1
    fi

    if ! curl -s -f "http://localhost:8080/group1/status" > /dev/null; then
        curl -s -X POST "$ALARM_URL" -d "service=$SERVICE_NAME&message=Health check failed"
        return 1
    fi

    return 0
}

check_service
```

### Cron 定时检查

```bash
# 添加到 crontab
crontab -e

# 每 5 分钟检查一次
*/5 * * * * /opt/go-fastdfs/health_check.sh
```

---

## 附录

### Systemd 配置（替代方案）

如果不使用 Supervisor，可以使用 Systemd：

创建 `/etc/systemd/system/go-fastdfs.service`：

```ini
[Unit]
Description=Go FastDFS File Server
After=network.target

[Service]
Type=simple
User=fastdfs
Group=fastdfs
WorkingDirectory=/opt/go-fastdfs
ExecStart=/opt/go-fastdfs/fileserver
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
systemctl daemon-reload
systemctl enable go-fastdfs
systemctl start go-fastdfs
systemctl status go-fastdfs
```

### Nginx 反向代理配置

```nginx
upstream fastdfs {
    server 127.0.0.1:8080;
}

server {
    listen 80;
    server_name fastdfs.example.com;

    client_max_body_size 1000M;
    proxy_connect_timeout 300s;
    proxy_send_timeout 300s;
    proxy_read_timeout 300s;

    location / {
        proxy_pass http://fastdfs;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```
