package outbound

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	N "github.com/metacubex/mihomo/common/net"
	"github.com/metacubex/mihomo/common/utils"
	"github.com/metacubex/mihomo/component/ca"
	"github.com/metacubex/mihomo/component/resolver"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/transport/tuic/common"

	"github.com/metacubex/http"
	"github.com/metacubex/quic-go"
	qtls "github.com/metacubex/sing-quic"
	"github.com/metacubex/sing-quic/hysteria2"
	"github.com/metacubex/sing-quic/hysteria2/realm"
	M "github.com/metacubex/sing/common/metadata"
	"github.com/metacubex/tls"
)

const minHopInterval = 5
const defaultHopInterval = 120 // [fork] 默认跳频间隔由上游 30s 调优为 120s：降低端口切换频率，减少握手重建与延迟抖动（用户仍可通过 hop-interval 覆盖）

// quicWindowDefault 返回用户配置的窗口值；未配置（0）时使用 fork 调优默认值
func quicWindowDefault(v, def uint64) uint64 {
	if v == 0 {
		return def
	}
	return v
}

type Hysteria2 struct {
	*Base

	option *Hysteria2Option
	client *hysteria2.Client
}

type Hysteria2Option struct {
	BasicOption
	Name              string     `proxy:"name"`
	Server            string     `proxy:"server"`
	Port              int        `proxy:"port,omitempty"`
	Ports             string     `proxy:"ports,omitempty"`
	HopInterval       string     `proxy:"hop-interval,omitempty"`
	Up                string     `proxy:"up,omitempty"`
	Down              string     `proxy:"down,omitempty"`
	Password          string     `proxy:"password,omitempty"`
	Obfs              string     `proxy:"obfs,omitempty"`
	ObfsPassword      string     `proxy:"obfs-password,omitempty"`
	ObfsMinPacketSize int        `proxy:"obfs-min-packet-size,omitempty"`
	ObfsMaxPacketSize int        `proxy:"obfs-max-packet-size,omitempty"`
	SNI               string     `proxy:"sni,omitempty"`
	ECHOpts           ECHOptions `proxy:"ech-opts,omitempty"`
	SkipCertVerify    bool       `proxy:"skip-cert-verify,omitempty"`
	NameCertVerify    string     `proxy:"name-cert-verify,omitempty"`
	Fingerprint       string     `proxy:"fingerprint,omitempty"`
	Certificate       string     `proxy:"certificate,omitempty"`
	PrivateKey        string     `proxy:"private-key,omitempty"`
	ALPN              []string   `proxy:"alpn,omitempty"`
	CWND              int        `proxy:"cwnd,omitempty"`
	BBRProfile        string     `proxy:"bbr-profile,omitempty"`
	UdpMTU            int        `proxy:"udp-mtu,omitempty"`
	HandshakeTimeout  int        `proxy:"handshake-timeout,omitempty"`

	RealmOpts Hysteria2RealmOption `proxy:"realm-opts,omitempty"`

	// quic-go special config
	InitialStreamReceiveWindow     uint64 `proxy:"initial-stream-receive-window,omitempty"`
	MaxStreamReceiveWindow         uint64 `proxy:"max-stream-receive-window,omitempty"`
	InitialConnectionReceiveWindow uint64 `proxy:"initial-connection-receive-window,omitempty"`
	MaxConnectionReceiveWindow     uint64 `proxy:"max-connection-receive-window,omitempty"`
}

type Hysteria2RealmOption struct {
	Enable      bool     `proxy:"enable,omitempty"`
	ServerURL   string   `proxy:"server-url,omitempty"`
	Token       string   `proxy:"token,omitempty"`
	RealmID     string   `proxy:"realm-id,omitempty"`
	STUNServers []string `proxy:"stun-servers,omitempty"`

	// for ServerURL
	SNI            string   `proxy:"sni,omitempty"`
	SkipCertVerify bool     `proxy:"skip-cert-verify,omitempty"`
	NameCertVerify string   `proxy:"name-cert-verify,omitempty"`
	Fingerprint    string   `proxy:"fingerprint,omitempty"`
	Certificate    string   `proxy:"certificate,omitempty"`
	PrivateKey     string   `proxy:"private-key,omitempty"`
	ALPN           []string `proxy:"alpn,omitempty"`
}

func (h *Hysteria2) DialContext(ctx context.Context, metadata *C.Metadata) (_ C.Conn, err error) {
	c, err := h.client.DialConn(ctx, M.ParseSocksaddrHostPort(metadata.String(), metadata.DstPort))
	if err != nil {
		return nil, h.wrapConnError("建连", err)
	}
	return NewConn(c, h), nil
}

