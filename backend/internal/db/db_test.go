package db

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewDB_CreatesConnection(t *testing.T) {
	tmpFile := createTempDB(t)
	defer os.Remove(tmpFile)

	database, err := NewDB(tmpFile)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer database.Close()

	if database.db == nil {
		t.Error("expected db connection to be non-nil")
	}
}

func TestMigration_CreatesAllTables(t *testing.T) {
	tmpFile := createTempDB(t)
	defer os.Remove(tmpFile)

	database, err := NewDB(tmpFile)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{"avatars", "conversations", "conversation_avatars", "messages"}
	for _, table := range tables {
		exists, err := database.tableExists(table)
		if err != nil {
			t.Errorf("failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s should exist after migration", table)
		}
	}
}

func TestSemaphoreExclusiveAccess(t *testing.T) {
	tmpFile := createTempDB(t)
	defer os.Remove(tmpFile)

	database, err := NewDB(tmpFile)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Track concurrent execution
	var maxConcurrent int32
	var currentConcurrent int32
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := database.WithLock(func() error {
				// Increment concurrent counter
				current := atomic.AddInt32(&currentConcurrent, 1)

				// Update max concurrent if needed
				for {
					max := atomic.LoadInt32(&maxConcurrent)
					if current <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
						break
					}
				}

				// Simulate work
				time.Sleep(10 * time.Millisecond)

				// Decrement concurrent counter
				atomic.AddInt32(&currentConcurrent, -1)
				return nil
			})
			if err != nil {
				t.Errorf("goroutine %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify only one goroutine accessed the database at a time
	if maxConcurrent != 1 {
		t.Errorf("expected max concurrent access to be 1, got %d", maxConcurrent)
	}
}

func TestDB_Close(t *testing.T) {
	tmpFile := createTempDB(t)
	defer os.Remove(tmpFile)

	database, err := NewDB(tmpFile)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if err := database.Close(); err != nil {
		t.Errorf("failed to close database: %v", err)
	}
}

// Helper function to create a temporary database file
func createTempDB(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

