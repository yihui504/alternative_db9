CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS oc_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oc_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID REFERENCES oc_accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    postgres_db_name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(account_id, name)
);

CREATE TABLE IF NOT EXISTS oc_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    bucket_path TEXT NOT NULL,
    size BIGINT,
    checksum TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(database_id, path)
);

CREATE TABLE IF NOT EXISTS oc_cron_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,
    sql_command TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oc_cron_job_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES oc_cron_jobs(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    message TEXT,
    executed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oc_branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    source_branch TEXT,
    snapshot_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oc_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id) ON DELETE CASCADE,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    model_name TEXT NOT NULL,
    dimension INT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oc_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    size BIGINT,
    bucket_path TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oc_databases_account_id ON oc_databases(account_id);
CREATE INDEX IF NOT EXISTS idx_oc_files_database_id ON oc_files(database_id);
CREATE INDEX IF NOT EXISTS idx_oc_cron_jobs_database_id ON oc_cron_jobs(database_id);
CREATE INDEX IF NOT EXISTS idx_oc_branches_database_id ON oc_branches(database_id);
CREATE INDEX IF NOT EXISTS idx_oc_embeddings_database_id ON oc_embeddings(database_id);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_oc_files_updated_at
    BEFORE UPDATE ON oc_files
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE OR REPLACE FUNCTION create_vector_table(
    table_name TEXT,
    dimension INT DEFAULT 1536
) RETURNS TEXT AS $$
DECLARE
    sql_text TEXT;
BEGIN
    sql_text := format('
        CREATE TABLE IF NOT EXISTS %I (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            content TEXT,
            embedding vector(%s),
            metadata JSONB DEFAULT ''{}''::jsonb,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', table_name, dimension);
    
    EXECUTE sql_text;
    
    sql_text := format('
        CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %I 
        USING ivfflat (embedding vector_cosine_ops)
        WITH (lists = 100)', table_name, table_name);
    
    EXECUTE sql_text;
    
    RETURN table_name;
END;
$$ LANGUAGE plpgsql;
