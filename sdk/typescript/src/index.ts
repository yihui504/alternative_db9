import { Pool, PoolClient, QueryResult } from 'pg';
import * as fs from 'fs';
import * as path from 'path';

/**
 * OpenClaw-db9 TypeScript SDK
 * 
 * 让 AI Agent 可以方便地使用 PostgreSQL + 文件系统
 * 
 * @example
 * ```typescript
 * import { instantDatabase } from 'oc-db9';
 * 
 * const db = await instantDatabase({
 *   name: 'myapp',
 *   seed: `CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT)`
 * });
 * 
 * await db.query('INSERT INTO users (name) VALUES ($1)', ['Alice']);
 * const result = await db.query('SELECT * FROM users');
 * console.log(result.rows);
 * ```
 */

export interface DatabaseConfig {
  /** 数据库名称 */
  name: string;
  /** 可选：初始化 SQL */
  seed?: string;
  /** 可选：API 基础 URL */
  apiUrl?: string;
}

export interface DatabaseInfo {
  databaseId: string;
  name: string;
  connectionString: string;
  adminUser: string;
  state: string;
}

export interface FileInfo {
  id: string;
  database_id: string;
  path: string;
  size: number;
  created_at: string;
}

/**
 * OpenClaw-db9 数据库客户端
 */
export class OCDB9Client {
  private apiUrl: string;
  private pool: Pool | null = null;
  private dbInfo: DatabaseInfo | null = null;

  constructor(apiUrl: string = 'http://localhost:8080') {
    this.apiUrl = apiUrl;
  }

  /**
   * 创建或获取数据库
   * 
   * 如果数据库已存在，直接返回现有数据库
   * 这是幂等操作，可以安全地多次调用
   */
  async instantDatabase(config: DatabaseConfig): Promise<DatabaseConnection> {
    // 1. 尝试创建数据库
    const createResponse = await fetch(`${this.apiUrl}/api/v1/databases`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: config.name }),
    });

    if (!createResponse.ok && createResponse.status !== 409) {
      throw new Error(`Failed to create database: ${await createResponse.text()}`);
    }

    // 2. 获取数据库信息
    const dbInfo = await this.getDatabase(config.name);
    this.dbInfo = dbInfo;

    // 3. 创建连接池
    this.pool = new Pool({
      connectionString: dbInfo.connectionString,
      ssl: { rejectUnauthorized: false },
    });

    // 4. 执行 seed SQL（如果有）
    if (config.seed) {
      try {
        await this.pool.query(config.seed);
      } catch (err) {
        // 如果表已存在等错误，忽略
        console.log('Seed SQL executed (some statements may have been skipped)');
      }
    }

    return new DatabaseConnection(this.pool, dbInfo);
  }

  /**
   * 获取数据库信息
   */
  async getDatabase(name: string): Promise<DatabaseInfo> {
    const response = await fetch(`${this.apiUrl}/api/v1/databases`);
    if (!response.ok) {
      throw new Error(`Failed to list databases: ${await response.text()}`);
    }

    const databases = await response.json();
    const db = databases.find((d: any) => d.name === name);
    
    if (!db) {
      throw new Error(`Database ${name} not found`);
    }

    return {
      databaseId: db.id,
      name: db.name,
      connectionString: db.connection_string,
      adminUser: db.admin_user,
      state: db.state,
    };
  }

  /**
   * 列出所有数据库
   */
  async listDatabases(): Promise<DatabaseInfo[]> {
    const response = await fetch(`${this.apiUrl}/api/v1/databases`);
    if (!response.ok) {
      throw new Error(`Failed to list databases: ${await response.text()}`);
    }

    const databases = await response.json();
    return databases.map((db: any) => ({
      databaseId: db.id,
      name: db.name,
      connectionString: db.connection_string,
      adminUser: db.admin_user,
      state: db.state,
    }));
  }

  /**
   * 删除数据库
   */
  async deleteDatabase(databaseId: string): Promise<void> {
    const response = await fetch(`${this.apiUrl}/api/v1/databases/${databaseId}`, {
      method: 'DELETE',
    });

    if (!response.ok) {
      throw new Error(`Failed to delete database: ${await response.text()}`);
    }
  }
}

/**
 * 数据库连接
 * 
 * 提供 SQL 查询和文件操作功能
 */
export class DatabaseConnection {
  constructor(
    private pool: Pool,
    private dbInfo: DatabaseInfo
  ) {}