func (h *Hysteria2) ListenPacketContext(ctx context.Context, metadata *C.Metadata) (_ C.PacketConn, err error) {
	if err = h.ResolveUDP(ctx, metadata); err != nil {
		return nil, err
	}
	pc, err := h.client.ListenPacket(ctx)
	if err != nil {
		return nil, fmt.Errorf("hysteria2 UDP 会话建立失败（服务器可能禁用了 UDP）: %s: %w", h.option.Server, err)
	}
	if pc == nil {
		return nil, errors.New("packetConn is nil")
	}
	return NewPacketConn(N.NewThreadSafePacketConn(pc), h), nil
}

// wrapConnError 将 hysteria2 握手/建连错误按原因分类包装为可读错误，便于应用层与用户排障。
// 匹配不到已知类别时也会携带服务器地址，保证错误信息始终可定位。
func (h *Hysteria2) wrapConnError(action string, err error) error {
	server := h.option.Server
	lower := strings.ToLower(err.Error())

	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return fmt.Errorf("hysteria2 %s %s 超时: 服务器不可达，或 TLS/obfs 参数不匹配导致握手被服务端静默断开; 请检查服务器连通性与 sni/alpn/obfs 配置 (%w)", action, server, err)
	case strings.Contains(lower, "x509") || strings.Contains(lower, "certificate") || strings.Contains(lower, "cert") ||
		strings.Contains(lower, "tls: handshake"):
		return fmt.Errorf("hysteria2 %s %s 失败: TLS 握手失败（证书校验/ALPN 不匹配/证书过期），可尝试 skip-cert-verify (%w)", action, server, err)
	case strings.Contains(lower, "authenticate") || strings.Contains(lower, "auth"):
		return fmt.Errorf("hysteria2 %s %s 失败: 服务器认证未通过，请检查 hysteria2 password (%w)", action, server, err)
	case strings.Contains(lower, "obfs") || strings.Contains(lower, "salamander") || strings.Contains(lower, "gecko"):
		return fmt.Errorf("hysteria2 %s %s 失败: obfs 参数不匹配（obfs/obfs-password 须与服务器一致）(%w)", action, server, err)
	default:
		return fmt.Errorf("hysteria2 %s %s 失败: %w", action, server, err)
	}
}

// Close implements C.ProxyAdapter
func (h *Hysteria2) Close() error {
	if h.client != nil {
		return h.client.CloseWithError(errors.New("proxy removed"))
	}
	return nil
}

// ProxyInfo implements C.ProxyAdapter
func (h *Hysteria2) ProxyInfo() C.ProxyInfo {
	info := h.Base.ProxyInfo()
	info.DialerProxy = h.option.DialerProxy
	return info
}

