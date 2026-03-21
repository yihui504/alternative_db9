# OpenClaw-db9 MCP Server

为 OpenClaw-db9 提供的 MCP (Model Context Protocol) 服务器，让 AI Agent 可以直接使用数据库服务。

## 功能

MCP Server 提供以下工具供 AI Agent 调用：

### 数据库管理
- `create_database` - 创建新数据库
- `list_databases` - 列出所有数据库
- `execute_sql` - 执行 SQL 语句
- `get_connection_info` - 获取连接信息

### 向量搜索
- `create_vector_table` - 创建向量表
- `insert_vector` - 插入文本并自动生成向量
- `search_similar` - 相似性搜索

### 其他功能
- `create_branch` - 创建数据库分支
- `create_backup` - 创建备份
- `create_cron_job` - 创建定时任务

## 安装

```bash
# 进入 MCP Server 目录
cd mcp-server

# 安装依赖
npm install

# 构建
npm run build
```

## 配置

### 环境变量

```bash
export OC_DB9_API_URL="http://localhost:8080"  # oc-db9 API 地址
```

### Claude Desktop 配置

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) 或 `%APPDATA%/Claude/claude_desktop_config.json` (Windows)：

```json
{
  "mcpServers": {
    "oc-db9": {
      "command": "node",
      "args": ["/path/to/mcp-server/dist/index.js"],
      "env": {
        "OC_DB9_API_URL": "http://localhost:8080"
      }
    }
  }
}
```

## 使用示例

配置完成后，在 Claude 中可以直接使用：

**用户**: "帮我创建一个博客数据库"

**Claude**: 调用 `create_database` 工具创建数据库

**用户**: "创建文章表，包含标题和内容"

**Claude**: 调用 `execute_sql` 执行 CREATE TABLE

**用户**: "搜索关于数据库的文章"

**Claude**: 调用 `search_similar` 进行向量搜索

## 开发

```bash
# 开发模式
npm run dev

# 构建
npm run build
```

## 许可证

MIT
