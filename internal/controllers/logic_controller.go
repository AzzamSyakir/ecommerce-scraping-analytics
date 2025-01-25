package controllers

import (
	"database/sql"
)

type LogicController struct {
	Db *sql.DB
}

func NewLogicController(db *sql.DB) *LogicController {
	logicController := &LogicController{
		Db: db,
	}
	return logicController
}
