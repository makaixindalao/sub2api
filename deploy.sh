#!/bin/bash
#
# Sub2API 一键部署脚本
# 从本地 macOS 交叉编译并部署到远程 Linux 服务器
#
# 用法:
#   ./deploy.sh              # 编译 + 部署 + 重启
#   ./deploy.sh --init       # 首次部署（含 systemd 配置）
#   ./deploy.sh --build-only # 仅编译不部署
#   ./deploy.sh --skip-build # 跳过编译，仅部署已有二进制
#
# 作者: mkx
# 日期: 2025-02-11

set -euo pipefail

# ============================================================
# 配置变量
# ============================================================
REMOTE_HOST="216.40.86.130"
REMOTE_PORT="22"
REMOTE_USER="root"
INSTALL_DIR="/opt/sub2api"
SERVICE_NAME="sub2api"
SERVICE_USER="sub2api"

# 项目路径（脚本所在目录）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$SCRIPT_DIR/backend"
FRONTEND_DIR="$SCRIPT_DIR/frontend"

# 构建产物
BINARY_NAME="sub2api"

# SSH/SCP 公共参数
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -p $REMOTE_PORT"
SSH_CMD="ssh $SSH_OPTS ${REMOTE_USER}@${REMOTE_HOST}"
SCP_CMD="scp -P $REMOTE_PORT -o StrictHostKeyChecking=no"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ============================================================
# 检测远程服务器架构
# ============================================================
detect_remote_arch() {
    info "检测远程服务器架构..."
    local remote_arch
    remote_arch=$($SSH_CMD "uname -m") || error "无法连接远程服务器"

    case "$remote_arch" in
        x86_64)       GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        *) error "不支持的架构: $remote_arch" ;;
    esac

    success "远程架构: $remote_arch -> GOARCH=$GOARCH"
}

# ============================================================
# 构建前端
# ============================================================
build_frontend() {
    info "构建前端..."
    pnpm --dir "$FRONTEND_DIR" run build || error "前端构建失败"
    success "前端构建完成"
}

# ============================================================
# 交叉编译后端
# ============================================================
build_backend() {
    info "交叉编译后端 (linux/$GOARCH, embed 模式)..."
    cd "$BACKEND_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -tags embed -o "$BINARY_NAME" ./cmd/server \
        || error "后端编译失败"
    cd "$SCRIPT_DIR"
    success "后端编译完成: $BACKEND_DIR/$BINARY_NAME"
}

# ============================================================
# 上传阶段（服务不停，先传到临时文件）
# ============================================================
upload() {
    local binary="$BACKEND_DIR/$BINARY_NAME"
    [ -f "$binary" ] || error "二进制文件不存在: $binary"

    info "上传二进制到 $REMOTE_HOST (临时文件)..."
    $SCP_CMD "$binary" "${REMOTE_USER}@${REMOTE_HOST}:${INSTALL_DIR}/${BINARY_NAME}.new" \
        || error "上传失败"
    # mkx: 等待远程 sftp-server 释放文件描述符，避免 ETXTBSY (2026-02-11)
    $SSH_CMD "chmod +x $INSTALL_DIR/$BINARY_NAME.new && \
        chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/$BINARY_NAME.new && \
        sync && \
        for i in 1 2 3 4 5; do fuser $INSTALL_DIR/$BINARY_NAME.new >/dev/null 2>&1 || break; sleep 1; done"
    success "上传完成"

    # migrations 也在服务运行期间同步
    local migrations_dir="$BACKEND_DIR/migrations"
    if [ -d "$migrations_dir" ]; then
        info "同步 migrations..."
        $SSH_CMD "mkdir -p $INSTALL_DIR/migrations"
        $SCP_CMD -r "$migrations_dir/"* "${REMOTE_USER}@${REMOTE_HOST}:${INSTALL_DIR}/migrations/" \
            || warn "migrations 同步失败（可能无新文件）"
        success "migrations 同步完成"
    fi
}

