package config

import (
	"testing"
)

func TestProbeParseJSON(t *testing.T) {
	cfgJSON := []byte(`{"mode":"rule","log-level":"debug","bind-address":"127.0.0.1","mixed-port":1031,"dns":{"enable":false},
"proxies":[{"name":"Master Studio-zz","type":"hysteria2","server":"device.yyyyyyz.site","port":29862,"password":"7my4428n85v3619a","alpn":["h3"],"client-fingerprint":"chrome"}],
"proxy-groups":[{"name":"PROXY","type":"select","proxies":["Master Studio-zz","DIRECT"]}],
"rules":["MATCH,PROXY"]}`)
	raw, err := UnmarshalRawConfig(cfgJSON)
	if err != nil {
		t.Fatalf("UnmarshalRawConfig err: %v", err)
	}
	t.Logf("raw.Proxy len=%d", len(raw.Proxy))
	t.Logf("raw.ProxyGroup len=%d", len(raw.ProxyGroup))
	t.Logf("raw.Rule len=%d %v", len(raw.Rule), raw.Rule)
	t.Logf("raw.MixedPort=%d", raw.MixedPort)

	cfg, err := ParseRawConfig(raw)
	if err != nil {
		t.Fatalf("ParseRawConfig err: %v", err)
	}
	t.Logf("cfg.Proxies keys:")
	for k := range cfg.Proxies {
		t.Logf("  - %s", k)
	}
	t.Logf("cfg.Rules len=%d", len(cfg.Rules))
}