package db

import (
	"database/sql"
	"log"
	"os"

	// mysql import
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type Config struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
}

type Manager struct {
	db *sql.DB
}

func NewDBManager() (*Manager, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("failed to load env", err)
	}

	db, err := sql.Open("mysql", os.Getenv("DSN"))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &Manager{
		db: db,
	}, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

func (m *Manager) Begin() (*sql.Tx, error) {
	return m.db.Begin()
}

func (m *Manager) Prepare(query string) (*sql.Stmt, error) {
	return m.db.Prepare(query)
}

func (m *Manager) Execute(query string, args ...interface{}) (sql.Result, error) {
	stmt, err := m.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *Manager) Query(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
