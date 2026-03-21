#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from "@modelcontextprotocol/sdk/types.js";
import axios from "axios";

// 配置
const API_BASE_URL = process.env.OC_DB9_API_URL || "http://localhost:8080";

// 工具定义
const TOOLS: Tool[] = [
  {
    name: "create_database",
    description: "创建一个新的 PostgreSQL 数据库实例",
    inputSchema: {
      type: "object",
      properties: {
        name: {
          type: "string",
          description: "数据库名称（只能包含字母、数字、下划线）"
        },
        description: {
          type: "string",
          description: "数据库描述（可选）"
        }
      },
      required: ["name"]
    }
  },
  {
    name: "list_databases",
    description: "列出所有数据库",
    inputSchema: {
      type: "object",
      properties: {}
    }
  },
  {
    name: "execute_sql",
    description: "在指定数据库执行 SQL 语句",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        sql: {
          type: "string",
          description: "SQL 语句"
        },
        params: {
          type: "array",
          description: "参数化查询参数（可选）",
          items: { type: "string" }
        }
      },
      required: ["database_id", "sql"]
    }
  },
  {
    name: "get_connection_info",
    description: "获取数据库连接信息",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        }
      },
      required: ["database_id"]
    }
  },
  {
    name: "create_vector_table",
    description: "创建向量存储表用于语义搜索",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        table_name: {
          type: "string",
          description: "表名"
        },
        dimensions: {
          type: "number",
          description: "向量维度（默认 768）",
          default: 768
        }
      },
      required: ["database_id", "table_name"]
    }
  },
  {
    name: "insert_vector",
    description: "插入文本并自动生成向量嵌入",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        table_name: {
          type: "string",
          description: "向量表名"
        },
        content: {
          type: "string",
          description: "文本内容"
        },
        metadata: {
          type: "object",
          description: "可选元数据"
        }
      },
      required: ["database_id", "table_name", "content"]
    }
  },
  {
    name: "search_similar",
    description: "向量相似性搜索",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        table_name: {
          type: "string",
          description: "向量表名"
        },
        query: {
          type: "string",
          description: "查询文本"
        },
        limit: {
          type: "number",
          description: "返回结果数量（默认 10）",
          default: 10
        }
      },
      required: ["database_id", "table_name", "query"]
    }
  },
  {
    name: "create_branch",
    description: "从现有数据库创建分支（复制数据）",
    inputSchema: {
      type: "object",
      properties: {
        name: {
          type: "string",
          description: "分支名称"
        },
        source_database_id: {
          type: "string",
          description: "源数据库 ID"
        }
      },
      required: ["name", "source_database_id"]
    }
  },
  {
    name: "create_backup",
    description: "创建数据库备份",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        name: {
          type: "string",
          description: "备份名称"
        }
      },
      required: ["database_id", "name"]
    }
  },
  {
    name: "create_cron_job",
    description: "创建定时任务",
    inputSchema: {
      type: "object",
      properties: {
        database_id: {
          type: "string",
          description: "数据库 ID"
        },
        name: {
          type: "string",
          description: "任务名称"
        },
        schedule: {
          type: "string",
          description: "Cron 表达式（秒 分 时 日 月 周）"
        },
        sql: {
          type: "string",
          description: "执行的 SQL 语句"
        }
      },
      required: ["database_id", "name", "schedule", "sql"]
    }
  }
];

// 创建 MCP Server
const server = new Server(
  {
    name: "oc-db9-mcp-server",
    version: "1.0.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// 处理工具列表请求
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return { tools: TOOLS };
});

