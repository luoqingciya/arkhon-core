# mihomo-teyvat

**Teyvat-Arkhon 定制内核**，基于 [MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo) 的 fork。

仅做"不改上游配置与 API 兼容性"的增量定制，作为 [Teyvat-Arkhon](https://github.com/luoqingciya/Teyvat-Arkhon) 桌面客户端的代理内核使用。

## 策略

- 保持与上游 mihomo 同步，配置格式与 REST API 完全兼容
- 定制点聚焦：**可排障性**与**默认开箱体验**，不改变协议语义
- 所有定制均有明确出处标注，便于随上游合流

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

依赖：[Go 1.20 及以上](https://go.dev/dl/)

```shell
git clone https://github.com/luoqingciya/mihomo-teyvat.git
cd mihomo-teyvat && go mod download
go build
```

使用 gvisor tun 栈构建：

```shell
go build -tags with_gvisor
```

无法直连 GitHub 时设置 Go 代理：

```shell
go env -w GOPROXY=https://goproxy.io,direct
```

## 发布

推送 `v*` 形 tag 即触发 [.github/workflows/release-custom.yml](.github/workflows/release-custom.yml)，自动交叉编译并发布 GitHub Release：

- 平台/架构：windows / linux / darwin × amd64 / arm64
- 资产命名遵循 Teyvat-Arkhon 应用下载脚本约定（windows 为 `.zip` 内含 `mihomo-windows-<arch>.exe`，其余为 `.gz`）
- release 附带 `checksums.txt`，应用侧据此做完整性校验

## 文档

上游配置与 API 文档：[mihomo Docs](https://wiki.metacubex.one/)

## 致谢

- 上游内核 [MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo)
- 及其依赖的 [Dreamacro/clash](https://github.com/Dreamacro/clash)、[SagerNet/sing-box](https://github.com/SagerNet/sing-box) 等开源项目

## 许可

[GPL-3.0](LICENSE)。**任何与 `MetaCubeX` 无关的下游项目，其名称不得包含 `mihomo` 字样。**