# ============================================================
# 原子替换 + 重启（停机时间最短）
# ============================================================
swap_and_restart() {
    info "停服务 → 原子替换 → 拉起服务..."
    # mkx: 停服务后等待进程完全退出，再做原子替换 (2026-02-11)
    $SSH_CMD "systemctl stop $SERVICE_NAME && \
        while fuser $INSTALL_DIR/$BINARY_NAME >/dev/null 2>&1; do sleep 0.5; done && \
        mv $INSTALL_DIR/$BINARY_NAME $INSTALL_DIR/${BINARY_NAME}.backup && \
        mv $INSTALL_DIR/${BINARY_NAME}.new $INSTALL_DIR/$BINARY_NAME && \
        systemctl start $SERVICE_NAME" \
        || error "替换或重启失败"

    info "等待服务启动..."
    sleep 3

    local svc_status=""
    svc_status=$($SSH_CMD "systemctl is-active $SERVICE_NAME" 2>/dev/null) || true
    if [ "$svc_status" = "active" ]; then
        success "服务运行正常 (active)"
    else
        warn "服务状态: ${svc_status:-unknown}，尝试回滚..."
        $SSH_CMD "mv $INSTALL_DIR/${BINARY_NAME}.backup $INSTALL_DIR/$BINARY_NAME && systemctl start $SERVICE_NAME" 2>/dev/null || true
        $SSH_CMD "journalctl -u $SERVICE_NAME -n 20 --no-pager" 2>/dev/null || true
        error "服务启动异常，已回滚到旧版本"
    fi
}

# ============================================================
# 首次部署初始化
# ============================================================
init_server() {
    info "首次部署初始化..."

    $SSH_CMD bash <<'INIT_EOF'
set -e

# 创建用户
if ! id sub2api &>/dev/null; then
    useradd -r -s /bin/sh -d /opt/sub2api sub2api
    echo "用户 sub2api 已创建"
else
    echo "用户 sub2api 已存在"
fi

# 创建目录
mkdir -p /opt/sub2api/migrations /opt/sub2api/data
chown -R sub2api:sub2api /opt/sub2api

# 安装 systemd 服务
cat > /etc/systemd/system/sub2api.service << 'SERVICE_EOF'
[Unit]
Description=Sub2API - AI API Gateway Platform
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=sub2api
Group=sub2api
WorkingDirectory=/opt/sub2api
ExecStart=/opt/sub2api/sub2api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sub2api

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/sub2api

Environment=GIN_MODE=release
Environment=SERVER_HOST=0.0.0.0
Environment=SERVER_PORT=8080

[Install]
WantedBy=multi-user.target
SERVICE_EOF

systemctl daemon-reload
systemctl enable sub2api
echo "systemd 服务已安装并启用开机自启"
INIT_EOF

    success "服务器初始化完成"
}

# ============================================================
# 用法提示
# ============================================================
usage() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  (无参数)       编译 + 部署 + 重启"
    echo "  --init         首次部署（含 systemd 配置）"
    echo "  --build-only   仅编译不部署"
    echo "  --skip-build   跳过编译，仅部署已有二进制"
    echo "  -h, --help     显示帮助"
}

# ============================================================
# 主流程
# ============================================================
main() {
    local do_build=true
    local do_deploy=true
    local do_init=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --init)       do_init=true; shift ;;
            --build-only) do_deploy=false; shift ;;
            --skip-build) do_build=false; shift ;;
            -h|--help)    usage; exit 0 ;;
            *) error "未知参数: $1" ;;
        esac
    done

    echo ""
    echo "=========================================="
    echo "  Sub2API 部署脚本"
    echo "=========================================="
    echo ""

    # 需要连接远程时，先检测架构
    if $do_build || $do_deploy; then
        detect_remote_arch
    fi

    # 首次初始化
    if $do_init; then
        init_server
    fi

    # 构建
    if $do_build; then
        build_frontend
        build_backend
    fi

    # 部署
    if $do_deploy; then
        upload
        swap_and_restart
    fi

    echo ""
    success "全部完成!"
}

main "$@"
