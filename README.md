# GORM NewRelic Telemetry Plugin
A plugin to allow telemetry by [NewRelic Go Agent](https://github.com/newrelic/go-agent) for [GORM](https://gorm.io/)

## Overview

Plugin implementation to add datastore segments on a [Newrelic](https://newrelic.com/) transaction injected on Go application context.

## Requirements

 - NewRelic Go Agent v3
 - Injected newrelic transaction on context of the app
 - Use `WithContext` function from `gorm.DB` passing the context with the transaction

## How to use

Since this plugin implements the interface [Plugin](https://gorm.io/docs/write_plugins.html#Plugin),
just follow the example below available on test file [newrelic_test.go](telemetry/newrelic_test.go):

```go
db := "sqlite.db"
gormDB, err := gorm.Open(sqlite.Open(db), &gorm.Config{})
if err != nil {
	panic(err)
}
err := gormDB.Use(telemetry.NewNrTracer(db, "local", "SQLite"))
assert.NoError(t, err)
```

```go
gormDB.WithContext(ctx).Create(newEntity)
```
