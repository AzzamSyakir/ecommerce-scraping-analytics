package controllers

import "gorm.io/gorm"

type LogicController struct {
	Db *gorm.DB
}

func NewLogicController(db *gorm.DB) *LogicController {
	logicController := &LogicController{
		Db: db,
	}
	return logicController
}
