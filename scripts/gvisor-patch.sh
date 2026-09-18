#!/usr/bin/env bash
#
# gvisor 本地补丁管理脚本
#
# 用途：
#   记录 / 应用 / 校验针对 venendered gvisor (third_party/gvisor) 的本地定制补丁。
#   当前补丁用于记录 OHOS getsockopt 修复；升级 gvisor 时可用 verify 检测冲突。
#
# 背景说明：
#   - third_party/gvisor 为本仓库手工 vendored 的 gvisor 源码拷贝（含 pkg/ 等）。
#     它需被 git 跟踪（提交进仓库）后，save 子命令的 `git diff` 才有实际内容；
#     当前阶段该目录为未跟踪状态，首次 save 前请先将其 git add/提交。
#   - base 默认取 MIHOMO_GVISOR_BASE，即该 gvisor 副本在被用于定制前的基线
#     commit/tag（与 go.mod 中 github.com/metacubex/gvisor 的版本对应）。
#
# 用法：
#   ./scripts/gvisor-patch.sh save [base-ref]   生成/覆盖 third_party/gvisor.patch
#   ./scripts/gvisor-patch.sh apply             应用补丁（已应用则提示，不报错）
#   ./scripts/gvisor-patch.sh verify            校验补丁是否可干净应用

set -euo pipefail

# 上游基线占位默认值：gvisor 被 vendored 到的版本参照。
#   取自 go.mod 中 `github.com/metacubex/gvisor v0.0.0-20260810011720-3cc44cf9ac22`。
#   （请按实际 vendoring 起点替换；可用环境变量或 save 的第二个参数覆盖。）
MIHOMO_GVISOR_BASE="${MIHOMO_GVISOR_BASE:-v0.0.0-20260810011720-3cc44cf9ac22}"
GVISOR_PATCH="third_party/gvisor.patch"

usage() {
    echo "usage: $0 {save [base-ref]|apply|verify}" >&2
    exit 2
}

[ $# -ge 1 ] || usage

case "$1" in
    save)
        base="${2:-${MIHOMO_GVISOR_BASE}}"
        git diff "${base}" -- third_party/gvisor > "${GVISOR_PATCH}"
        echo "saved gvisor patch (base=${base}) -> ${GVISOR_PATCH}"
        ;;
    apply)
        if git apply --check "${GVISOR_PATCH}" 2>/dev/null; then
            git apply "${GVISOR_PATCH}"
            echo "gvisor patch applied."
        else
            echo "gvisor patch already applied (or not applicable)." >&2
        fi
        ;;
    verify)
        if git apply --check "${GVISOR_PATCH}"; then
            echo "OK: gvisor patch applies cleanly."
        else
            echo "FAIL: gvisor patch does NOT apply cleanly (conflict after upgrade?)." >&2
            exit 1
        fi
        ;;
    *)
        usage
        ;;
esac