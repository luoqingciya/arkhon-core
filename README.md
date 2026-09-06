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

本仓库是 [Teyvat-Arkhon](https://github.com/luoqingciya/Teyvat-Arkhon) 的定制内核，基于上游 MetaCubeX/mihomo。在不破坏上游 API 的前提下，做了以下增量定制（均在 `feat/custom-kernel` → `main` 分支）：

- **只读 REST 扩展**（纯新增端点，不修改上游行为）
  - `GET /usage`：按节点聚合的流量统计（当前活跃连接），随全局 `upTotal/downTotal`
  - `GET /delay/latest`：全部节点最近一次延迟测试的快照（读取缓存，不触发测速）
- **错误日志可读化**
  - hysteria2 握手/建连失败按原因分类提示：TLS 握手（证书/ALPN/skip-cert-verify）、认证（password）、obfs（salamander 参数不匹配）、超时（服务器不可达或参数不匹配导致静默断开）
  - UDP 会话建立失败明确提示"服务器可能禁用了 UDP"
- **默认配置调优**（内部兜底默认值，不改任何配置字段）
  - hysteria2 默认 `hop-interval` 由上游 30s 调优为 120s，减少端口切换与延迟抖动
  - 未配置 QUIC 流控窗口时注入推荐值（stream 8MB / connection 16MB），提升大带宽下行吞吐
  - DNS 缓存默认容量由 4096 调优为 8192

> 这些定制点不影响上游配置格式与 API 兼容性，仅作为 Teyvat-Arkhon 应用侧排障与性能体验优化的一环。

## Features

- Local HTTP/HTTPS/SOCKS server with authentication support
- VMess, VLESS, Shadowsocks, Trojan, Snell, TUIC, Hysteria protocol support
- Built-in DNS server that aims to minimize DNS pollution attack impact, supports DoH/DoT upstream and fake IP.
- Rules based off domains, GEOIP, IPCIDR or Process to forward packets to different nodes
- Remote groups allow users to implement powerful rules. Supports automatic fallback, load balancing or auto select node
  based off latency
- Remote providers, allowing users to get node lists remotely instead of hard-coding in config
- Netfilter TCP redirecting. Deploy Mihomo on your Internet gateway with `iptables`.
- Comprehensive HTTP RESTful API controller

## Dashboard

A web dashboard with first-class support for this project has been created; it can be checked out at [metacubexd](https://github.com/MetaCubeX/metacubexd).

## Configration example

Configuration example is located at [/docs/config.yaml](https://github.com/MetaCubeX/mihomo/blob/Alpha/docs/config.yaml).

## Docs

Documentation can be found in [mihomo Docs](https://wiki.metacubex.one/).

## For development

Requirements:
[Go 1.20 or newer](https://go.dev/dl/)

Build mihomo:

```shell
git clone https://github.com/MetaCubeX/mihomo.git
cd mihomo && go mod download
go build
```

Set go proxy if a connection to GitHub is not possible:

```shell
go env -w GOPROXY=https://goproxy.io,direct
```

Build with gvisor tun stack:

```shell
go build -tags with_gvisor
```

### IPTABLES configuration

Work on Linux OS which supported `iptables`

```yaml
# Enable the TPROXY listener
tproxy-port: 9898

iptables:
  enable: true # default is false
  inbound-interface: eth0 # detect the inbound interface, default is 'lo'
```

## Debugging

Check [wiki](https://wiki.metacubex.one/api/#debug) to get an instruction on using debug
API.

## Credits

- [Dreamacro/clash](https://github.com/Dreamacro/clash)
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [riobard/go-shadowsocks2](https://github.com/riobard/go-shadowsocks2)
- [v2ray/v2ray-core](https://github.com/v2ray/v2ray-core)
- [WireGuard/wireguard-go](https://github.com/WireGuard/wireguard-go)
- [yaling888/clash-plus-pro](https://github.com/yaling888/clash)

## License

This software is released under the GPL-3.0 license.

**In addition, any downstream projects not affiliated with `MetaCubeX` shall not contain the word `mihomo` in their names.**