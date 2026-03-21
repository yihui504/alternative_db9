package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Agent 类型和配置路径映射
var agentConfigs = map[string]struct {
	name        string
	skillDir    string
	skillFile   string
	description string
}{
	"claude": {
		name:        "Claude Code",
		skillDir:    ".claude-code",
		skillFile:   "oc-db9.md",
		description: "Claude Code 的命令行工具",
	},
	"codex": {
		name:        "OpenAI Codex",
		skillDir:    ".codex",
		skillFile:   "oc-db9.md",
		description: "OpenAI 的 Codex CLI",
	},
	"cursor": {
		name:        "Cursor",
		skillDir:    ".cursor",
		skillFile:   "oc-db9.md",
		description: "Cursor 编辑器",
	},
	"cline": {
		name:        "Cline",
		skillDir:    ".cline",
		skillFile:   "oc-db9.md",
		description: "Cline VS Code 扩展",
	},
	"opencode": {
		name:        "OpenCode",
		skillDir:    ".opencode",
		skillFile:   "oc-db9.md",
		description: "OpenCode 编辑器",
	},
}

// skillContent 是提供给 Agent 的 skill 文件内容
const skillContent = `# OpenClaw-db9 Skill

你是 OpenClaw-db9 数据库平台的专家助手。这个平台让 AI Agent 可以方便地使用 PostgreSQL + 文件系统。

## 核心概念

OpenClaw-db9 是一个面向 Agent 的数据库平台，特点是：
1. **即时数据库** - 秒级创建 PostgreSQL 数据库
2. **文件即表** - 用 SQL 直接查询 CSV/JSONL/Parquet 文件
3. **向量搜索** - 内置 embedding，无需外部服务
4. **自托管** - 完全开源，数据自主可控

## 你应该如何使用

### 场景 1：用户需要存储数据

如果用户说"帮我存一些数据"或"我需要数据库"：

1. **创建数据库**
   ` + "`" + `oc-db9 db create <name>` + "`" + `

2. **如果用户有文件**
   - 上传文件：` + "`" + `oc-db9 fs cp <file> :<path> --db <name>` + "`" + `
   - 查询文件：` + "`" + `oc-db9 fs query <path> --db <name>` + "`" + `
   - 导入到表：` + "`" + `oc-db9 db sql <name> -q "INSERT INTO table SELECT * FROM fs9('<path>')"` + "`" + `

3. **如果用户需要建表**
   ` + "`" + `oc-db9 db sql <name> -q "CREATE TABLE ..."` + "`" + `

### 场景 2：用户需要查询数据

如果用户说"帮我查一下数据"：

1. **先看有哪些数据库**
   ` + "`" + `oc-db9 db list` + "`" + `

2. **执行查询**
   ` + "`" + `oc-db9 db sql <db> -q "SELECT ..."` + "`" + `

3. **如果有文件需要查询**
   ` + "`" + `oc-db9 db sql <db> -q "SELECT * FROM fs9('<path>')"` + "`" + `

### 场景 3：用户需要向量搜索

如果用户说"帮我做语义搜索"或"找相似的内容"：

1. **创建带向量的表**
   ` + "`" + `oc-db9 db sql <db> -q "CREATE TABLE docs (id SERIAL PRIMARY KEY, content TEXT, vec vector(768))"` + "`" + `

2. **生成向量**（自动调用 embedding）
   ` + "`" + `oc-db9 db sql <db> -q "UPDATE docs SET vec = embedding(content)"` + "`" + `

3. **相似度搜索**
   ` + "`" + `oc-db9 db sql <db> -q "SELECT content FROM docs ORDER BY vec <-> embedding('查询文本') LIMIT 5"` + "`" + `

## 文件格式支持

| 格式 | 使用场景 | 命令 |
|------|---------|------|
| CSV | 小数据、表格数据 | ` + "`" + `oc-db9 fs query /path.csv --db <db>` + "`" + ` |
| JSONL | 日志、事件流 | ` + "`" + `oc-db9 fs query /path.jsonl --db <db>` + "`" + ` |
| Parquet | 大数据、分析 | ` + "`" + `oc-db9 fs query /path.parquet --db <db>` + "`" + ` |

## 常用命令速查

```bash
# 数据库管理
oc-db9 db create <name>              # 创建数据库
oc-db9 db list                       # 列出所有数据库
oc-db9 db sql <name> -q "..."        # 执行 SQL
oc-db9 db connect <name>             # 获取连接字符串

# 文件管理
oc-db9 fs cp <local> :<remote> --db <id>    # 上传文件
oc-db9 fs ls --db <id>                       # 列出文件
oc-db9 fs query <path> --db <id>            # 查询文件内容
oc-db9 fs cat <path> --db <id>              # 查看文件内容

