// Package database provides common database interfaces and utilities
package database

import (
	"database/sql"
	"time"
)

// DB represents a database connection
type DB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Close() error
}

// QueryResult represents a database query result
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
	Timestamp time.Time
}

// ConnectionConfig holds database connection configuration
type ConnectionConfig struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}