  /**
   * 执行 SQL 查询
   */
  async query(sql: string, params?: any[]): Promise<QueryResult> {
    return this.pool.query(sql, params);
  }

  /**
   * 获取数据库信息
   */
  getInfo(): DatabaseInfo {
    return this.dbInfo;
  }

  /**
   * 上传文件
   * 
   * @param localPath 本地文件路径
   * @param remotePath 远程路径（如 /data/file.csv）
   */
  async uploadFile(localPath: string, remotePath: string): Promise<FileInfo> {
    const content = fs.readFileSync(localPath);
    const filename = path.basename(localPath);

    const formData = new FormData();
    formData.append('file', new Blob([content]), filename);
    formData.append('database_id', this.dbInfo.databaseId);
    formData.append('path', remotePath);

    const response = await fetch(`http://localhost:8080/api/v1/files/upload`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      throw new Error(`Failed to upload file: ${await response.text()}`);
    }

    return await response.json();
  }

  /**
   * 查询文件内容
   * 
   * 支持 CSV/JSONL/Parquet 格式
   */
  async queryFile(filePath: string): Promise<any[]> {
    const response = await fetch(
      `http://localhost:8080/api/v1/files/query?database_id=${this.dbInfo.databaseId}&path=${encodeURIComponent(filePath)}`
    );

    if (!response.ok) {
      throw new Error(`Failed to query file: ${await response.text()}`);
    }

    const result = await response.json();
    return result.results;
  }

  /**
   * 用 SQL 查询文件
   * 
   * 使用 fs9() 函数在 SQL 中查询文件
   */
  async queryFileWithSQL(filePath: string, whereClause?: string): Promise<QueryResult> {
    let sql = `SELECT * FROM fs9('${filePath}')`;
    if (whereClause) {
      sql += ` WHERE ${whereClause}`;
    }
    return this.pool.query(sql);
  }

  /**
   * 列出文件
   */
  async listFiles(): Promise<FileInfo[]> {
    const response = await fetch(
      `http://localhost:8080/api/v1/files?database_id=${this.dbInfo.databaseId}`
    );

    if (!response.ok) {
      throw new Error(`Failed to list files: ${await response.text()}`);
    }

    return await response.json();
  }

  /**
   * 生成 embedding
   * 
   * 自动调用内置的 embedding 函数
   */
  async embedding(text: string): Promise<number[]> {
    const result = await this.pool.query('SELECT embedding($1) as vec', [text]);
    return result.rows[0].vec;
  }

  /**
   * 向量相似度搜索
   * 
   * @example
   * ```typescript
   * const results = await db.similaritySearch(
   *   'docs',
   *   'query text',
   *   'content',
   *   'vec',
   *   5
   * );
   * ```
   */
  async similaritySearch(
    table: string,
    query: string,
    contentColumn: string = 'content',
    vectorColumn: string = 'vec',
    limit: number = 5
  ): Promise<any[]> {
    const sql = `
      SELECT ${contentColumn}, ${vectorColumn} <-> embedding($1) as distance
      FROM ${table}
      ORDER BY ${vectorColumn} <-> embedding($1)
      LIMIT $2
    `;
    const result = await this.pool.query(sql, [query, limit]);
    return result.rows;
  }

  /**
   * 关闭连接
   */
  async close(): Promise<void> {
    await this.pool.end();
  }
}

/**
 * 便捷函数：创建或获取数据库
 * 
 * 这是最主要的入口函数，对标 db9.ai 的 instantDatabase
 * 
 * @example
 * ```typescript
 * const db = await instantDatabase({
 *   name: 'myapp',
 *   seed: `CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT)`
 * });
 * 
 * // 直接使用
 * await db.query('INSERT INTO users (name) VALUES ($1)', ['Alice']);
 * const result = await db.query('SELECT * FROM users');
 * 
 * // 上传文件并查询
 * await db.uploadFile('./data.csv', '/data.csv');
 * const data = await db.queryFile('/data.csv');
 * 
 * // 向量搜索
 * const similar = await db.similaritySearch('docs', 'search query');
 * ```
 */
export async function instantDatabase(config: DatabaseConfig): Promise<DatabaseConnection> {
  const client = new OCDB9Client(config.apiUrl);
  return client.instantDatabase(config);
}

// 导出所有类型
export { Pool, PoolClient, QueryResult } from 'pg';
