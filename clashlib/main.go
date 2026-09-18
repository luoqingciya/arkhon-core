// clashlib: 把 mihomo(arkhon-core) 内核封装为可被 NAPI dlopen 的 c-shared .so。
// 供 Gate-D 在手机应用进程内加载真实内核并启动（external-controller 回显 /version 作为存活信号）。
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/metacubex/mihomo/config"
	MC "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	LC "github.com/metacubex/mihomo/listener/config"
	"github.com/metacubex/mihomo/listener/sing_tun"
	"github.com/metacubex/mihomo/tunnel"
	"golang.org/x/sys/unix"
)

// D5e 诊断：把 C 边界实际收到的 cfgStr 与解析计数落盘成文件，供 shell 直接读取，
// 用于定位"真机 proxies 未注册"到底是被截断 还是 解析丢 slice。
const debugFile = "clash-debug.txt"

func writeDebug(homeDir, cfgStr string) {
	if homeDir == "" {
		return
	}
	dbg := "cfgLen=" + strconv.Itoa(len(cfgStr)) + "\nRAWCFG:\n" + cfgStr + "\n"
	if raw, err := config.UnmarshalRawConfig([]byte(cfgStr)); err == nil && raw != nil {
		dbg += fmt.Sprintf("RAWCOUNT Proxy=%d ProxyGroup=%d Rule=%d MixedPort=%d Mode=%s\n",
			len(raw.Proxy), len(raw.ProxyGroup), len(raw.Rule), raw.MixedPort, string(raw.Mode))
		// 继续 ParseRawConfig：确认 proxies 是否在 parse 阶段存活（设备/本地差异点在 _hl）
		if cfg, perr := config.ParseRawConfig(raw); perr == nil && cfg != nil {
			dbg += fmt.Sprintf("PARSEDCFG Proxies=%d Rules=%d", len(cfg.Proxies), len(cfg.Rules))
			for k := range cfg.Proxies {
				dbg += (" key=" + k)
			}
			dbg += "\n"
		} else if perr != nil {
			dbg += fmt.Sprintf("PARSEDCFG_ERR %v\n", perr)
		}
	}
	_ = os.WriteFile(homeDir+"/"+debugFile, []byte(dbg), 0o644)
}

// c 字符串返回约定：成功返回 NULL，失败返回需 C.free 的错误字符串（堆上）。
func cmsg(format string, a ...any) *C.char {
	return C.CString(fmt.Sprintf(format, a...))
}

//export arkhon_core_version
func arkhon_core_version() *C.char {
	return cmsg("%s build %s", MC.Version, MC.BuildTime)
}

// 内核运行态标记（供 attach 先校验，避免对未启动内核挂 TUN）。
var kernelRunning atomic.Bool

//export arkhon_core_start
// homeDir 可为空（使用默认 C.Path.HomeDir()）；configJSON 为 mihomo YAML/JSON 配置文本。
func arkhon_core_start(home *C.char, configJSON *C.char, extController *C.char) *C.char {
	homeDir := C.GoString(home)
	cfgStr := C.GoString(configJSON)
	ctrl := C.GoString(extController)

	// D5e 诊断：C 边界落盘实际收到的 cfgStr + 解析计数
	writeDebug(homeDir, cfgStr)
	lastHome = homeDir // 供 attach 阶段 fd 试读/诊断文件落盘使用

	if homeDir != "" {
		MC.SetHomeDir(homeDir)
	}

	// 注意：传入 configBytes 时不调用 config.Init（对应 mihomo main.go 的 configString 分支），
	// 避免在沙箱内创建 config.yaml 触发权限问题；hub.Parse 以配置文本为准。
	if ctrl == "" {
		ctrl = "127.0.0.1:9097"
	}
	options := []hub.Option{hub.WithExternalController(ctrl)}

	if err := hub.Parse([]byte(cfgStr), options...); err != nil {
		return cmsg("hub.Parse: %v", err)
	}

	// D5e 运行时 ground-truth：hub.Parse(→ApplyConfig) 完成后，直接读 tunnel 全局 store，
	// 确认 cfg.Proxies(9) 到底有没有真的注册进 tunnel，区分「解析丢」还是「ApplyConfig 丢/被重置」。
	if homeDir != "" {
		ps := tunnel.Proxies()
		rs := tunnel.Rules()
		var sb strings.Builder
		sb.WriteString("\nAFTERAPPLY tunnel.Proxies=")
		sb.WriteString(strconv.Itoa(len(ps)))
		sb.WriteString(" keys=")
		for k := range ps {
			sb.WriteString(" ")
			sb.WriteString(k)
		}
		sb.WriteString(fmt.Sprintf("\ntunnel.Rules=%d\n", len(rs)))
		_ = os.WriteFile(homeDir+"/tunnel-store.txt", []byte(sb.String()), 0o644)
	}

	// 成功：返回空串而非 NULL？调用端用 dlsym 后若返回 NULL 视为成功。
	kernelRunning.Store(true)
	return nil
}

//export arkhon_core_stop
func arkhon_core_stop() {
	if kernelRunning.Swap(false) {
		executor.Shutdown()
	}
}

