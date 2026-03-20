# OpenClaw-db9 部署指南

本文档详细说明如何在各种环境中部署 OpenClaw-db9。

## 目录

1. [开发环境部署](#开发环境部署)
2. [生产环境部署](#生产环境部署)
3. [Docker 部署](#docker-部署)
4. [手动部署](#手动部署)
5. [配置说明](#配置说明)
6. [安全建议](#安全建议)
7. [故障排除](#故障排除)

---

## 开发环境部署

### 前置要求

- Docker Desktop 20.10+
- Docker Compose 2.0+
- Git
- (可选) Go 1.23+

### 快速启动

```bash
# 克隆项目
git clone https://github.com/openclaw-db9/oc-db9.git
cd oc-db9

# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f api
```

### 验证部署

```bash
# 检查服务状态
docker-compose ps

# 测试 API
curl http://localhost:8080/health
```

### 访问服务

| 服务 | 地址 | 说明 |
|------|------|------|
| API | http://localhost:8080 | REST API |
| MinIO 控制台 | http://localhost:9001 | 文件存储管理 |
| PostgreSQL | localhost:5432 | 数据库连接 |

---

## 生产环境部署

### 系统要求

| 组件 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核+ |
| 内存 | 4 GB | 8 GB+ |
| 存储 | 50 GB SSD | 100 GB+ SSD |
| 操作系统 | Linux (Ubuntu 22.04+) | Linux |

### 1. 准备服务器

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装 Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 安装 Docker Compose
sudo apt install docker-compose-plugin -y

# 验证安装
docker --version
docker compose version
```

### 2. 配置环境变量

创建 `.env` 文件：

```env
# PostgreSQL 配置
POSTGRES_PASSWORD=your_secure_password_here

# MinIO 配置
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=your_secure_secret_key_here

# API 配置
API_PORT=8080
ENVIRONMENT=production

# Ollama 配置（可选）
OLLAMA_ENABLED=false
```

### 3. 启动服务

```bash
# 构建并启动
docker compose up -d --build

# 查看状态
docker compose ps

# 查看日志
docker compose logs -f
```

### 4. 配置反向代理 (Nginx)

```nginx
# /etc/nginx/sites-available/oc-db9
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 文件上传大小限制
        client_max_body_size 100M;
    }
}
```

```bash
# 启用站点
sudo ln -s /etc/nginx/sites-available/oc-db9 /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 5. 配置 SSL (Let's Encrypt)

```bash
# 安装 Certbot
sudo apt install certbot python3-certbot-nginx -y

# 获取证书
sudo certbot --nginx -d your-domain.com

# 自动续期
sudo systemctl enable certbot.timer
```

---

## Docker 部署

### 使用 Docker Compose（推荐）

```yaml
# docker-compose.yml
services:
  postgres:
    image: pgvector/pgvector:pg16
    container_name: oc-db9-postgres
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init-scripts:/docker-entrypoint-initdb.d
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  minio:
    image: minio/minio:latest
    container_name: oc-db9-minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-minioadmin}
    volumes:
      - minio_data:/data
    ports:
      - "9000:9000"
      - "9001:9001"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3
    restart: unless-stopped

  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    container_name: oc-db9-api
    environment:
      DATABASE_URL: postgres://postgres:${POSTGRES_PASSWORD:-postgres}@postgres:5432/postgres
      MINIO_ENDPOINT: minio:9000
      MINIO_ACCESS_KEY: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_SECRET_KEY: ${MINIO_ROOT_PASSWORD:-minioadmin}
      API_PORT: "8080"
      ENVIRONMENT: production
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped

volumes:
  postgres_data:
  minio_data:
```

### 单独运行容器

```bash
# 创建网络
docker network create oc-db9-network

# PostgreSQL
docker run -d \
  --name oc-db9-postgres \
  --network oc-db9-network \
  -e POSTGRES_PASSWORD=postgres \
  -v postgres_data:/var/lib/postgresql/data \
  -p 5432:5432 \
  pgvector/pgvector:pg16

# MinIO
docker run -d \
  --name oc-db9-minio \
  --network oc-db9-network \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v minio_data:/data \
  -p 9000:9000 \
  -p 9001:9001 \
  minio/minio server /data --console-address ":9001"

# API
docker run -d \
  --name oc-db9-api \
  --network oc-db9-network \
  -e DATABASE_URL="postgres://postgres:postgres@oc-db9-postgres:5432/postgres" \
  -e MINIO_ENDPOINT="oc-db9-minio:9000" \
  -e MINIO_ACCESS_KEY=minioadmin \
  -e MINIO_SECRET_KEY=minioadmin \
  -p 8080:8080 \
  oc-db9-api:latest
```

---

## 手动部署

### 1. 安装依赖

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y postgresql-16 postgresql-16-pgvector

# 添加 MinIO 仓库并安装
wget https://dl.min.io/server/minio/release/linux-amd64/minio
sudo chmod +x minio
sudo mv minio /usr/local/bin/
```

### 2. 配置 PostgreSQL

```bash
# 启动 PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# 创建数据库和扩展
sudo -u postgres psql -c "CREATE EXTENSION IF NOT EXISTS vector;"

# 创建初始化脚本
sudo -u postgres psql -f init-scripts/01-extensions.sql
```

### 3. 配置 MinIO

```bash
# 创建数据目录
sudo mkdir -p /data/minio
sudo chown $USER:$USER /data/minio

# 启动 MinIO
minio server /data/minio --console-address ":9001" &
```

### 4. 编译并运行 API

```bash
# 安装 Go
wget https://go.dev/dl/go1.23.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 编译
cd api
go build -o oc-db9-api ./cmd/api

# 运行
DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres" \
MINIO_ENDPOINT="localhost:9000" \
MINIO_ACCESS_KEY="minioadmin" \
MINIO_SECRET_KEY="minioadmin" \
./oc-db9-api
```

### 5. 创建 Systemd 服务

```ini
# /etc/systemd/system/oc-db9-api.service
[Unit]
Description=OpenClaw-db9 API Service
After=network.target postgresql.service

[Service]
Type=simple
User=oc-db9
WorkingDirectory=/opt/oc-db9
Environment="DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres"
Environment="MINIO_ENDPOINT=localhost:9000"
Environment="MINIO_ACCESS_KEY=minioadmin"
Environment="MINIO_SECRET_KEY=minioadmin"
Environment="API_PORT=8080"
ExecStart=/opt/oc-db9/oc-db9-api
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable oc-db9-api
sudo systemctl start oc-db9-api
```

---

## 配置说明

### 环境变量

| 变量 | 描述 | 默认值 | 必填 |
|------|------|--------|------|
| `DATABASE_URL` | PostgreSQL 连接字符串 | - | 是 |
| `MINIO_ENDPOINT` | MinIO 服务端点 | `minio:9000` | 是 |
| `MINIO_ACCESS_KEY` | MinIO 访问密钥 | `minioadmin` | 是 |
| `MINIO_SECRET_KEY` | MinIO 密钥 | `minioadmin` | 是 |
| `OLLAMA_BASE_URL` | Ollama 服务地址 | `http://ollama:11434` | 否 |
| `API_PORT` | API 服务端口 | `8080` | 否 |
| `ENVIRONMENT` | 运行环境 | `development` | 否 |

### 数据库配置

编辑 `postgresql.conf`：

```ini
# 内存配置
shared_buffers = 256MB
effective_cache_size = 1GB

# 连接配置
max_connections = 100

# 日志配置
logging_collector = on
log_directory = 'pg_log'
```

编辑 `pg_hba.conf`：

```ini
# 仅允许本地连接（生产环境）
host    all             all             127.0.0.1/32            scram-sha-256
```

---

## 安全建议

### 1. 密码安全

```bash
# 生成强密码
openssl rand -base64 32

# 在 .env 中设置
POSTGRES_PASSWORD=$(openssl rand -base64 32)
MINIO_ROOT_PASSWORD=$(openssl rand -base64 32)
```

### 2. 网络安全

```yaml
# docker-compose.yml - 不暴露端口到外部
services:
  postgres:
    ports: []  # 仅内部网络访问
  minio:
    ports: []  # 仅内部网络访问
  api:
    ports:
      - "127.0.0.1:8080:8080"  # 仅本地访问
```

### 3. TLS/SSL

```yaml
# 添加 HTTPS 支持
services:
  api:
    environment:
      TLS_ENABLED: "true"
      TLS_CERT: "/etc/ssl/certs/server.crt"
      TLS_KEY: "/etc/ssl/private/server.key"
    volumes:
      - ./certs:/etc/ssl:ro
```

### 4. 备份策略

```bash
# 自动备份脚本
#!/bin/bash
BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d)

# 备份 PostgreSQL
docker exec oc-db9-postgres pg_dump -U postgres postgres > $BACKUP_DIR/db_$DATE.sql

# 备份 MinIO
mc mirror local/oc-db9 $BACKUP_DIR/minio_$DATE/

# 清理旧备份（保留 7 天）
find $BACKUP_DIR -type f -mtime +7 -delete
```

---

## 故障排除

### 常见问题

#### 1. 容器无法启动

```bash
# 查看日志
docker compose logs api

# 检查端口占用
netstat -tlnp | grep 8080

# 重新构建
docker compose down -v
docker compose up -d --build
```

#### 2. 数据库连接失败

```bash
# 检查 PostgreSQL 状态
docker exec oc-db9-postgres pg_isready -U postgres

# 测试连接
docker exec oc-db9-postgres psql -U postgres -c "SELECT 1"

# 检查网络
docker network inspect db9_default
```

#### 3. MinIO 连接问题

```bash
# 检查 MinIO 状态
curl http://localhost:9000/minio/health/live

# 使用 mc 客户端测试
mc alias set local http://localhost:9000 minioadmin minioadmin
mc admin info local
```

#### 4. 磁盘空间不足

```bash
# 检查磁盘使用
df -h

# 清理 Docker 资源
docker system prune -a

# 清理旧备份
find /backups -type f -mtime +30 -delete
```

### 日志查看

```bash
# 查看所有服务日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f api
docker compose logs -f postgres

# 查看最近 100 行
docker compose logs --tail=100 api
```

### 性能监控

```bash
# 容器资源使用
docker stats

# PostgreSQL 性能
docker exec oc-db9-postgres psql -c "SELECT * FROM pg_stat_activity;"

# 系统资源
htop
```

---

## 升级指南

```bash
# 1. 备份数据
docker exec oc-db9-postgres pg_dump -U postgres postgres > backup.sql

# 2. 拉取最新代码
git pull origin main

# 3. 重新构建并启动
docker compose down
docker compose up -d --build

# 4. 验证
curl http://localhost:8080/health
```
