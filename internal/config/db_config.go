package config

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseConfig struct {
	DB *GormDatabase
}

type GormDatabase struct {
	Connection *gorm.DB
}

func NewDBConfig() *DatabaseConfig {
	databaseConfig := &DatabaseConfig{
		DB: NewDatabaseConnection(),
	}
	return databaseConfig
}

func NewDatabaseConnection() *GormDatabase {
	var (
		postgresHost     = os.Getenv("POSTGRES_HOST")
		postgresPort     = os.Getenv("POSTGRES_PORT")
		postgresUser     = os.Getenv("POSTGRES_USER")
		postgresPassword = os.Getenv("POSTGRES_PASSWORD")
		postgresDb       = os.Getenv("POSTGRES_DB")
	)
	postgresPortInt, err := strconv.Atoi(postgresPort)
	if err != nil {
		panic(err)
	}
	sqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", postgresHost, postgresPortInt, postgresUser, postgresPassword, postgresDb)
	connection, err := gorm.Open(postgres.Open(sqlInfo), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	Db := &GormDatabase{
		Connection: connection,
	}
	return Db
}
