package nodepass

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"NodePassDash/internal/models"
)

func i64Ptr(v int64) *int64 { return &v }

// baseTunnel 返回一份最小可用的隧道配置（能构造出合法的 URL）
func baseTunnel() models.Tunnel {
	return models.Tunnel{
		Type:          models.TunnelType("client"),
		TunnelAddress: "1.2.3.4",
		TunnelPort:    "10101",
		TargetAddress: "127.0.0.1",
		TargetPort:    "8080",
	}
}

// mapInt64 读取 map 里的 *int64 值，返回（值，是否有非 nil 的值）
func mapInt64(t *testing.T, m map[string]interface{}, key string) (int64, bool) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("map 里缺少键 %q", key)
	}
	p, ok := v.(*int64)
	if !ok {
		t.Fatalf("键 %q 的值类型是 %T，期望 *int64", key, v)
	}
	if p == nil {
		return 0, false
	}
	return *p, true
}

// 设了值：URL 里带上参数，能解析回来，TunnelToMap 里键也有值
func TestTunnelURLRoundTrip_WithValues(t *testing.T) {
	tunnel := baseTunnel()
	tunnel.Rate = i64Ptr(1024)
	tunnel.Slot = i64Ptr(100)
	tunnel.IPSlot = i64Ptr(5)

	raw := BuildTunnelURLs(tunnel)
	for _, want := range []string{"rate=1024", "slot=100", "ipslot=5"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("URL 里缺少 %q：%s", want, raw)
		}
	}

	parsed := ParseTunnelURL(raw)
	if parsed.IPSlot == nil || *parsed.IPSlot != 5 {
		t.Fatalf("解析回来的 IPSlot = %v，期望 5", parsed.IPSlot)
	}

	updates := TunnelToMap(parsed, true)
	if got, ok := mapInt64(t, updates, "ipslot"); !ok || got != 5 {
		t.Fatalf("TunnelToMap[ipslot] = %d(存在=%v)，期望 5", got, ok)
	}
	if got, ok := mapInt64(t, updates, "slot"); !ok || got != 100 {
		t.Fatalf("TunnelToMap[slot] = %d(存在=%v)，期望 100", got, ok)
	}
	if got, ok := mapInt64(t, updates, "rate"); !ok || got != 1024 {
		t.Fatalf("TunnelToMap[rate] = %d(存在=%v)，期望 1024", got, ok)
	}
}

// 清空：URL 里没有参数，TunnelToMap 里键仍然存在且为 nil（= 落库写成 NULL）
// 这是"清空值存不下去"修复的核心断言
func TestTunnelToMap_ClearedValuesBecomeNull(t *testing.T) {
	tunnel := baseTunnel() // Rate/Slot/IPSlot 全为 nil

	raw := BuildTunnelURLs(tunnel)
	for _, forbidden := range []string{"ipslot=", "slot=", "rate="} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("URL 里不应出现 %q：%s", forbidden, raw)
		}
	}

	parsed := ParseTunnelURL(raw)
	updates := TunnelToMap(parsed, true)

	for _, key := range []string{"ipslot", "slot", "rate", "min", "max", "mode", "read", "dial", "dns", "sni", "block", "lbs", "pool_type", "cert_path", "key_path"} {
		v, ok := updates[key]
		if !ok {
			t.Fatalf("map 里缺少键 %q：清空的值不会被写进数据库", key)
		}
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Ptr || !rv.IsNil() {
			t.Fatalf("键 %q = %v(%T)，期望一个 nil 指针（落库即 NULL）", key, v, v)
		}
		// 定型 nil 指针经 database/sql 转换后就是 SQL NULL
		if converted, err := driver.DefaultParameterConverter.ConvertValue(v); err != nil || converted != nil {
			t.Fatalf("键 %q 的值转换后 = %v (err=%v)，期望 NULL", key, converted, err)
		}
	}
}

// withConfig=false 只写运行时列，不碰配置列（实例没上报 URL 时用）
func TestTunnelToMap_RuntimeOnly(t *testing.T) {
	tunnel := baseTunnel()
	tunnel.IPSlot = i64Ptr(7)

	updates := TunnelToMap(&tunnel, false)
	for _, key := range []string{"ipslot", "slot", "rate", "command_line", "target_address"} {
		if _, ok := updates[key]; ok {
			t.Fatalf("运行时更新不该包含配置列 %q", key)
		}
	}
	for _, key := range []string{"tcp_rx", "tcp_tx", "pool", "ping", "updated_at"} {
		if _, ok := updates[key]; !ok {
			t.Fatalf("运行时更新缺少键 %q", key)
		}
	}
}

// 详情页 config 块：ParseTunnelConfig 能读出 ipslot
func TestParseTunnelConfig_IPSlot(t *testing.T) {
	cfg := ParseTunnelConfig("client://pass@1.2.3.4:10101/127.0.0.1:8080?slot=100&ipslot=5")
	if cfg.IPSlot != "5" {
		t.Fatalf("IPSlot = %q，期望 \"5\"", cfg.IPSlot)
	}

	empty := ParseTunnelConfig("client://pass@1.2.3.4:10101/127.0.0.1:8080?slot=100")
	if empty.IPSlot != "" {
		t.Fatalf("未设置时 IPSlot = %q，期望空字符串", empty.IPSlot)
	}
}

// BuildTunnelConfigURL（当前无人调用，保持一致）也要带上 ipslot
func TestBuildTunnelConfigURL_IPSlot(t *testing.T) {
	cfg := &TunnelConfig{
		Type:          "client",
		TunnelAddress: "1.2.3.4",
		TunnelPort:    "10101",
		TargetAddress: "127.0.0.1",
		TargetPort:    "8080",
		Slot:          "100",
		IPSlot:        "5",
	}
	raw := cfg.BuildTunnelConfigURL()
	if !strings.Contains(raw, "ipslot=5") {
		t.Fatalf("URL 里缺少 ipslot=5：%s", raw)
	}
}
