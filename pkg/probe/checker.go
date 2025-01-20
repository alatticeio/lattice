package probe

import (
	"linkany/internal"
	"net"
)

// a RelayConn presents a connection to the relay server address.
// one peer will have a unique RelayConn.
type RelayConn struct {
	mappedAddr net.Addr       // the peer's mapped address
	relayConn  net.PacketConn // the peer's relay conn
}

func (r *RelayConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	return r.relayConn.ReadFrom(b)
}

// ConnChecker is the interface for checking the connection.
// DirectChecker and RelayChecker are the two implementations.
type ConnChecker interface {
	handleOffer(offer internal.Offer) error
	OnSuccess(addr string) error          // will add peer to wireguard
	OnFailure(offer internal.Offer) error // will remove peer from wireguard
}

// GenerateRandomUfragPwd generates random ufrag and pwd, pion ice agent need them to connect
// a peer will have a unique ufrag and pwd pair.
func GenerateRandomUfragPwd() (string, string) {
	//ufrag := generateRandom(UfragLen)
	//pwd := generateRandom(PwdLen)

	ufrag := "uwBOCX6qe/XEJ29aqFpHL4b1"
	pwd := "Bqi5HXo7FfBGXoMcVvy4H5Fjf7AWHFkv"

	return ufrag, pwd
}
