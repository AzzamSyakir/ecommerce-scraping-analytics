package config

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DatabaseConnection() (*gorm.DB, error) {
	var (
		postgresHost     = os.Getenv("POSTGRES_HOST")
		postgresPort     = os.Getenv("POSTGRES_PORT")
		postgresUser     = os.Getenv("POSTGRES_USER")
		postgresPassword = os.Getenv("POSTGRES_PORT")
		postgresDb       = os.Getenv("POSTGRES_DB")
	)
	postgresPortInt, err := strconv.Atoi(postgresPort)
	if err != nil {
		return nil, err
	}
	sqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", postgresHost, postgresPortInt, postgresUser, postgresPassword, postgresDb)

	db, err := gorm.Open(postgres.Open(sqlInfo), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
