package controller

import (
	"gorm.io/gorm"
	"wireflow/pkg/log"
)

type PlanController struct {
	logger *log.Logger
	db     *gorm.DB
}

func NewPlanController(db *gorm.DB) *PlanController {
	return &PlanController{db: db, logger: log.NewLogger(log.Loglevel, "plan-controller")}
}
