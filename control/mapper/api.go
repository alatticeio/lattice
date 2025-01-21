package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
)

type UserInterface interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)
}

type PeerInterface interface {
	Register(e *dto.PeerDto) (*entity.Peer, error)
	Update(e *dto.PeerDto) (*entity.Peer, error)
	Delete(e *dto.PeerDto) error
	GetByAppId(appId string) (*entity.Peer, error)
	FetchAll() ([]*entity.Peer, error)
	Watch() (<-chan *entity.Peer, error)
}
