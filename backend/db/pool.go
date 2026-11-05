package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func ConnectDB() (*sql.DB, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("could not load .env: %v", err)
	}
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}
	if err := DB.Ping(); err != nil {
		DB.Close()
		DB = nil
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	if err := InitDB(); err != nil {
		DB.Close()
		DB = nil
		return nil, err
	}
	log.Println("Database connection established successfully.")
	return DB, nil
}

func CloseDB() error {
	if DB != nil {
		DB.Close()
	}
	return nil
}
