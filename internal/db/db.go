package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type DBConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
}

type DBManager struct {
	db *sql.DB
}

func NewDBManager() (*DBManager, error) {
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

	return &DBManager{
		db: db,
	}, nil
}

func (m *DBManager) Close() error {
	return m.db.Close()
}

func (m *DBManager) Begin() (*sql.Tx, error) {
	return m.db.Begin()
}

func (m *DBManager) Prepare(query string) (*sql.Stmt, error) {
	return m.db.Prepare(query)
}

func (m *DBManager) Execute(query string, args ...interface{}) (sql.Result, error) {
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

func (m *DBManager) Query(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
