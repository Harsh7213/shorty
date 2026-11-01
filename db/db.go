package db

import (
	_ "embed"
	"fmt"
	"log"
)

//go:embed init.sql
var initSQL string

func InitDB() error {
	if DB == nil {
		return fmt.Errorf("database connection pool is not initialized")
	}
	_, err := DB.Exec(initSQL)
	if err != nil {
		return fmt.Errorf("failed to execute init.sql: %v", err)
	}
	log.Println("Database initialized successfully.")
	return nil
}
