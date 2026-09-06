package route

import (
	"github.com/metacubex/mihomo/tunnel/statistic"

	"github.com/metacubex/chi/render"
	"github.com/metacubex/http"
)

// ProxyUsage 单个节点（按名称）在当前活跃会话期间的累计流量
type ProxyUsage struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// usage 返回全局流量与按节点聚合的流量统计（只读，不触发任何操作）。
// 数据来源：当前活跃连接（statistic.Manager）的链上节点聚合；连接关闭后对应计数随之消失，
// 全局累计流量（upTotal/downTotal）不受影响。
func usage(w http.ResponseWriter, r *http.Request) {
	t := statistic.DefaultManager

	proxyUsage := map[string]ProxyUsage{}
	t.Range(func(c statistic.Tracker) bool {
		info := c.Info()
		chain := info.Chain
		if len(chain) == 0 {
			return true
		}
		up := info.UploadTotal.Load()
		down := info.DownloadTotal.Load()
		// 同一连接链路中去重（避免嵌套链中同名节点被重复计入）
		seen := make(map[string]struct{}, len(chain))
		for _, name := range chain {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			p := proxyUsage[name]
			p.Up += up
			p.Down += down
			proxyUsage[name] = p
		}
		return true
	})

	up, down := t.Now()
	upTotal, downTotal := t.Total()

	render.JSON(w, r, render.M{
		"up":       up,
		"down":     down,
		"upTotal":  upTotal,
		"downTotal": downTotal,
		"proxies":  proxyUsage,
	})
}