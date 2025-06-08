package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"linkany/pkg/config"
	"os"
	"testing"
)

func TestClient_Login(t *testing.T) {

	user := config.User{
		Username: "linkany",
		Password: "123456",
		Token:    "",
	}
	conf, _ := config.GetLocalConfig()
	client := NewClient(&ClientConfig{
		Conf: conf,
	})
	err := client.Login(&user)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_Get(t *testing.T) {

	// controlclient
	//mgtclient, err := mgtclient.NewClient(&mgtclient.GrpcConfig{Addr: "console.linkany.io:32051", Logger: log.NewLogger(log.Loglevel, fmt.Sprintf("%s", "grpcclient"))})
	//if err != nil {
	//	t.Fatal(err)
	//}
	//conf, _ := config.GetLocalConfig()
	//client := NewClient(&ClientConfig{
	//	Conf:       conf,
	//	GrpcClient: mgtclient,
	//})
	//peer, err := client.Get(context.Background())
	//if err != nil {
	//	t.Fatal(err)
	//}
	//fmt.Println(peer)
}

func TestFetchPeers(t *testing.T) {
	bs, err := os.ReadFile("test.json")
	if err != nil {
		t.Fatal(err)
	}

	type records struct {
		Records []config.Node `json:"records,omitempty"`
	}

	var resp HttpResponse[records]
	err = json.Unmarshal(bs, &resp)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(resp)
}

func TestParse(t *testing.T) {
	bs, err := os.ReadFile("test.json")
	if err != nil {
		t.Fatal(err)
	}

	type records struct {
		Records []config.Node `json:"records,omitempty"`
	}

	var resp HttpResponse[records]
	err = json.Unmarshal(bs, &resp)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(resp)

}

func TestKey(t *testing.T) {
	k, err := wgtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(k.String())
	dst := hex.EncodeToString(k[:])
	fmt.Println(dst)

	src, err := hex.DecodeString(dst)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(src)
}