//export arkhon_core_attach
// 将内核的 TUN listener 挂到 HarmonyOS VPN 隧道 fd 上（conn.create() 返回）。
// 系统经虚拟网卡下发的 L3 IP 包会从这个 fd 被读入 sing-tun 用户态栈，再进入内核代理链。
// 前置：必须在 arkhon_core_start 成功（内核已 Parse 运行）后调用；fd 属扩展进程私有。
// 返回 NULL 成功，失败返回需 C.free 的错误串。
func arkhon_core_attach(fd int) *C.char {
	if fd <= 0 {
		return cmsg("invalid tun fd: %d", fd)
	}
	if !kernelRunning.Load() {
		return cmsg("kernel not running, call arkhon_core_start first")
	}

	// D5e 诊断：后台试读首帧（不阻塞 attach），与 tunLoop 竞争消费一帧用于确认包格式。
	// 同步 probe 等 2s 时用户还没点「隧道出网」，必然 no packet；改后台等真流量来。
	go probeFdFrame(fd)

	// D5e(第二轮)：弃 system 栈，改回 gvisor 纯用户态栈。
	// system 栈的 TCP 机制依赖「把 SYN 改写成本地 loopback → 宿主本机 TCP 栈完成握手」，
	// 即 start() 在宿主进程 bind 127.0.0.1:随机端口 listener，SYN 会被改写后写回 fd，
	// 期望系统把写回的包注入本机栈完成本地交付 —— 但 OHOS VPN 隧道 fd 的写回路径是
	// 「发往 VPN 网络转发」而非注入本机协议栈 → 改写后的 SYN 有去无回（真机 TCP 表
	// 显示 SYN 源 10.21.0.2 目标公网 IP 状态 SYN_SENT 重传 N 次，loopback 到宿主 listener
	// 从未发生）。gvisor 栈在用户态独立完成 TCP 握手，SYN-ACK 直接写回 fd，不依赖宿主栈。
	// 此前弃 gvisor 因 fdbased 创建时 unix.Fstat(fd) 被 OHOS 沙箱 EPERM → attach 失败；
	// 已在 mihomo-teyvat/third_party/gvisor fdbased.endpoint.go 把 IsSocketFD 改为
	// getsockopt(SO_TYPE) 探测（失败容忍为非 socket，走 readv dispatcher），彻底绕开 Fstat。
	// MTU 1500、GSO 关闭；Inet4/Inet6 用 mihomo gvisor 常规网段（198.18.0.0/30 + fdfe:dcba:9876::/126）。
	tunConf := LC.Tun{
		Enable:         true,
		Stack:          MC.TunGvisor,
		FileDescriptor: fd,
		MTU:            1500,
		Inet4Address:   []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")},
		Inet6Address:   []netip.Prefix{netip.MustParsePrefix("fdfe:dcba:9876::1/126")},
	}
	tunConf.Sort()

	// 直接调 sing_tun.New 拿真实错误（listener.ReCreateTun 会吞 err 只留日志，
	// 此前 attach "code=0" 但 gvisor 栈可能根本没建起来，导致系统包进 fd 后无应答）。
	lister, err := sing_tun.New(tunConf, tunnel.Tunnel)
	if err != nil {
		return cmsg("sing_tun.New: %v", err)
	}
	tunLister.Store(lister)
	return nil
}

// probeFdFrame 后台连续抢读隧道 fd 的前 N 帧（25s 窗口），每帧记时间戳+首字节+hex 落盘
// fd-probe.txt，用于确认哪些流量类型真的进隧道（IPv4 SYN 有没有进来是 IPv4 出网超时根因的分水岭）。
// 注意：与 tunLoop 竞争同一 fd，抢走的帧不会回给 tunLoop——TCP 会重传，不影响后续诊断。
// 单个 fd 全程只有一个 probe goroutine；首次读到包后继续抓满 12 帧再收工。
var lastHome string

func probeFdFrame(fd int) {
	buf := make([]byte, 4096)
	_ = unix.SetNonblock(fd, true)
	deadline := time.Now().Add(25 * time.Second)
	const maxFrames = 12
	var sb strings.Builder
	frames := 0
	flush := func() {
		_ = os.WriteFile(lastHome+"/fd-probe.txt", []byte(sb.String()), 0o644)
	}
	for frames < maxFrames && time.Now().Before(deadline) {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			frames++
			tag := "?"
			switch {
			case buf[0]>>4 == 4:
				tag = "IPv4"
			case buf[0]>>4 == 6:
				tag = "IPv6"
			}
			sb.WriteString(fmt.Sprintf("[%02d] t=%dms n=%d first=0x%02x(%s) hex=%s\n",
				frames, time.Now().UnixMilli(), n, buf[0], tag, hexEncode(buf[:minInt(n, 24)])))
			flush() // 每帧立即落盘，ETS 侧延迟读时能拿到已抓部分
		} else if err != nil && err != unix.EAGAIN {
			sb.WriteString("read err=" + err.Error() + "\n")
			flush()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if frames == 0 {
		sb.WriteString("no packet within 25s\n")
	} else {
		sb.WriteString(fmt.Sprintf("total frames=%d\n", frames))
	}
	flush()
}

func hexEncode(b []byte) string {
	const lower = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, v := range b {
		out = append(out, lower[v>>4], lower[v&0x0f])
	}
	return string(out)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// tunLister 保存 sing-tun listener 句柄，防止被 GC 回收导致 goroutine 停摆。
var tunLister atomic.Pointer[sing_tun.Listener]

func main() {}

// 规避 cgo 对 main 包内未引用“import C”生成 C.CString 的 codegen 冲突时对程序集的符号探测：保持导出符号稳定。
var _ = unsafe.Pointer(C.CString(""))