func NewHysteria2(option Hysteria2Option) (*Hysteria2, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	outbound := &Hysteria2{
		Base: NewBase(BaseOption{
			Name:         option.Name,
			Addr:         addr,
			Type:         C.Hysteria2,
			ProviderName: option.ProviderName,
			UDP:          true,
			Interface:    option.Interface,
			RoutingMark:  option.RoutingMark,
			Prefer:       option.IPVersion,
		}),
		option: &option,
	}
	outbound.dialer = option.NewDialer(outbound.DialOptions())

	var salamanderPassword string
	var geckoPassword string
	var geckoMinPacketSize, geckoMaxPacketSize int
	if len(option.Obfs) > 0 {
		if option.ObfsPassword == "" {
			return nil, errors.New("missing obfs password")
		}
		switch option.Obfs {
		case hysteria2.ObfsTypeSalamander:
			salamanderPassword = option.ObfsPassword
		case hysteria2.ObfsTypeGecko:
			geckoPassword = option.ObfsPassword
			geckoMinPacketSize = option.ObfsMinPacketSize
			geckoMaxPacketSize = option.ObfsMaxPacketSize
		default:
			return nil, fmt.Errorf("unknown obfs type: %s", option.Obfs)
		}
	}

	serverName := option.Server
	if option.SNI != "" {
		serverName = option.SNI
	}

	tlsConfig, err := ca.GetTLSConfig(ca.Option{
		TLSConfig: &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: option.SkipCertVerify,
			MinVersion:         tls.VersionTLS13,
		},
		Fingerprint:    option.Fingerprint,
		NameCertVerify: option.NameCertVerify,
		Certificate:    option.Certificate,
		PrivateKey:     option.PrivateKey,
	})
	if err != nil {
		return nil, err
	}

	if option.ALPN != nil { // structure's Decode will ensure value not nil when input has value even it was set an empty array
		tlsConfig.NextProtos = option.ALPN
	}

	tlsClientConfig := tlsConfig
	echConfig, err := option.ECHOpts.Parse()
	if err != nil {
		return nil, err
	}

	if option.UdpMTU == 0 {
		// "1200" from quic-go's MaxDatagramSize
		// "-3" from quic-go's DatagramFrame.MaxDataLen
		option.UdpMTU = 1200 - 3
	}

	// [fork] QUIC 流控窗口默认值调优：用户未显式配置（0）时注入 Meta 官方推荐值，
	// 提升大带宽场景下的下行吞吐；显式配置时保持用户覆盖
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     quicWindowDefault(option.InitialStreamReceiveWindow, 8<<20),
		MaxStreamReceiveWindow:         quicWindowDefault(option.MaxStreamReceiveWindow, 8<<20),
		InitialConnectionReceiveWindow: quicWindowDefault(option.InitialConnectionReceiveWindow, 16<<20),
		MaxConnectionReceiveWindow:     quicWindowDefault(option.MaxConnectionReceiveWindow, 16<<20),
	}

	clientOptions := hysteria2.ClientOptions{
		Context:            context.TODO(),
		Logger:             log.SingLogger,
		SendBPS:            utils.StringToBps(option.Up),
		ReceiveBPS:         utils.StringToBps(option.Down),
		SalamanderPassword: salamanderPassword,
		GeckoPassword:      geckoPassword,
		GeckoMinPacketSize: geckoMinPacketSize,
		GeckoMaxPacketSize: geckoMaxPacketSize,
		Password:           option.Password,
		TLSConfig:          tlsClientConfig,
		QUICConfig:         quicConfig,
		UDPDisabled:        false,
		UdpMTU:             option.UdpMTU,
		ServerAddress:      M.ParseSocksaddr(addr),
		PacketListener:     outbound.dialer,
		QuicDialer: qtls.QuicDialerFunc(func(ctx context.Context, addr string, dialer qtls.PacketDialer, tlsCfg *tls.Config, cfg *quic.Config, early bool) (net.PacketConn, *quic.Conn, error) {
			err := echConfig.ClientHandle(ctx, tlsCfg)
			if err != nil {
				return nil, nil, err
			}
			return common.DialQuic(ctx, addr, outbound.DialOptions(), dialer, tlsCfg, cfg, common.DialQuicOption{Early: early})
		}),
		SetBBRCongestion: func(quicConn *quic.Conn) {
			common.SetCongestionController(quicConn, "bbr", option.CWND, option.BBRProfile)
		},
		HandshakeTimeout: time.Duration(option.HandshakeTimeout) * time.Second,
	}

	var serverPorts []uint16
	if option.Ports != "" {
		ranges, err := utils.NewUnsignedRanges[uint16](option.Ports)
		if err != nil {
			return nil, err
		}
		ranges.Range(func(port uint16) bool {
			serverPorts = append(serverPorts, port)
			return true
		})
		if len(serverPorts) > 0 {
			hopRange, err := utils.NewUnsignedRange[uint64](option.HopInterval)
			if err != nil {
				return nil, err
			}
			start, end := hopRange.Start(), hopRange.End()
			if start == 0 {
				start = defaultHopInterval
			} else if start < minHopInterval {
				start = minHopInterval
			}
			if end < start {
				end = start
			}
			clientOptions.HopInterval = time.Duration(start) * time.Second
			clientOptions.HopIntervalMax = time.Duration(end) * time.Second
			clientOptions.ServerPorts = serverPorts
		}
	}
	if option.Port == 0 && len(serverPorts) == 0 {
		return nil, errors.New("invalid port")
	}

	if option.RealmOpts.Enable {
		httpTLSClientConfig, err := ca.GetTLSConfig(ca.Option{
			TLSConfig: &tls.Config{
				ServerName:         option.RealmOpts.SNI,
				InsecureSkipVerify: option.RealmOpts.SkipCertVerify,
				NextProtos:         option.RealmOpts.ALPN,
			},
			Fingerprint:    option.RealmOpts.Fingerprint,
			NameCertVerify: option.RealmOpts.NameCertVerify,
			Certificate:    option.RealmOpts.Certificate,
			PrivateKey:     option.RealmOpts.PrivateKey,
		})
		if err != nil {
			return nil, err
		}
		clientOptions.RealmOptions = &realm.Options{
			ServerURL:   option.RealmOpts.ServerURL,
			Token:       option.RealmOpts.Token,
			RealmID:     option.RealmOpts.RealmID,
			STUNServers: option.RealmOpts.STUNServers,
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					DialContext:     outbound.dialer.DialContext,
					TLSClientConfig: httpTLSClientConfig,
					// from http.DefaultTransport
					ForceAttemptHTTP2:     true,
					MaxIdleConns:          100,
					IdleConnTimeout:       90 * time.Second,
					TLSHandshakeTimeout:   10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			},
			Resolver: func(ctx context.Context, host string, ipv4, ipv6 bool) ([]netip.Addr, error) {
				if ipv4 && !ipv6 {
					return resolver.LookupIPv4WithResolver(ctx, host, resolver.ProxyServerHostResolver)
				} else if ipv6 && !ipv4 {
					return resolver.LookupIPv6WithResolver(ctx, host, resolver.ProxyServerHostResolver)
				}
				return resolver.LookupIPWithResolver(ctx, host, resolver.ProxyServerHostResolver)
			},
			Logger: log.SingLogger,
		}
	}

	client, err := hysteria2.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}
	outbound.client = client

	return outbound, nil
}
