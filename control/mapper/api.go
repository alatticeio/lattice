package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
)

type UserInterface interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)

	//Get returns a user by token
	Get(username string) (*entity.User, error)
}

type PeerInterface interface {
	Register(e *dto.PeerDto) (*entity.Peer, error)
	Update(e *dto.PeerDto) (*entity.Peer, error)
	Delete(e *dto.PeerDto) error

	// GetByAppId returns a peer by appId, every client has its own appId
	GetByAppId(appId string) (*entity.Peer, error)

	// List returns a list of peers by userId，when client start up, it will call this method to get all the peers once
	// after that, it will call Watch method to get the latest peers
	List(userId string) ([]*entity.Peer, error)

	// Watch returns a channel that will be used to send the latest peers to the client
	//Watch() (<-chan *entity.Peer, error)
}
