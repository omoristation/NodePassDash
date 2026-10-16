package nodepass

import (
	"database/sql"
	"testing"

	gormsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"NodePassDash/internal/models"
)

// 在真实 GORM + SQLite 上验证：清空后的配置值会写成 NULL（而不是"不写、留旧值"），
// 且配置列的列名与 TunnelConfigToMap 的键一一对应。
func TestTunnelConfigToMap_ClearsColumnsInSQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开 SQLite 失败: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{})
	if err != nil {
		t.Fatalf("初始化 GORM 失败: %v", err)
	}
	if err := db.AutoMigrate(&models.Endpoint{}, &models.Tunnel{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	// 列名必须真的是 ipslot（GORM 默认会把 IPSlot 命名成 ip_slot）
	if !db.Migrator().HasColumn(&models.Tunnel{}, "ipslot") {
		t.Fatal("tunnels 表没有 ipslot 列")
	}
	// TunnelConfigToMap 的每个键都必须是真实存在的列名
	for key := range TunnelConfigToMap(&models.Tunnel{}) {
		if !db.Migrator().HasColumn(&models.Tunnel{}, key) {
			t.Fatalf("TunnelConfigToMap 里的键 %q 不是 tunnels 表的列", key)
		}
	}

	rate, slot, ipSlot := int64(1024), int64(100), int64(5)
	row := models.Tunnel{
		Name:          "t1",
		EndpointID:    1,
		Type:          models.TunnelType("client"),
		TunnelAddress: "1.2.3.4",
		TunnelPort:    "10101",
		TargetAddress: "127.0.0.1",
		TargetPort:    "8080",
		Rate:          &rate,
		Slot:          &slot,
		IPSlot:        &ipSlot,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 模拟"清空后保存"：请求体里这些字段都没带
	cleared := models.Tunnel{
		Type:          models.TunnelType("client"),
		TunnelAddress: "1.2.3.4",
		TunnelPort:    "10101",
		TargetAddress: "127.0.0.1",
		TargetPort:    "8080",
	}
	if err := db.Model(&models.Tunnel{}).Where("id = ?", row.ID).
		Updates(TunnelConfigToMap(&cleared)).Error; err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	var got struct {
		Rate     *int64
		Slot     *int64
		IPSlot   *int64
		Min      *int64
		CertPath *string
	}
	if err := db.Table("tunnels").
		Select("rate, slot, ipslot, min, cert_path").
		Where("id = ?", row.ID).Scan(&got).Error; err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if got.Rate != nil || got.Slot != nil || got.IPSlot != nil || got.Min != nil || got.CertPath != nil {
		t.Fatalf("清空后仍是旧值: rate=%v slot=%v ipslot=%v min=%v cert_path=%v",
			got.Rate, got.Slot, got.IPSlot, got.Min, got.CertPath)
	}

	// 运行时列（name/流量）不该被动到
	var name string
	var tcpRx int64
	if err := db.Table("tunnels").Select("name, tcp_rx").
		Where("id = ?", row.ID).Row().Scan(&name, &tcpRx); err != nil {
		t.Fatalf("回读运行时列失败: %v", err)
	}
	if name != "t1" {
		t.Fatalf("name 被改成了 %q", name)
	}
}

// 老数据库（建表时还没有 ipslot 列）经 AutoMigrate 后应该自动补上这一列
func TestAutoMigrate_AddsIPSlotToExistingTable(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开 SQLite 失败: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{})
	if err != nil {
		t.Fatalf("初始化 GORM 失败: %v", err)
	}

	// 模拟旧版本建出来的 tunnels 表：没有 ipslot
	if err := db.Exec(`CREATE TABLE tunnels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		endpoint_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		tunnel_address TEXT NOT NULL,
		tunnel_port TEXT NOT NULL,
		target_address TEXT NOT NULL,
		target_port TEXT NOT NULL,
		slot INTEGER,
		command_line TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("建旧表失败: %v", err)
	}
	if db.Migrator().HasColumn(&models.Tunnel{}, "ipslot") {
		t.Fatal("旧表不该有 ipslot 列")
	}

	if err := db.AutoMigrate(&models.Tunnel{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	if !db.Migrator().HasColumn(&models.Tunnel{}, "ipslot") {
		t.Fatal("AutoMigrate 没有补上 ipslot 列")
	}
}
