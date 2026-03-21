package config

type TableSchema struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsVector bool   `json:"is_vector,omitempty"`
}

type Index struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Column string `json:"column"`
}

type Trigger struct {
	Name   string `json:"name"`
	Timing string `json:"timing"`
	Event  string `json:"event"`
	Func   string `json:"function"`
}

type DatabaseTemplate struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Tables        []TableSchema `json:"tables"`
	Indexes       []Index       `json:"indexes"`
	Triggers      []Trigger     `json:"triggers"`
	VectorDim     int           `json:"vector_dimension,omitempty"`
	RetentionDays int           `json:"retention_days,omitempty"`
}

var Templates = map[string]DatabaseTemplate{
	"ai-memory": {
		ID:          "ai-memory",
		Name:        "AI Agent Memory",
		Description: "AI Agent 跨对话记忆存储模板，包含用户偏好、对话历史、知识库",
		Tables: []TableSchema{
			{
				Name: "user_preferences",
				Columns: []Column{
					{Name: "user_id", Type: "TEXT NOT NULL"},
					{Name: "key", Type: "TEXT NOT NULL"},
					{Name: "value", Type: "JSONB NOT NULL"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
					{Name: "PRIMARY KEY", Type: "(user_id, key)"},
				},
			},
			{
				Name: "conversation_history",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "user_id", Type: "UUID NOT NULL"},
					{Name: "session_id", Type: "TEXT NOT NULL"},
					{Name: "role", Type: "TEXT NOT NULL"},
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "knowledge_base",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "embedding", Type: "vector(1536)"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_conv_user", Type: "btree", Column: "user_id"},
			{Name: "idx_conv_session", Type: "btree", Column: "session_id"},
			{Name: "idx_kb_embedding", Type: "ivfflat", Column: "embedding"},
		},
		VectorDim:     1536,
		RetentionDays: 90,
	},
	"workflow-state": {
		ID:          "workflow-state",
		Name:        "Workflow State",
		Description: "工作流状态管理模板，包含任务状态、执行历史、事件日志",
		Tables: []TableSchema{
			{
				Name: "workflow_instances",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "name", Type: "TEXT NOT NULL"},
					{Name: "status", Type: "TEXT NOT NULL DEFAULT 'pending'"},
					{Name: "state", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "workflow_events",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "instance_id", Type: "UUID REFERENCES workflow_instances(id)"},
					{Name: "event_type", Type: "TEXT NOT NULL"},
					{Name: "payload", Type: "JSONB"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_wf_status", Type: "btree", Column: "status"},
			{Name: "idx_wf_event_instance", Type: "btree", Column: "instance_id"},
		},
	},
	"knowledge-base": {
		ID:          "knowledge-base",
		Name:        "Knowledge Base",
		Description: "企业知识库模板，支持向量检索和标签分类",
		Tables: []TableSchema{
			{
				Name: "documents",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "title", Type: "TEXT NOT NULL"},
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "embedding", Type: "vector(1536)"},
					{Name: "tags", Type: "TEXT[] DEFAULT '{}'"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "tags",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "name", Type: "TEXT UNIQUE NOT NULL"},
					{Name: "color", Type: "TEXT DEFAULT '#6366f1'"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_doc_embedding", Type: "ivfflat", Column: "embedding"},
			{Name: "idx_doc_tags", Type: "gin", Column: "tags"},
		},
		VectorDim: 1536,
	},
}
