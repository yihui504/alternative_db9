-- embedding 扩展
-- 提供内置的向量嵌入功能

-- embedding() 函数 - 生成文本的向量表示
CREATE OR REPLACE FUNCTION embedding(
    input_text TEXT,
    model_name TEXT DEFAULT 'nomic-embed-text'
)
RETURNS vector(768) AS $$
DECLARE
    api_url TEXT;
    response JSONB;
    embedding_vector vector(768);
BEGIN
    -- 获取 API URL
    api_url := current_setting('app.api_url', true);
    IF api_url IS NULL OR api_url = '' THEN
        api_url := 'http://localhost:8080';
    END IF;

    -- 调用 embedding API
    SELECT content::JSONB INTO response
    FROM http_post(
        url := api_url || '/api/v1/embeddings/generate',
        body := jsonb_build_object(
            'text', input_text,
            'model', model_name
        )
    );

    -- 解析向量
    SELECT array_agg(x::float8)::vector(768) INTO embedding_vector
    FROM jsonb_array_elements_text(response->'embedding') AS x;

    RETURN embedding_vector;
END;
$$ LANGUAGE plpgsql;

-- 相似度搜索辅助函数
CREATE OR REPLACE FUNCTION similarity(
    vec1 vector,
    vec2 vector
)
RETURNS float AS $$
BEGIN
    RETURN 1 - (vec1 <=> vec2);  -- 余弦相似度
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION embedding(TEXT, TEXT) IS 'Generate embedding vector for text using configured model';
