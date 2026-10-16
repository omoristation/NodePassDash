package tunnelcache

import (
	"testing"

	"NodePassDash/internal/models"
)

// copyTunnel 是逐字段深拷贝，走缓存的列表接口全靠它
func TestCopyTunnelKeepsIPSlot(t *testing.T) {
	src := &models.Tunnel{
		ID:     1,
		Name:   "t1",
		Slot:   i64Ptr(100),
		IPSlot: i64Ptr(5),
	}

	got := (&TunnelShared{}).copyTunnel(src)
	if got == nil {
		t.Fatal("copyTunnel 返回 nil")
	}
	if got.IPSlot == nil || *got.IPSlot != 5 {
		t.Fatalf("拷贝后 IPSlot = %v，期望 5", got.IPSlot)
	}
	if got.IPSlot == src.IPSlot {
		t.Fatal("IPSlot 应该是深拷贝，不能与原对象共用同一个指针")
	}
	if got.Slot == nil || *got.Slot != 100 {
		t.Fatalf("拷贝后 Slot = %v，期望 100", got.Slot)
	}
}

func i64Ptr(v int64) *int64 { return &v }
