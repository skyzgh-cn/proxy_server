#!/bin/bash

# Go代理服务器自动部署脚本
# 使用方法: ./run.sh

set -e  # 遇到错误立即退出

# 颜色输出函数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检测系统架构
detect_architecture() {
    local arch=$(uname -m)
    case $arch in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        armv7l|armv6l)
            echo "arm64"  # 使用arm64版本作为fallback
            ;;
        *)
            log_error "不支持的系统架构: $arch"
            log_info "支持的架构: x86_64/amd64, aarch64/arm64"
            exit 1
            ;;
    esac
}

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    log_error "请使用root权限运行此脚本"
    log_info "使用方法: sudo ./deploy_proxy_linux.sh"
    exit 1
fi

# 配置变量
SERVICE_NAME="proxy-server"
WORK_DIR="/opt/proxy-server"
CONFIG_NAME="proxy_config.json"

# 检测系统架构并选择对应的二进制文件
SYSTEM_ARCH=$(detect_architecture)
BINARY_NAME="proxy_server_${SYSTEM_ARCH}"
TARGET_BINARY_NAME="proxy_server"

log_info "开始部署Go代理服务器..."
log_info "检测到系统架构: $(uname -m) -> 使用 $BINARY_NAME"

# 1. 检查必要文件是否存在
log_info "检查必要文件..."
if [ ! -f "$BINARY_NAME" ]; then
    log_error "找不到对应架构的二进制文件: $BINARY_NAME"
    log_info "当前目录文件:"
    ls -la proxy_server_* 2>/dev/null || echo "没有找到任何proxy_server_*文件"
    exit 1
fi

if [ ! -f "$CONFIG_NAME" ]; then
    log_error "找不到配置文件: $CONFIG_NAME"
    exit 1
fi

# 2. 清理不需要的二进制文件
log_info "清理不需要的二进制文件..."
for binary_file in proxy_server_amd64 proxy_server_arm64; do
    if [ "$binary_file" != "$BINARY_NAME" ] && [ -f "$binary_file" ]; then
        log_info "删除不需要的文件: $binary_file"
        rm -f "$binary_file"
    fi
done

# 3. 创建工作目录
log_info "创建工作目录..."
mkdir -p "$WORK_DIR"

# 4. 停止现有服务（如果存在）
log_info "停止现有服务..."
if systemctl is-active --quiet "$SERVICE_NAME"; then
    systemctl stop "$SERVICE_NAME"
    log_info "已停止现有服务"
fi

# 5. 移动文件到工作目录并重命名
log_info "移动文件到工作目录..."
mv "$BINARY_NAME" "$WORK_DIR/$TARGET_BINARY_NAME"
mv "$CONFIG_NAME" "$WORK_DIR/"
log_info "已将 $BINARY_NAME 重命名为 $WORK_DIR/$TARGET_BINARY_NAME"

# 6. 设置可执行权限
log_info "设置可执行权限..."
chmod +x "$WORK_DIR/$TARGET_BINARY_NAME"

# 7. 创建systemd服务文件
log_info "创建systemd服务文件..."
cat > "/etc/systemd/system/$SERVICE_NAME.service" << EOF
[Unit]
Description=Go Proxy Server
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=$WORK_DIR
ExecStart=$WORK_DIR/$TARGET_BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

[Install]
WantedBy=multi-user.target
EOF

# 8. 重新加载systemd配置
log_info "重新加载systemd配置..."
systemctl daemon-reload

# 9. 启用并启动服务
log_info "启用并启动服务..."
systemctl enable "$SERVICE_NAME"
systemctl start "$SERVICE_NAME"

# 10. 检查服务状态
log_info "检查服务状态..."
sleep 2
if systemctl is-active --quiet "$SERVICE_NAME"; then
    log_info "✅ 代理服务器部署成功！"
    log_info "系统架构: $(uname -m)"
    log_info "使用的二进制文件: $BINARY_NAME -> $TARGET_BINARY_NAME"
    log_info "服务状态: $(systemctl is-active $SERVICE_NAME)"
    log_info "服务已启用开机自启动"
else
    log_error "❌ 服务启动失败"
    log_error "请检查日志: journalctl -u $SERVICE_NAME -f"
    exit 1
fi

# 11. 显示服务信息
log_info "=== 服务信息 ==="
echo "服务名称: $SERVICE_NAME"
echo "系统架构: $(uname -m)"
echo "二进制文件: $TARGET_BINARY_NAME (来源: $BINARY_NAME)"
echo "工作目录: $WORK_DIR"
echo "运行用户: root"
echo "配置文件: $WORK_DIR/$CONFIG_NAME"
echo ""
log_info "=== 常用命令 ==="
echo "查看状态: systemctl status $SERVICE_NAME"
echo "查看日志: journalctl -u $SERVICE_NAME -f"
echo "重启服务: systemctl restart $SERVICE_NAME"
echo "停止服务: systemctl stop $SERVICE_NAME"
echo "禁用服务: systemctl disable $SERVICE_NAME"
echo ""
log_info "=== 配置修改 ==="
echo "修改配置: nano $WORK_DIR/$CONFIG_NAME"
echo "重启生效: systemctl restart $SERVICE_NAME"
echo ""
log_info "部署完成！代理服务器正在运行中..."