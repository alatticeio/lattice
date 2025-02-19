package probe

import (
	"fmt"
	"linkany/internal/direct"
	"testing"
)

func TestGenerateRandomUfragPwd(t *testing.T) {
	t.Log("TestGenerateRandomUfragPwd")
	ufrag, pwd := GenerateRandomUfragPwd()
	t.Logf("ufrag: %s, pwd: %s", ufrag, pwd)

	offer := &direct.DirectOffer{
		WgPort:    51820,
		Ufrag:     ufrag,
		Pwd:       pwd,
		LocalKey:  321321412,
		Candidate: "3847084945 1 udp 1694498815 223.108.79.98 27900 typ srflx raddr 0.0.0.0 rport 56157",
	}

	_, b, _ := offer.Marshal()
	fmt.Println(b, string(b))

	offer2, err := direct.UnmarshalOfferAnswer(b)
	if err != nil {
		panic(err)
	}
	fmt.Println("offer2:", offer2.WgPort, offer2.Ufrag, offer2.Pwd, offer2.LocalKey, offer2.Candidate)
}
