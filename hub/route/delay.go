package route

import (
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/tunnel"

	"github.com/metacubex/chi/render"
	"github.com/metacubex/http"
)

// NodeDelay 单个节点的最近延迟快照
type NodeDelay struct {
	Alive  bool                 `json:"alive"`
	Delay  *uint16              `json:"delay"` // 最近一次成功测速延迟（ms），未测过/失败为 null
	Extras map[string]NodeDelay `json:"extras,omitempty"`
}

// delayLatest 返回全部节点（含代理组）最近一次延迟测试快照，读取已有缓存，不触发测速。
func delayLatest(w http.ResponseWriter, r *http.Request) {
	proxies := tunnel.Proxies()
	result := make(map[string]NodeDelay, len(proxies))

	for name, proxy := range proxies {
		entry := NodeDelay{
			Alive: proxy.AliveForTestUrl(""),
			Delay: lastDelay(proxy.DelayHistory()),
		}

		extras := proxy.ExtraDelayHistories()
		if len(extras) > 0 {
			entry.Extras = make(map[string]NodeDelay, len(extras))
			for testUrl, state := range extras {
				entry.Extras[testUrl] = NodeDelay{
					Alive: state.Alive,
					Delay: lastDelay(state.History),
				}
			}
		}

		result[name] = entry
	}

	render.JSON(w, r, render.M{"proxies": result})
}

// lastDelay 取历史最后一条（最近一次）的延迟；历史为空或延迟为 0（上次测试失败）时返回 nil
func lastDelay(history []C.DelayHistory) *uint16 {
	if len(history) == 0 {
		return nil
	}
	delay := history[len(history)-1].Delay
	if delay == 0 {
		return nil
	}
	return &delay
}