// 处理工具调用
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case "create_database": {
        const response = await axios.post(`${API_BASE_URL}/api/v1/databases`, {
          name: args.name,
          description: args.description
        });
        return {
          content: [{
            type: "text",
            text: `数据库创建成功！\nID: ${response.data.id}\n名称: ${response.data.name}\n连接字符串: ${response.data.connection_string || 'N/A'}`
          }]
        };
      }

      case "list_databases": {
        const response = await axios.get(`${API_BASE_URL}/api/v1/databases`);
        const databases = response.data || [];
        const list = databases.map((db: any) => 
          `- ${db.name} (ID: ${db.id})`
        ).join('\n');
        return {
          content: [{
            type: "text",
            text: `共有 ${databases.length} 个数据库:\n${list || '暂无数据库'}`
          }]
        };
      }

      case "execute_sql": {
        const response = await axios.post(
          `${API_BASE_URL}/api/v1/databases/${args.database_id}/sql`,
          {
            SQL: args.sql,
            params: args.params || []
          }
        );
        const results = response.data.results;
        let text = "SQL 执行成功！\n";
        if (results && results.length > 0) {
          text += `\n返回 ${results.length} 行数据:\n`;
          text += JSON.stringify(results, null, 2);
        } else {
          text += "\n无返回数据（可能是 INSERT/UPDATE/DELETE 操作）";
        }
        return {
          content: [{ type: "text", text }]
        };
      }

      case "get_connection_info": {
        const response = await axios.get(
          `${API_BASE_URL}/api/v1/databases/${args.database_id}/connect`
        );
        const conn = response.data;
        return {
          content: [{
            type: "text",
            text: `连接信息:\n主机: ${conn.host}\n端口: ${conn.port}\n数据库: ${conn.database}\n用户: ${conn.user}\n密码: ${conn.password}\n连接字符串: ${conn.connection_string}`
          }]
        };
      }

      case "create_vector_table": {
        const response = await axios.post(
          `${API_BASE_URL}/api/v1/embeddings/tables`,
          {
            database_id: args.database_id,
            table_name: args.table_name,
            dimensions: args.dimensions || 768
          }
        );
        return {
          content: [{
            type: "text",
            text: `向量表创建成功！\n表名: ${args.table_name}\n维度: ${args.dimensions || 768}`
          }]
        };
      }

      case "insert_vector": {
        const response = await axios.post(
          `${API_BASE_URL}/api/v1/embeddings/insert`,
          {
            database_id: args.database_id,
            table_name: args.table_name,
            content: args.content,
            metadata: args.metadata || {}
          }
        );
        return {
          content: [{
            type: "text",
            text: `向量插入成功！\nID: ${response.data.id}`
          }]
        };
      }

      case "search_similar": {
        const response = await axios.post(
          `${API_BASE_URL}/api/v1/embeddings/search`,
          {
            database_id: args.database_id,
            table_name: args.table_name,
            query: args.query,
            limit: args.limit || 10
          }
        );
        const results = response.data.results || [];
        let text = `找到 ${results.length} 个相似结果:\n\n`;
        results.forEach((item: any, index: number) => {
          text += `[${index + 1}] 相似度: ${(item.similarity * 100).toFixed(2)}%\n`;
          text += `内容: ${item.content}\n`;
          if (item.metadata) {
            text += `元数据: ${JSON.stringify(item.metadata)}\n`;
          }
          text += '\n';
        });
        return {
          content: [{ type: "text", text }]
        };
      }

      case "create_branch": {
        const response = await axios.post(`${API_BASE_URL}/api/v1/branches`, {
          name: args.name,
          source_database_id: args.source_database_id
        });
        return {
          content: [{
            type: "text",
            text: `分支创建成功！\n分支名: ${response.data.name}\n分支 ID: ${response.data.id}`
          }]
        };
      }

      case "create_backup": {
        const response = await axios.post(`${API_BASE_URL}/api/v1/backups`, {
          database_id: args.database_id,
          name: args.name
        });
        return {
          content: [{
            type: "text",
            text: `备份创建成功！\n备份名: ${response.data.name}\n备份 ID: ${response.data.id}\n状态: ${response.data.status}`
          }]
        };
      }

      case "create_cron_job": {
        const response = await axios.post(`${API_BASE_URL}/api/v1/cron`, {
          database_id: args.database_id,
          name: args.name,
          schedule: args.schedule,
          sql: args.sql
        });
        return {
          content: [{
            type: "text",
            text: `定时任务创建成功！\n任务名: ${response.data.name}\n调度: ${response.data.schedule}\n状态: ${response.data.active ? '启用' : '禁用'}`
          }]
        };
      }

      default:
        throw new Error(`未知工具: ${name}`);
    }
  } catch (error: any) {
    const errorMessage = error.response?.data?.error || error.message || "未知错误";
    return {
      content: [{
        type: "text",
        text: `错误: ${errorMessage}`
      }],
      isError: true
    };
  }
});

// 启动服务器
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("OpenClaw-db9 MCP Server 已启动");
}

main().catch((error) => {
  console.error("服务器错误:", error);
  process.exit(1);
});
