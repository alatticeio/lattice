package client

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/pkg/config"
	"net/http"
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
	fmt.Println(u)
}

func TestFetchPeers(t *testing.T) {
	bs, err := os.ReadFile("test.json")
	if err != nil {
		t.Fatal(err)
	}

	type records struct {
		Records []config.Peer `json:"records,omitempty"`
	}

	var resp HttpResponse[records]
	err = json.Unmarshal(bs, &resp)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(resp)
}

func TestClient_Register(t *testing.T) {
	conf, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	cli := NewClient(&ClientConfig{
		Conf: conf,
	})

	c := cli.(*Client)

	hostname, err := os.Hostname()
	if err != nil {
		klog.Errorf("get hostname failed: %v", err)
	}

	peer := &config.PeerRegisterInfo{
		AppId:    c.conf.AppId,
		Hostname: hostname,
	}

	jsonStr, err := json.Marshal(peer)
	if err != nil {
		klog.Errorf("marshal peer failed: %v", err)
	}

	data := bytes.NewBuffer(jsonStr)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/peer/register", internal.ConsoleDomain), data)
	request.Header.Add("TOKEN", c.conf.Token)
	resp, err := c.httpClient.Do(request)

	fmt.Println(resp.StatusCode, err)
	//fmt.Println(response.StatusCode)

}

func TestParse(t *testing.T) {
	bs, err := os.ReadFile("test.json")
	if err != nil {
		t.Fatal(err)
	}

	type records struct {
		Records []config.Peer `json:"records,omitempty"`
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