# 类型生成
oc-db9 gen types <name>              # 生成 TypeScript 类型
```

## 最佳实践

1. **优先使用 fs query 预览** - 导入前先看看数据结构
2. **大数据用 Parquet** - 比 CSV 快 10-100 倍
3. **利用 fs9() 函数** - 可以直接在 SQL 中查询文件
4. **向量搜索很方便** - 内置 embedding，无需配置

## 错误处理

如果遇到错误：
1. 检查服务状态：` + "`" + `docker-compose ps` + "`" + `
2. 查看日志：` + "`" + `docker-compose logs -f api` + "`" + `
3. 重置服务：` + "`" + `docker-compose restart` + "`" + `

## 示例对话

用户："帮我分析一下这些销售数据"
你："好的，我来帮你处理。首先让我看看数据文件的结构..."

` + "`" + `oc-db9 db create sales_analysis
oc-db9 fs cp sales_data.csv :/data/sales.csv --db sales_analysis
oc-db9 fs query /data/sales.csv --db sales_analysis` + "`" + `

"我看到数据包含日期、产品、销售额等字段。让我创建一个表并导入数据..."

` + "`" + `oc-db9 db sql sales_analysis -q "
  CREATE TABLE sales (
    id SERIAL PRIMARY KEY,
    date DATE,
    product TEXT,
    amount DECIMAL
  )
"
oc-db9 db sql sales_analysis -q "
  INSERT INTO sales (date, product, amount)
  SELECT 
    (data->>'date')::date,
    data->>'product',
    (data->>'amount')::decimal
  FROM fs9('/data/sales.csv')
"` + "`" + `

"现在可以进行分析查询了。你想看什么维度的分析？"
`

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "为 AI Agent 安装使用技能",
	Long: `为 AI Agent（如 Claude Code、Codex、Cursor 等）安装 OpenClaw-db9 使用技能。

这会在 Agent 的配置目录中放置 skill 文件，教 Agent 如何使用 oc-db9。

支持的 Agent:
- claude: Claude Code 命令行工具
- codex: OpenAI Codex CLI
- cursor: Cursor 编辑器
- cline: Cline VS Code 扩展
- opencode: OpenCode 编辑器

示例:
  # 为 Claude Code 安装
  oc-db9 onboard --agent claude

  # 为所有支持的 Agent 安装
  oc-db9 onboard --all

  # 预览将要安装的内容（不实际安装）
  oc-db9 onboard --agent claude --dry-run
`,
}

var (
	agentFlag string
	allFlag   bool
	dryRunFlag bool
)

func init() {
	rootCmd.AddCommand(onboardCmd)
	onboardCmd.Flags().StringVar(&agentFlag, "agent", "", "指定 Agent 类型 (claude|codex|cursor|cline|opencode)")
	onboardCmd.Flags().BoolVar(&allFlag, "all", false, "为所有支持的 Agent 安装")
	onboardCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "预览将要安装的内容，不实际安装")

	onboardCmd.Run = runOnboard
}

func runOnboard(cmd *cobra.Command, args []string) {
	if !allFlag && agentFlag == "" {
		fmt.Fprintln(os.Stderr, "错误: 请指定 --agent 或 --all")
		fmt.Fprintln(os.Stderr, "\n支持的 Agent:")
		for key, config := range agentConfigs {
			fmt.Fprintf(os.Stderr, "  - %s: %s\n", key, config.description)
		}
		os.Exit(1)
	}

	// 确定要安装的 Agent 列表
	var agentsToInstall []string
	if allFlag {
		for key := range agentConfigs {
			agentsToInstall = append(agentsToInstall, key)
		}
	} else {
		if _, ok := agentConfigs[agentFlag]; !ok {
			fmt.Fprintf(os.Stderr, "错误: 不支持的 Agent 类型: %s\n", agentFlag)
			fmt.Fprintln(os.Stderr, "\n支持的 Agent:")
			for key, config := range agentConfigs {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", key, config.description)
			}
			os.Exit(1)
		}
		agentsToInstall = []string{agentFlag}
	}

	// 安装到每个 Agent
	for _, agent := range agentsToInstall {
		config := agentConfigs[agent]
		if err := installSkill(agent, config); err != nil {
			fmt.Fprintf(os.Stderr, "错误: 为 %s 安装失败: %v\n", config.name, err)
			continue
		}
	}
}

func installSkill(agentType string, config struct {
	name        string
	skillDir    string
	skillFile   string
	description string
}) error {
	// 获取 home 目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("无法获取 home 目录: %w", err)
	}

	// 构建 skill 文件路径
	skillDir := filepath.Join(homeDir, config.skillDir)
	skillPath := filepath.Join(skillDir, config.skillFile)

	if dryRunFlag {
		fmt.Printf("[预览] 将为 %s 安装 skill 文件:\n", config.name)
		fmt.Printf("  目标路径: %s\n", skillPath)
		fmt.Printf("  文件大小: %d 字节\n", len(skillContent))
		return nil
	}

	// 创建目录
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入 skill 文件
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	fmt.Printf("✅ 已为 %s 安装 skill 文件: %s\n", config.name, skillPath)
	fmt.Printf("   现在 %s 知道如何使用 OpenClaw-db9 了！\n", config.name)

	return nil
}
