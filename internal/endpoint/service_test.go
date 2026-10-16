package endpoint

import (
	"NodePassDash/internal/models"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDeleteEndpointSkipsMissingOptionalCleanupTables(t *testing.T) {
	db := newEndpointTestDB(t)
	endpointID := createEndpointForDeleteTest(t, db, "master-a")

	for _, tableName := range optionalEndpointCleanupTables {
		if db.Migrator().HasTable(tableName) {
			t.Fatalf("optional table %s should not exist in this test", tableName)
		}
	}

	if err := NewService(db).DeleteEndpoint(endpointID); err != nil {
		t.Fatalf("DeleteEndpoint returned error with missing optional tables: %v", err)
	}

	var count int64
	if err := db.Model(&models.Endpoint{}).Where("id = ?", endpointID).Count(&count).Error; err != nil {
		t.Fatalf("count endpoint: %v", err)
	}
	if count != 0 {
		t.Fatalf("endpoint was not deleted, count=%d", count)
	}
}

func TestDeleteEndpointCleansExistingOptionalTables(t *testing.T) {
	db := newEndpointTestDB(t)
	endpointID := createEndpointForDeleteTest(t, db, "master-b")
	otherEndpointID := createEndpointForDeleteTest(t, db, "master-c")

	for _, tableName := range optionalEndpointCleanupTables {
		if err := db.Exec("CREATE TABLE " + tableName + " (id INTEGER PRIMARY KEY AUTOINCREMENT, endpoint_id INTEGER NOT NULL)").Error; err != nil {
			t.Fatalf("create optional table %s: %v", tableName, err)
		}
		if err := db.Exec("INSERT INTO "+tableName+" (endpoint_id) VALUES (?), (?)", endpointID, otherEndpointID).Error; err != nil {
			t.Fatalf("insert optional table %s: %v", tableName, err)
		}
	}

	if err := NewService(db).DeleteEndpoint(endpointID); err != nil {
		t.Fatalf("DeleteEndpoint returned error: %v", err)
	}

	for _, tableName := range optionalEndpointCleanupTables {
		var deletedCount int64
		if err := db.Table(tableName).Where("endpoint_id = ?", endpointID).Count(&deletedCount).Error; err != nil {
			t.Fatalf("count deleted rows in %s: %v", tableName, err)
		}
		if deletedCount != 0 {
			t.Fatalf("expected no rows for deleted endpoint in %s, got %d", tableName, deletedCount)
		}

		var keptCount int64
		if err := db.Table(tableName).Where("endpoint_id = ?", otherEndpointID).Count(&keptCount).Error; err != nil {
			t.Fatalf("count kept rows in %s: %v", tableName, err)
		}
		if keptCount != 1 {
			t.Fatalf("expected other endpoint row to remain in %s, got %d", tableName, keptCount)
		}
	}
}

func newEndpointTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&models.Endpoint{},
		&models.Tunnel{},
		&models.TunnelGroup{},
		&models.TunnelOperationLog{},
		&models.TrafficHourlySummary{},
		&models.ServiceHistory{},
		&models.Services{},
	); err != nil {
		t.Fatalf("migrate endpoint test schema: %v", err)
	}

	return db
}

func createEndpointForDeleteTest(t *testing.T, db *gorm.DB, name string) int64 {
	t.Helper()

	endpoint := models.Endpoint{
		Name:    name,
		URL:     "http://" + name + ".example.com",
		APIPath: "/api",
		APIKey:  "test-key",
		Status:  models.EndpointStatusOffline,
	}
	if err := db.Create(&endpoint).Error; err != nil {
		t.Fatalf("create endpoint %s: %v", name, err)
	}
	return endpoint.ID
}
