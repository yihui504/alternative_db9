#!/bin/bash

# OpenClaw-db9 一键安装脚本
# 支持 macOS 和 Linux (x86_64, arm64)

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 默认安装目录
INSTALL_DIR="${OC_DB9_INSTALL_DIR:-/usr/local/bin}"
REPO="yihui504/alternative_db9"

# 检测操作系统和架构
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}不支持的架构: $ARCH${NC}"
            exit 1
            ;;
    esac
    
    case "$OS" in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            echo -e "${RED}不支持的操作系统: $OS${NC}"
            exit 1
            ;;
    esac
    
    echo "检测到平台: $PLATFORM/$ARCH"
}

# 检查依赖
check_dependencies() {
    echo -e "${YELLOW}检查依赖...${NC}"
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}错误: 需要安装 Docker${NC}"
        echo "请访问 https://docs.docker.com/get-docker/ 安装"
        exit 1
    fi
    
    # 检查 Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}错误: 需要安装 Docker Compose${NC}"
        exit 1
    fi
    
    # 检查 Go (用于构建 CLI)
    if ! command -v go &> /dev/null; then
        echo -e "${YELLOW}警告: 未检测到 Go，将尝试下载预编译二进制文件${NC}"
        USE_PREBUILT=true
    else
        USE_PREBUILT=false
    fi
    
    echo -e "${GREEN}依赖检查通过${NC}"
}

# 下载最新版本
download_latest() {
    echo -e "${YELLOW}下载 OpenClaw-db9...${NC}"
    
    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    # 克隆仓库
    git clone --depth 1 "https://github.com/$REPO.git" oc-db9
    cd oc-db9
    
    echo -e "${GREEN}下载完成${NC}"
}

# 安装到系统
install_binary() {
    echo -e "${YELLOW}安装 oc-db9...${NC}"
    
    # 构建或下载二进制文件
    if [ "$USE_PREBUILT" = true ]; then
        echo -e "${YELLOW}下载预编译二进制文件...${NC}"
        BINARY_URL="https://github.com/$REPO/releases/latest/download/oc-db9-$PLATFORM-$ARCH"
        curl -fsSL "$BINARY_URL" -o oc-db9
        chmod +x oc-db9
    else
        echo -e "${YELLOW}从源码构建...${NC}"
        go build -o oc-db9 ./internal/cmd
    fi
    
    # 移动到安装目录
    if [ -w "$INSTALL_DIR" ]; then
        mv oc-db9 "$INSTALL_DIR/"
    else
        echo -e "${YELLOW}需要管理员权限来安装到 $INSTALL_DIR${NC}"
        sudo mv oc-db9 "$INSTALL_DIR/"
    fi
    
    echo -e "${GREEN}oc-db9 已安装到 $INSTALL_DIR${NC}"
}

# 启动服务
start_services() {
    echo -e "${YELLOW}启动 OpenClaw-db9 服务...${NC}"
    
    # 复制到最终位置
    if [ ! -d "$HOME/.oc-db9" ]; then
        mkdir -p "$HOME/.oc-db9"
        cp -r . "$HOME/.oc-db9/"
    fi
    
    cd "$HOME/.oc-db9"
    
    # 启动 Docker 服务
    docker-compose up -d
    
    # 等待服务就绪
    echo -e "${YELLOW}等待服务启动...${NC}"
    sleep 10
    
    # 检查服务状态
    if docker-compose ps | grep -q "Up"; then
        echo -e "${GREEN}服务启动成功！${NC}"
    else
        echo -e "${RED}服务启动失败，请检查日志: docker-compose logs${NC}"
        exit 1
    fi
}

# 配置环境
setup_environment() {
    echo -e "${YELLOW}配置环境...${NC}"
    
    # 添加到 PATH（如果需要）
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$HOME/.bashrc"
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$HOME/.zshrc" 2>/dev/null || true
        echo -e "${YELLOW}已将 $INSTALL_DIR 添加到 PATH，请重新加载 shell 配置或运行: source ~/.bashrc${NC}"
    fi
    
    # 创建配置目录
    mkdir -p "$HOME/.config/oc-db9"
    
    echo -e "${GREEN}环境配置完成${NC}"
}

# 主函数
main() {
    echo -e "${GREEN}=== OpenClaw-db9 安装程序 ===${NC}"
    echo ""
    
    detect_platform
    check_dependencies
    download_latest
    install_binary
    start_services
    setup_environment
    
    echo ""
    echo -e "${GREEN}=== 安装完成！===${NC}"
    echo ""
    echo "使用方法:"
    echo "  oc-db9 db create myapp        # 创建数据库"
    echo "  oc-db9 fs cp file.csv :/data  # 上传文件"
    echo "  oc-db9 db sql myapp -q '...'  # 执行 SQL"
    echo ""
    echo "查看文档: https://github.com/$REPO/blob/main/docs/skill.md"
    echo ""
    
    # 显示版本
    "$INSTALL_DIR/oc-db9" --version 2>/dev/null || echo "请重新加载 shell 后使用"
}

# 运行主函数
main
