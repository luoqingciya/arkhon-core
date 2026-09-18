package route

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/metacubex/http"
	"github.com/metacubex/http/httptest"
	C "github.com/metacubex/mihomo/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lastDelay 语义：历史最后一条即最近一次测试；历史为空或最近延迟为 0（上次失败）时返回 nil
func TestLastDelay(t *testing.T) {
	assert.Nil(t, lastDelay(nil), "空历史返回 nil")
	assert.Nil(t, lastDelay([]C.DelayHistory{{Time: time.Now(), Delay: 0}}), "最近一次延迟为 0 时返回 nil")
	assert.Nil(t, lastDelay([]C.DelayHistory{{Delay: 100}, {Delay: 0}}), "末条失败时返回 nil")
	d := lastDelay([]C.DelayHistory{{Delay: 120}, {Delay: 80}})
	require.NotNil(t, d, "取到最近一次成功延迟")
	assert.Equal(t, uint16(80), *d)
}

// delayLatest 空态：无代理注册时返回 {"proxies":{}} 且不产生错误（新端点 E2E 的稳定形态断言）
func TestDelayLatestEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/delay/latest", nil)
	rec := httptest.NewRecorder()
	delayLatest(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Proxies map[string]NodeDelay `json:"proxies"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotNil(t, body.Proxies)
	assert.Len(t, body.Proxies, 0)
}