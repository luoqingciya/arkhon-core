<h1 align="center">
  <img src="Meta.png" alt="Meta Kernel" width="200">
  <br>Meta Kernel<br>
</h1>

<h3 align="center">Teyvat-Arkhon 定制内核（上游 MetaCubeX/mihomo fork）</h3>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/MetaCubeX/mihomo">
    <img src="https://goreportcard.com/badge/github.com/MetaCubeX/mihomo?style=flat-square">
  </a>
  <img src="https://img.shields.io/github/go-mod/go-version/MetaCubeX/mihomo/Alpha?style=flat-square">
  <a href="https://github.com/MetaCubeX/mihomo/releases">
    <img src="https://img.shields.io/github/release/MetaCubeX/mihomo/all.svg?style=flat-square">
  </a>
  <a href="https://github.com/MetaCubeX/mihomo">
    <img src="https://img.shields.io/badge/release-Meta-00b4f0?style=flat-square">
  </a>
</p>

## 定制说明（第一梯队）

本仓库是 [Teyvat-Arkhon](https://github.com/luoqingciya/Teyvat-Arkhon) 的定制内核，基于上游 MetaCubeX/mihomo。在不破坏上游配置格式与 API 兼容性的前提下，做了以下增量定制（均在 `feat/custom-kernel` → `main` 分支）：

- **只读 REST 扩展**（纯新增端点，不修改上游行为）
  - `GET /usage`：按节点聚合的流量统计（当前活跃连接），伴随全局 `upTotal/downTotal`
  - `GET /delay/latest`：全部节点最近一次延迟测试的快照（读取缓存，不触发测速）
- **错误日志可读化**
  - hysteria2 握手/建连失败按原因分类提示：TLS 握手（证书/ALPN/skip-cert-verify）、认证（password）、obfs（salamander 参数不匹配）、超时（服务器不可达或参数不匹配导致静默断开）
  - UDP 会话建立失败明确提示"服务器可能禁用了 UDP"
- **默认配置调优**（内部兜底默认值，不改任何配置字段）
  - hysteria2 默认 `hop-interval` 由上游 30s 调优为 120s，减少端口切换与延迟抖动
  - 未配置 QUIC 流控窗口时注入推荐值（stream 8MB / connection 16MB），提升大带宽下行吞吐
  - DNS 缓存默认容量由 4096 调优为 8192

> 这些定制点不影响上游配置格式与 API 兼容性，仅作为 Teyvat-Arkhon 应用侧排障与性能体验优化的一环。

## 特性

- 本地 HTTP/HTTPS/SOCKS 服务，支持认证
- 支持 VMess、VLESS、Shadowsocks、Trojan、Snell、TUIC、Hysteria 等协议
- 内置 DNS 服务器，旨在最大限度降低 DNS 污染攻击影响，支持 DoH/DoT 上游与 Fake IP
- 基于域名、GEOIP、IPCIDR 或进程的规则，将报文转发到不同节点
- 远程分组允许用户实现更强大的规则，支持基于延迟的自动回退、负载均衡或自动选优
- 远程 Provider 允许用户远程获取节点列表，而无需在配置中硬编码
- Netfilter TCP 重定向：配合 `iptables` 将 mihomo 部署为上网网关
- 完整的 HTTP RESTful API 控制器

## 面板

本项目支持的一等公民 Web 面板参见 [metacubexd](https://github.com/MetaCubeX/metacubexd)。

## 配置示例

配置示例见 [/docs/config.yaml](https://github.com/MetaCubeX/mihomo/blob/Alpha/docs/config.yaml)。

## 文档

使用文档见 [mihomo Docs](https://wiki.metacubex.one/)。

## 开发

依赖：
[Go 1.20 及以上](https://go.dev/dl/)

构建 mihomo：

```shell
git clone https://github.com/MetaCubeX/mihomo.git
cd mihomo && go mod download
go build
```

如果无法直连 GitHub，可设置 Go 代理：

```shell
go env -w GOPROXY=https://goproxy.io,direct
```

使用 gvisor tun 栈构建：

```shell
go build -tags with_gvisor
```

### IPTABLES 配置

适用于支持 `iptables` 的 Linux 系统

```yaml
# 启用 TPROXY 监听器
tproxy-port: 9898

iptables:
  enable: true # 默认 false
  inbound-interface: eth0 # 检测入站接口，默认为 'lo'
```

## 调试

调试 API 的使用说明参见 [wiki](https://wiki.metacubex.one/api/#debug)。

## 致谢

- [Dreamacro/clash](https://github.com/Dreamacro/clash)
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [riobard/go-shadowsocks2](https://github.com/riobard/go-shadowsocks2)
- [v2ray/v2ray-core](https://github.com/v2ray/v2ray-core)
- [WireGuard/wireguard-go](https://github.com/WireGuard/wireguard-go)
- [yaling888/clash-plus-pro](https://github.com/yaling888/clash)

## 许可

本项目以 GPL-3.0 许可协议发布。

**此外，任何与 `MetaCubeX` 无关的下游项目，其名称不得包含 `mihomo` 字样。**