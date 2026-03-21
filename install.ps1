# OpenClaw-db9 Windows 安装脚本
# 使用方式：以管理员身份运行 PowerShell

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\oc-db9",
    [string]$BinaryDir = "$env:LOCALAPPDATA\Programs\oc-db9"
)

$ErrorActionPreference = "Stop"

Write-Host "=== OpenClaw-db9 安装程序 (Windows) ===" -ForegroundColor Green
Write-Host ""

# 检测 Docker
Write-Host "检查依赖..." -ForegroundColor Yellow

$docker = Get-Command docker -ErrorAction SilentlyContinue
if (-not $docker) {
    Write-Host "错误: 需要安装 Docker" -ForegroundColor Red
    Write-Host "请访问 https://docs.docker.com/desktop/install/windows-install/ 安装 Docker Desktop" -ForegroundColor Cyan
    exit 1
}

$dockerCompose = Get-Command docker-compose -ErrorAction SilentlyContinue
if (-not $dockerCompose) {
    Write-Host "错误: 需要安装 Docker Compose" -ForegroundColor Red
    exit 1
}

Write-Host "Docker 已安装" -ForegroundColor Green

# 检测 Go
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    Write-Host "警告: 未检测到 Go，将尝试下载预编译二进制文件" -ForegroundColor Yellow
    $UsePrebuilt = $true
} else {
    $UsePrebuilt = $false
    Write-Host "Go 已安装: $($go.Version)" -ForegroundColor Green
}

# 获取平台信息
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq "AMD64") {
    $arch = "amd64"
} elseif ($arch -eq "ARM64") {
    $arch = "arm64"
} else {
    Write-Host "不支持的架构: $arch" -ForegroundColor Red
    exit 1
}

Write-Host "检测到架构: $arch"

# 下载最新代码
Write-Host ""
Write-Host "下载 OpenClaw-db9..." -ForegroundColor Yellow

$tempDir = [System.IO.Path]::GetTempPath()
$repoDir = Join-Path $tempDir "oc-db9"

if (Test-Path $repoDir) {
    Remove-Item -Recurse -Force $repoDir
}

git clone --depth 1 https://github.com/yihui504/alternative_db9.git $repoDir

if (-not $LASTEXITCODE -eq 0) {
    Write-Host "错误: 下载失败" -ForegroundColor Red
    exit 1
}

Write-Host "下载完成" -ForegroundColor Green

# 构建或下载二进制文件
Write-Host ""
Write-Host "安装 oc-db9..." -ForegroundColor Yellow

if (-not $UsePrebuilt) {
    Push-Location $repoDir
    try {
        go build -o oc-db9.exe .\internal\cmd
        if (-not $LASTEXITCODE -eq 0) {
            throw "Build failed"
        }
    }
    finally {
        Pop-Location
    }
}
else {
    # 尝试下载预编译版本
    $url = "https://github.com/yihui504/alternative_db9/releases/latest/download/oc-db9-windows-$arch.exe"
    $binaryPath = Join-Path $tempDir "oc-db9.exe"

    try {
        Invoke-WebRequest -Uri $url -OutFile $binaryPath -UseBasicParsing
    }
    catch {
        Write-Host "警告: 无法下载预编译版本，将使用源码构建" -ForegroundColor Yellow
        Push-Location $repoDir
        try {
            go build -o oc-db9.exe .\internal\cmd
        }
        finally {
            Pop-Location
        }
    }
    Copy-Item (Join-Path $tempDir "oc-db9.exe") (Join-Path $repoDir "oc-db9.exe") -ErrorAction SilentlyContinue
}

# 创建安装目录
New-Item -ItemType Directory -Force -Path $BinaryDir | Out-Null

# 复制二进制文件
Copy-Item (Join-Path $repoDir "oc-db9.exe") $BinaryDir

# 添加到 PATH
$pathEntry = $BinaryDir
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$BinaryDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$BinaryDir", "User")
    Write-Host "已将 $BinaryDir 添加到 PATH" -ForegroundColor Green
    Write-Host "请重新启动终端或运行: refreshenv" -ForegroundColor Yellow
}

Write-Host "oc-db9 已安装到 $BinaryDir" -ForegroundColor Green

# 启动服务
Write-Host ""
Write-Host "启动 OpenClaw-db9 服务..." -ForegroundColor Yellow

# 复制 docker-compose.yml 到安装目录
$deployDir = Join-Path $env:LOCALAPPDATA "oc-db9"
if (-not (Test-Path $deployDir)) {
    New-Item -ItemType Directory -Force -Path $deployDir | Out-Null
    Copy-Item (Join-Path $repoDir "*") $deployDir -Recurse
}

Push-Location $deployDir
try {
    docker-compose up -d
    if ($LASTEXITCODE -eq 0) {
        Write-Host "服务启动成功！" -ForegroundColor Green
    } else {
        Write-Host "服务启动失败，请检查日志: docker-compose logs" -ForegroundColor Red
    }
}
finally {
    Pop-Location
}

# 清理临时文件
Remove-Item -Recurse -Force $repoDir

Write-Host ""
Write-Host "=== 安装完成！ ===" -ForegroundColor Green
Write-Host ""
Write-Host "使用方法:" -ForegroundColor Cyan
Write-Host "  oc-db9 db create myapp        # 创建数据库"
Write-Host "  oc-db9 fs cp file.csv :/data  # 上传文件"
Write-Host "  oc-db9 db sql myapp -q '...'  # 执行 SQL"
Write-Host ""
Write-Host "查看文档: https://github.com/yihui504/alternative_db9/blob/main/docs/skill.md" -ForegroundColor Gray
