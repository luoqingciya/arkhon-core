# arkhon-core

**Teyvat-Arkhon 独立代理内核**：在 Clash / Mihomo 配置与 REST API 事实标准之上独立开发维护，作为 [Teyvat-Arkhon](https://github.com/luoqingciya/Teyvat-Arkhon) 桌面客户端的代理内核使用。

## 策略

- 配置格式与 REST API 保持对 Clash/Mihomo 标准的**向后兼容**（协议语义不变，用户配置可直接复用）
- 定制方向聚焦：**可排障性**、**默认开箱体验**与**平台适配**（OHOS/OpenHarmony 等），不改变协议语义
- 代码中保留与上游分叉的标注，便于独立演进与按需合流

## 定制内容

### 1. 只读 REST 扩展

纯新增端点，不修改上游行为：

| 端点 | 说明 |
| --- | --- |
| `GET /usage` | 按节点聚合的流量统计（当前活跃连接），伴随全局 `upTotal` / `downTotal` |
| `GET /delay/latest` | 全部节点最近一次延迟测试快照（读取缓存，不触发测速） |

### 2. 错误日志可读化

hysteria2 握手 / 建连失败按原因分类提示：

- **TLS 握手失败**：证书校验 / ALPN 不匹配 / 证书过期，并提示可尝试 `skip-cert-verify`
- **认证失败**：提示检查 hysteria2 `password`
- **obfs 不匹配**：提示 `obfs` / `obfs-password` 须与服务器一致
- **超时**：提示服务器不可达，或 TLS/obfs 参数不匹配导致握手被静默断开

另：UDP 会话建立失败会明确提示"服务器可能禁用了 UDP"。

### 3. 默认配置调优

内部兜底默认值，不改任何配置字段：

- hysteria2 默认 `hop-interval`：30s → **120s**（减少端口切换与延迟抖动）
- 未配置 QUIC 流控窗口时注入推荐值：stream **8MB** / connection **16MB**（提升大带宽下行吞吐）
- DNS 缓存默认容量：4096 → **8192**

## 构建

依赖：[Go 1.20 及以上](https://go.dev/dl/)（内核主程序）；`clashlib/`（OHOS c-shared 导出层）需 Go 1.24，OHOS 交叉编译使用 OpenHarmony SIG 官方 Go fork [ohos_golang_go](https://gitcode.com/openharmony-sig/ohos_golang_go)（`GOOS=openharmony`，内置 general dynamic TLS 支持，解决 musl `initial-exec TLS` 导致 `.so` 无法 `dlopen`）。

```shell
git clone https://github.com/luoqingciya/arkhon-core.git
cd arkhon-core && go mod download
go build -o arkhon
```

使用 gvisor tun 栈构建：

```shell
go build -o arkhon -tags with_gvisor
```

无法直连 GitHub 时设置 Go 代理：

```shell
go env -w GOPROXY=https://goproxy.io,direct
```

## Teyvat 定制特性

- **OHOS/OpenHarmony 适配**：TUN 隧道 fd 探测改用 `getsockopt(SO_TYPE)`（沙箱限制 `fstat` 的 `EPERM`），gvisor 补丁固化于 `third_party/gvisor`，可用 `scripts/gvisor-patch.sh save/apply/verify` 维护，升级上游时检测冲突。
- **TUN 自愈与可观测**：启动打印结构化日志 `[TUN] attach stack/goos/fd/fd_socket`；`gvisor`/`mixed` 栈初始化失败自动降级 `system` 栈并告警，避免网络黑洞。
- **调试开关**：设置环境变量 `TEYVAT_ARKHON_PPROF=127.0.0.1:6060` 可启用 `net/http/pprof`，断流定位无需重新构建。
- **内存优化**：配置顶层加 `geodata-mode: memconservative` 可懒加载 geo 数据；规则集支持 `.mrs` 二进制（`behavior: mrs`）。

## 发布

推送 `v*` 形 tag 即触发 [.github/workflows/release-custom.yml](.github/workflows/release-custom.yml)，自动交叉编译并发布 GitHub Release：

- 平台/架构：windows / linux / darwin × amd64 / arm64
- 资产命名遵循 Teyvat-Arkhon 应用下载脚本约定（windows 为 `.zip` 内含 `arkhon-windows-<arch>.exe`，其余为 `.gz`）
- release 附带 `checksums.txt`，应用侧据此做完整性校验
- **OpenHarmony/OHOS c-shared（软失败）**：`ohos` job 从 `ohos_golang_go` fork 就地自举工具链，交叉编译 `clashlib/`（c-shared 导出层 `arkhon_core_version/start/stop/attach`）产出 `arkhon-ohos-arm64-<tag>.so`，供鸿蒙端子项目 [Arkhon](https://github.com/luoqingciya/arkhon) 的 NAPI `dlopen("libclash.so")` 使用。该 job 为 `continue-on-error` 软失败：OHOS SDK 原生工具链默认内置 `cidownload.openharmony.cn` 的 `ohos-sdk-full_ohos`（约 3GB，可用仓库变量 `OHOS_NDK_URL` / `OHOS_NDK_NATIVE_ZIP` 覆盖），但已用 `actions/cache` 缓存解压后的 `native/` 目录——**仅首次下载整包，后续 release 直接命中缓存零下载**（换 SDK 版本时 bump `NDK_CACHE_KEY` 即可重拉）。工具链自举失败、NDK 下载/解压失败时仅跳过 `.so`，不影响桌面三平台资产发布。

## 文档

配置与 API 遵循 Clash / Mihomo 公开标准；标准参考：[mihomo Wiki](https://wiki.metacubex.one/)。

## 起源

本项目自 [MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo) 分叉演进（GPL-3.0），现已作为独立内核维护，不再以上游为依赖。

## 许可

[GPL-3.0](LICENSE)。**任何与 `MetaCubeX` 无关的下游项目，其名称不得包含 `mihomo` 字样。**