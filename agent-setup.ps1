# OpenClaw-db9 Zero-Touch Agent Setup Script (PowerShell)
# This script is designed to be executed by AI agents to automatically provision
# a long-term memory database (ai-memory template) and setup the MCP server.

$ErrorActionPreference = "Stop"

Write-Host "🚀 [1/4] Starting OpenClaw-db9 Docker services..."
docker-compose up -d

Write-Host "⏳ [2/4] Waiting for API service to be ready..."
$max_retries = 30
$counter = 0
$api_ready = $false

while (-not $api_ready -and $counter -lt $max_retries) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -Method Get -UseBasicParsing -ErrorAction SilentlyContinue
        if ($response.StatusCode -eq 200) {
            $api_ready = $true
        }
    } catch {
        # Ignore errors and retry
    }
    
    if (-not $api_ready) {
        Start-Sleep -Seconds 1
        $counter++
    }
}

if (-not $api_ready) {
    Write-Error "❌ API service did not start in time. Please check docker logs."
    exit 1
}
Write-Host "✅ API service is ready!"

Write-Host "🧠 [3/4] Provisioning 'ai-memory' database..."
$body = @{
    template = "ai-memory"
    name = "agent-memory"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/databases/quick-setup" -Method Post -Body $body -ContentType "application/json"
    $db_id = $response.database_id
} catch {
    Write-Error "❌ Failed to create database. Error: $_"
    exit 1
}

if (-not $db_id) {
    Write-Error "❌ Failed to get database_id from response."
    exit 1
}

Write-Host "✅ Database provisioned successfully! ID: $db_id"

Write-Host "⚙️  [4/5] Building MCP Server..."
Set-Location mcp-server
npm install
npm run build
Set-Location ..

Write-Host "🛠️  [5/5] Building oc-db9 CLI..."
go build -o oc-db9.exe ./cmd/oc-db9

Write-Host "==========================================================="
Write-Host "🎉 Setup Complete!"
Write-Host "Your AI Long-Term Memory Database is ready."
Write-Host "Database ID: $db_id"
Write-Host "==========================================================="
Write-Host "🤖 AGENT INSTRUCTIONS:"
Write-Host "1. Read the instructions in 'SKILL.md' or '.cursorrules'."
Write-Host "2. You can use the MCP tools OR use the newly built CLI: .\oc-db9.exe memory store --db $db_id --content `"...`""
Write-Host "3. You are now equipped with Long-Term Memory!"
Write-Host "==========================================================="
