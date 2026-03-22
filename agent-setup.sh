#!/bin/bash

# OpenClaw-db9 Zero-Touch Agent Setup Script
# This script is designed to be executed by AI agents to automatically provision
# a long-term memory database (ai-memory template) and setup the MCP server.

echo "🚀 [1/4] Starting OpenClaw-db9 Docker services..."
docker-compose up -d

echo "⏳ [2/4] Waiting for API service to be ready..."
# Wait until the API is healthy (up to 30 seconds)
max_retries=30
counter=0
until curl -s http://localhost:8080/health > /dev/null; do
    sleep 1
    counter=$((counter+1))
    if [ $counter -ge $max_retries ]; then
        echo "❌ API service did not start in time. Please check docker logs."
        exit 1
    fi
done
echo "✅ API service is ready!"

echo "🧠 [3/4] Provisioning 'ai-memory' database..."
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/databases/quick-setup \
  -H "Content-Type: application/json" \
  -d '{
    "template": "ai-memory",
    "name": "agent-memory"
  }')

DB_ID=$(echo $RESPONSE | grep -o '"database_id":"[^"]*' | cut -d'"' -f4)

if [ -z "$DB_ID" ]; then
    echo "❌ Failed to create database. Response: $RESPONSE"
    exit 1
fi

echo "✅ Database provisioned successfully! ID: $DB_ID"

echo "⚙️  [4/5] Building MCP Server..."
cd mcp-server && npm install && npm run build
cd ..

echo "🛠️  [5/5] Building oc-db9 CLI..."
go build -o oc-db9 ./cmd/oc-db9

echo "==========================================================="
echo "🎉 Setup Complete!"
echo "Your AI Long-Term Memory Database is ready."
echo "Database ID: $DB_ID"
echo "==========================================================="
echo "🤖 AGENT INSTRUCTIONS:"
echo "1. Read the instructions in 'SKILL.md' or '.cursorrules'."
echo "2. You can use the MCP tools OR use the newly built CLI: ./oc-db9 memory store --db $DB_ID --content \"...\""
echo "3. You are now equipped with Long-Term Memory!"
echo "==========================================================="
