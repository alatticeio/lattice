package repository

import (
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type ProfileRepository struct {
	log *log.Logger
	*BaseRepository[models.UserProfile]
}

func NewProfileRepository(db *gorm.DB) *ProfileRepository {
	return &ProfileRepository{
		log:            log.GetLogger("profile"),
		BaseRepository: NewBaseRepository[models.UserProfile](db),
	}
}
