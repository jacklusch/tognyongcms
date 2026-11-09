#!/usr/bin/env bash
# ============================================================
# dulizhan-cms 一键更新脚本
# 用法: bash /opt/dulizhan/update.sh [仓库地址] [分支名]
#   仓库地址: 留空则沿用当前 origin；也可切换为其他 GitHub 仓库
#   分支名:   默认 main
# 示例:
#   bash update.sh https://github.com/jacklusch/tognyongcms.git feature/theme-01-home-sections
#   bash update.sh "" main
# ============================================================
set -euo pipefail

BASE_DIR="/opt/dulizhan"
BRANCH="${2:-main}"
DOMAIN="${DOMAIN:-www.foodsequippro.com}"
LOG="$BASE_DIR/update.log"
MAX_WAIT_SEC=1800

cd "$BASE_DIR"

# 1. 可选：切换仓库地址
if [[ -n "${1:-}" ]]; then
  echo "==> 设置仓库地址: $1"
  git remote set-url origin "$1"
fi

OLD_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")

# 2. 拉取并切换到目标分支
echo "==> 拉取分支 $BRANCH ..."
git fetch --depth 1 origin "$BRANCH" 2>&1 | tail -1
git checkout --detach FETCH_HEAD 2>&1 | tail -1
NEW_COMMIT=$(git rev-parse --short HEAD)
echo "==> 版本切换: $OLD_COMMIT -> $NEW_COMMIT"
if [[ "$OLD_COMMIT" == "$NEW_COMMIT" ]]; then
  echo "==> 与当前版本一致，将复用构建缓存快速重建"
fi

# 3. 防止并发构建
if pgrep -f "docker compose up" >/dev/null 2>&1; then
  echo "!! 检测到已有构建任务在运行，请稍后再试"
  exit 1
fi

# 4. 后台构建（setsid 脱离会话，SSH 断线不影响构建），并轮询等待
echo "==> 构建镜像并重启容器（日志: $LOG）"
setsid nohup sudo docker compose up -d --build > "$LOG" 2>&1 < /dev/null &

WAITED=0
while pgrep -f "docker compose up" >/dev/null 2>&1; do
  sleep 10
  WAITED=$((WAITED + 10))
  if [[ $WAITED -ge $MAX_WAIT_SEC ]]; then
    echo "!! 构建超时（${MAX_WAIT_SEC}s），请检查 $LOG"
    exit 1
  fi
done
echo "==> 构建完成，用时 $((WAITED + 10))s"

# 5. 校验容器状态
sleep 3
if ! sudo docker ps --filter name=dulizhan-cms --filter status=running | grep -q dulizhan-cms; then
  echo "!! 容器未正常运行，最近日志:"
  tail -30 "$LOG"
  exit 1
fi

# 6. 初始化数据（幂等，可重复执行）
echo "==> 执行 seed（幂等）"
sudo docker exec dulizhan-cms ./dulizhan seed 2>&1 | tail -2

# 7. 健康检查
sleep 2
CODE=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 http://127.0.0.1:8080/) || CODE="000"
echo "==> 本机健康检查:  http://127.0.0.1:8080/  -> HTTP $CODE"
EXT=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 --resolve "$DOMAIN:443:127.0.0.1" "https://$DOMAIN/") || EXT="000"
echo "==> 域名健康检查:  https://$DOMAIN/       -> HTTP $EXT"

if [[ "$CODE" == "200" ]]; then
  echo ""
  echo "=========================================="
  echo "  更新成功  $NEW_COMMIT ($BRANCH)"
  echo "  线上地址: https://$DOMAIN/"
  echo "  管理后台: https://$DOMAIN/admin"
  echo "=========================================="
else
  echo "!! 健康检查未通过，最近容器日志:"
  sudo docker logs --tail 20 dulizhan-cms
  exit 1
fi
