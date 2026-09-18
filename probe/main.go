package main

import (
	"fmt"

	"github.com/metacubex/mihomo/config"
)

func main() {
	cfgJSON := []byte(`{"mode":"rule","log-level":"debug","bind-address":"127.0.0.1","mixed-port":1031,"dns":{"enable":false},
"proxies":[{"name":"Master Studio-zz","type":"hysteria2","server":"device.yyyyyyz.site","port":29862,"password":"7my4428n85v3619a","alpn":["h3"],"client-fingerprint":"chrome"}],
"proxy-groups":[{"name":"PROXY","type":"select","proxies":["Master Studio-zz","DIRECT"]}],
"rules":["MATCH,PROXY"]}`)
	raw, err := config.UnmarshalRawConfig(cfgJSON)
	if err != nil {
		fmt.Printf("UnmarshalRawConfig err: %v\n", err)
		return
	}
	fmt.Printf("raw.Proxy=%d raw.ProxyGroup=%d raw.Rule=%v raw.MixedPort=%d\n",
		len(raw.Proxy), len(raw.ProxyGroup), raw.Rule, raw.MixedPort)

	cfg, err := config.ParseRawConfig(raw)
	if err != nil {
		fmt.Printf("ParseRawConfig err: %v\n", err)
		return
	}
	for k := range cfg.Proxies {
		fmt.Printf("  proxy: %s\n", k)
	}
	fmt.Printf("cfg.Rules=%d\n", len(cfg.Rules))
}