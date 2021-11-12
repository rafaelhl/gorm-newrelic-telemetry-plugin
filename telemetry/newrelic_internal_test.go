package telemetry

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type entityGorm struct {
	ID   *uint `gorm:"primarykey"`
	Name string
}

func (entityGorm) TableName() string {
	return "teste"
}

func createEntityValidData() *entityGorm {
	return &entityGorm{
		Name: fmt.Sprintf("%d", rand.Int()),
	}
}

const (
	databaseName = "testdb"
	databaseAddress = "local"
	sqliteDB = "gorm.cfg"
)

var (
	beforeOriginalFn        = before
	afterOriginalFn         = after
	createSegmentOriginalFn = createSegment
)

func TestNrTracer(t *testing.T) {
	ctxWithTxn := newrelic.NewContext(context.Background(), &newrelic.Transaction{})

	cases := []struct {
		title      string
		ctx        context.Context
		operations []string
		test       func(context.Context, *gorm.DB, *entityGorm)
	}{
		{
			title:      "shouldn't break the app without nr txn on context",
			ctx:        context.Background(),
			operations: []string{"SELECT"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				db.WithContext(ctx).First(entity)
			},
		},
		{
			title:      "shouldn't break the app without using context",
			ctx:        context.Background(),
			operations: []string{"SELECT"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				db.First(entity)
			},
		},
		{
			title:      "should track complete create operation",
			ctx:        ctxWithTxn,
			operations: []string{"TRANSACTION", "INSERT"},
			test: func(ctx context.Context, db *gorm.DB, _ *entityGorm) {
				newEntity := createEntityValidData()
				db.WithContext(ctx).Create(newEntity)
			},
		},
		{
			title:      "should track complete row operation",
			ctx:        ctxWithTxn,
			operations: []string{"ROW"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				var expected entityGorm
				row := db.WithContext(ctx).Table("teste").Where("name = ?", "test").Select("name").Row()
				_ = row.Scan(&expected)
			},
		},
		{
			title:      "should track complete raw operation",
			ctx:        ctxWithTxn,
			operations: []string{"RAW"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				db.WithContext(ctx).Exec("DELETE FROM teste")
			},
		},
		{
			title:      "should track complete update operation",
			ctx:        ctxWithTxn,
			operations: []string{"TRANSACTION", "UPDATE"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				db.WithContext(ctx).Model(entity).Update("name", "updated name")
			},
		},
		{
			title:      "should track complete delete operation",
			ctx:        ctxWithTxn,
			operations: []string{"TRANSACTION", "DELETE"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				db.WithContext(ctx).Delete(entity)
			},
		},
		{
			title:      "should track complete find operation",
			ctx:        ctxWithTxn,
			operations: []string{"SELECT"},
			test: func(ctx context.Context, db *gorm.DB, entity *entityGorm) {
				var expected entityGorm
				db.WithContext(ctx).Model(expected).Find(&expected, entity.ID)
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.title, func(t *testing.T) {
			ctx := testCase.ctx
			defer cleanTrackDataTest()
			gormDB, _ := gorm.Open(sqlite.Open(sqliteDB), &gorm.Config{})
			expectedCalls := setupMocks(ctx, t, testCase.operations)
			existentEntity := createEntityValidData()
			gormDB.WithContext(ctx).Create(existentEntity)

			err := gormDB.Use(NewNrTracer(databaseName, databaseAddress, "SQLite"))
			assert.NoError(t, err)

			testCase.test(ctx, gormDB, existentEntity)

			assertFuncCalls(t, testCase.operations, expectedCalls)
		})
	}
}

func setupMocks(ctx context.Context, t *testing.T, expectedOperations []string) []map[string]bool {
	expectedBeforeCall := make(map[string]bool, len(expectedOperations))
	expectedAfterCall := make(map[string]bool, len(expectedOperations))
	expectedCreateSegmentCall := make(map[string]bool, len(expectedOperations))
	withoutNrTxn := newrelic.FromContext(ctx) == nil

	before = func(operation string) func(db *gorm.DB) {
		return func(db *gorm.DB) {
			beforeOriginalFn(operation)(db)
			expectedBeforeCall[operation] = true
			startTime, ok := db.Get(name(newrelicStartKey, operation))
			assert.Equal(t, ok, !withoutNrTxn)
			if ok {
				assert.IsType(t, newrelic.SegmentStartTime{}, startTime)
			}
		}
	}

	after = func(operation string, cfg config) func(*gorm.DB) {
		return func(db *gorm.DB) {
			afterOriginalFn(operation, cfg)(db)
			expectedAfterCall[operation] = true
		}
	}

	createSegment = func(db *gorm.DB, operation string, cfg config) *newrelic.DatastoreSegment {
		segment := createSegmentOriginalFn(db, operation, cfg)
		expectedCreateSegmentCall[operation] = true
		if withoutNrTxn {
			assert.Nil(t, segment)
			return nil
		}
		entity := entityGorm{}
		assert.NotNil(t, segment)
		assert.Equal(t, strings.ToLower(operation), segment.Operation)
		if operation != "RAW" {
			assert.Equal(t, entity.TableName(), segment.Collection)
		}
		return segment
	}

	return []map[string]bool{
		expectedBeforeCall,
		expectedAfterCall,
		expectedCreateSegmentCall,
	}
}

func assertFuncCalls(t *testing.T, expectedOperations []string, expectedCalls []map[string]bool) {
	for _, expectedCall := range expectedCalls {
		assert.Equal(t, len(expectedOperations), len(expectedCall), fmt.Sprintf("unexpected call %v", expectedCall))
		for i := range expectedOperations {
			assert.True(t, expectedCall[expectedOperations[i]])
		}
	}
}

func cleanTrackDataTest() {
	_ = os.Remove(sqliteDB)
	before = beforeOriginalFn
	after = afterOriginalFn
	createSegment = createSegmentOriginalFn
}
