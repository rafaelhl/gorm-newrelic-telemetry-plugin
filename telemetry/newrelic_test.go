package telemetry_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/rafaelhl/gorm-newrelic-telemetry-plugin/telemetry"
)

var db = "sqlite.db"

func TestMain(m *testing.M) {
	m.Run()
	_ = os.Remove(db)
}

func TestNrTracer_Initialize(t *testing.T) {
	gormDB, _ := gorm.Open(sqlite.Open(db), &gorm.Config{})
	err := gormDB.Use(telemetry.NewNrTracer(db, "local", "SQLite"))
	assert.NoError(t, err)
}
