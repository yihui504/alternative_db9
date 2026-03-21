-- fs9 文件系统桥接扩展
-- 提供 SQL 接口查询 MinIO 中的文件

-- 创建文件查询函数（通过 http 调用 API）
CREATE EXTENSION IF NOT EXISTS http;

-- fs9() 函数 - 查询文件内容
CREATE OR REPLACE FUNCTION fs9(file_path TEXT)
RETURNS TABLE (
    _line_number BIGINT,
    _path TEXT,
    data JSONB
) AS $$
DECLARE
    api_url TEXT;
    response JSONB;
    row_data JSONB;
BEGIN
    -- 从配置获取 API URL
    api_url := current_setting('app.api_url', true);
    IF api_url IS NULL OR api_url = '' THEN
        api_url := 'http://localhost:8080';
    END IF;

    -- 调用 API 获取文件内容
    SELECT content::JSONB INTO response
    FROM http_get(
        api_url || '/api/v1/files/query?database_id=' ||
        current_setting('app.current_database_id', true) ||
        '&path=' || uri_encode(file_path)
    );

    -- 返回结果集中的每一行
    FOR row_data IN SELECT jsonb_array_elements(response->'results')
    LOOP
        _line_number := (row_data->>'_line_number')::BIGINT;
        _path := row_data->>'_path';
        data := row_data - '_line_number' - '_path';
        RETURN NEXT;
    END LOOP;

    RETURN;
END;
$$ LANGUAGE plpgsql;

-- 辅助函数：URI 编码
CREATE OR REPLACE FUNCTION uri_encode(str TEXT)
RETURNS TEXT AS $$
DECLARE
    result TEXT := '';
    ch TEXT;
BEGIN
    FOR i IN 1..length(str) LOOP
        ch := substr(str, i, 1);
        IF ch ~ '[A-Za-z0-9_.~-]' THEN
            result := result || ch;
        ELSE
            result := result || '%' || encode(ch::bytea, 'hex');
        END IF;
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 创建视图简化查询
CREATE OR REPLACE VIEW fs9_files AS
SELECT
    f.id,
    f.database_id,
    f.path,
    f.size,
    f.created_at
FROM oc_files f;

-- 添加注释
COMMENT ON FUNCTION fs9(TEXT) IS 'Query file content as virtual table. Usage: SELECT * FROM fs9(''/path/to/file.csv'')';
