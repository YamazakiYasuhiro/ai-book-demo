package db

import (
	"database/sql"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database with semaphore-based exclusive access
type DB struct {
	db    *sql.DB
	mutex sync.Mutex
}

// NewDB creates a new database connection with exclusive access control
func NewDB(dbPath string) (*DB, error) {
	// Enable WAL mode and foreign keys via connection string
	dsn := dbPath + "?_journal_mode=WAL&_foreign_keys=on"

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// Verify connection works
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	// Set connection pool to 1 to ensure single connection
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return &DB{db: sqlDB}, nil
}

// WithLock executes a function with exclusive database access
func (d *DB) WithLock(fn func() error) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return fn()
}

// WithLockResult executes a function with exclusive database access and returns a result
func WithLockResult[T any](d *DB, fn func() (T, error)) (T, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return fn()
}

// Exec executes a query with exclusive access
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return WithLockResult(d, func() (sql.Result, error) {
		return d.db.Exec(query, args...)
	})
}

// Query executes a query and returns rows with exclusive access
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return WithLockResult(d, func() (*sql.Rows, error) {
		return d.db.Query(query, args...)
	})
}

// QueryRow executes a query that returns a single row
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.db.QueryRow(query, args...)
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// tableExists checks if a table exists in the database
func (d *DB) tableExists(tableName string) (bool, error) {
	var count int
	err := d.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
