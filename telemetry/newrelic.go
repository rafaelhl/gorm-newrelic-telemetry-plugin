package telemetry

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/newrelic/go-agent/v3/newrelic"
	"gorm.io/gorm"
)

const (
	callbackName     = "gorm:%s"
	registerName     = "telemetrynr:%s"
	newrelicStartKey = "newrelicStartTime:%s"
)

type (
	config struct {
		name    string
		address string
		product newrelic.DatastoreProduct
	}
	NrTracer struct {
		cfg config
	}
)

func NewNrTracer(databaseName, databaseAddress, product string) NrTracer {
	return NrTracer{
		cfg: config{
			name:    databaseName,
			address: databaseAddress,
			product: newrelic.DatastoreProduct(product),
		},
	}
}

func name(pattern, value string) string {
	return fmt.Sprintf(pattern, value)
}

// Name will return the name of plugin that will be registered on GORM DB.
func (n NrTracer) Name() string {
	return "newrelic-telemetry-plugin"
}

// Initialize will register necessary callbacks to creates segments on newrelic transaction sending the used SQL.
func (n NrTracer) Initialize(db *gorm.DB) error {
	n.registerCreateTelemetry(db)
	n.registerQueryTelemetry(db)
	n.registerRowTelemetry(db)
	n.registerRawTelemetry(db)
	n.registerUpdateTelemetry(db)
	n.registerDeleteTelemetry(db)
	return nil
}

func (n NrTracer) registerCreateTelemetry(db *gorm.DB) {
	callback := name(callbackName, "create")

	_ = db.Callback().Create().Before(callback).
		Register(name(registerName, "before_create"), before("INSERT"))

	_ = db.Callback().Create().After(callback).
		Register(name(registerName, "after_create"), after("INSERT", n.cfg))

	_ = db.Callback().Create().Before(name(callbackName, "begin_transaction")).
		Register(name(registerName, "before_transaction_create"), before("TRANSACTION"))
	_ = db.Callback().Create().After(name(callbackName, "commit_or_rollback_transaction")).
		Register(name(registerName, "after_transaction_create"), after("TRANSACTION", n.cfg))
}

func (n NrTracer) registerQueryTelemetry(db *gorm.DB) {
	callback := name(callbackName, "query")

	_ = db.Callback().Query().Before(callback).
		Register(name(registerName, "before_query"), before("SELECT"))

	_ = db.Callback().Query().After(callback).
		Register(name(registerName, "after_query"), after("SELECT", n.cfg))
}

func (n NrTracer) registerRowTelemetry(db *gorm.DB) {
	callback := name(callbackName, "row")

	_ = db.Callback().Row().Before(callback).
		Register(name(registerName, "before_row"), before("ROW"))

	_ = db.Callback().Row().After(callback).
		Register(name(registerName, "after_row"), after("ROW", n.cfg))
}

func (n NrTracer) registerRawTelemetry(db *gorm.DB) {
	callback := name(callbackName, "raw")

	_ = db.Callback().Raw().Before(callback).
		Register(name(registerName, "before_raw"), before("RAW"))

	_ = db.Callback().Raw().After(callback).
		Register(name(registerName, "after_raw"), after("RAW", n.cfg))
}

func (n NrTracer) registerUpdateTelemetry(db *gorm.DB) {
	callback := name(callbackName, "update")

	_ = db.Callback().Update().Before(callback).
		Register(name(registerName, "before_update"), before("UPDATE"))

	_ = db.Callback().Update().After(callback).
		Register(name(registerName, "after_update"), after("UPDATE", n.cfg))

	_ = db.Callback().Update().Before(name(callbackName, "begin_transaction")).
		Register(name(registerName, "before_transaction_update"), before("TRANSACTION"))
	_ = db.Callback().Update().After(name(callbackName, "commit_or_rollback_transaction")).
		Register(name(registerName, "after_transaction_update"), after("TRANSACTION", n.cfg))
}

func (n NrTracer) registerDeleteTelemetry(db *gorm.DB) {
	callback := name(callbackName, "delete")

	_ = db.Callback().Delete().Before(callback).
		Register(name(registerName, "before_delete"), before("DELETE"))

	_ = db.Callback().Delete().After(callback).
		Register(name(registerName, "after_delete"), after("DELETE", n.cfg))

	_ = db.Callback().Delete().Before(name(callbackName, "begin_transaction")).
		Register(name(registerName, "before_transaction_update"), before("TRANSACTION"))
	_ = db.Callback().Delete().After(name(callbackName, "commit_or_rollback_transaction")).
		Register(name(registerName, "after_transaction_delete"), after("TRANSACTION", n.cfg))
}

var after = func(operation string, cfg config) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if segment := createSegment(db, operation, cfg); segment != nil {
			segment.End()
		}
		if transaction := newrelic.FromContext(db.Statement.Context); transaction != nil {
			transaction.End()
		}
	}
}

var createSegment = func(db *gorm.DB, operation string, cfg config) *newrelic.DatastoreSegment {
	startTime, ok := db.Get(name(newrelicStartKey, operation))
	if !ok {
		return nil
	}

	return &newrelic.DatastoreSegment{
		StartTime:          startTime.(newrelic.SegmentStartTime),
		Product:            cfg.product,
		Operation:          strings.ToLower(operation),
		Collection:         db.Statement.Table,
		ParameterizedQuery: db.Statement.SQL.String(),
		QueryParameters:    parseVars(db.Statement.Vars),
		DatabaseName:       cfg.name,
		Host:               cfg.address,
	}
}

func parseVars(vars []interface{}) map[string]interface{} {
	queryParameters := make(map[string]interface{})
	for i, v := range vars {
		queryParameters[strconv.Itoa(i+1)] = fmt.Sprintf("%v", v)
	}
	return queryParameters
}

var before = func(operation string) func(db *gorm.DB) {
	segmentKey := name(newrelicStartKey, operation)
	return func(db *gorm.DB) {
		if transaction := newrelic.FromContext(db.Statement.Context); transaction != nil {
			db.Set(segmentKey, transaction.StartSegmentNow())
		}
	}
}
