package config

import (
	"database/sql"
	"fmt"
)

type DatabaseConfig struct {
	DB *PostgresDatabase
}

type PostgresDatabase struct {
	Connection *sql.DB
}

func NewDBConfig(env *EnvConfig) *DatabaseConfig {
	databaseConfig := &DatabaseConfig{
		DB: NewDatabaseConnection(env),
	}
	return databaseConfig
}

func NewDatabaseConnection(envConfig *EnvConfig) *PostgresDatabase {
	if envConfig.Db.DBNeed == "true" {
		var url string
		if envConfig.Db.Password == "" {
			url = fmt.Sprintf(
				"postgresql://%s@%s:%s/%s",
				envConfig.Db.User,
				envConfig.Db.Host,
				envConfig.Db.Port,
				envConfig.Db.Database,
			)
		} else {
			url = fmt.Sprintf(
				"postgresql://%s:%s@%s:%s/%s?sslmode=disable",
				envConfig.Db.User,
				envConfig.Db.Password,
				envConfig.Db.Host,
				envConfig.Db.Port,
				envConfig.Db.Database,
			)
		}
		connection, err := sql.Open("postgres", url)
		if err != nil {
			panic(err)
		}

		Db := &PostgresDatabase{
			Connection: connection,
		}
		return Db
	}
	return nil
}
