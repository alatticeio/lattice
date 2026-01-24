package wrrper

import (
	"testing"
)

func TestNewWrrpClient(t *testing.T) {

	//privateKey, err := wgtypes.GeneratePrivateKey()
	//if err != nil {
	//	panic(err)
	//}
	//
	//publicKey := privateKey.PublicKey()
	//
	////sessionId, err := infra.IDFromPublicKey(publicKey.String())
	////if err != nil {
	////	panic(err)
	////}
	//
	//client := NewWrrpClient(sessionId, "127.0.0.1:8080")
	//if err = client.Connect(); err != nil {
	//	panic(err)
	//}
	//go func() {
	//	for {
	//		fn := client.ReceiveFunc()
	//		buf := make([][]byte, 10)
	//		buf[0] = make([]byte, 2048)
	//		ep := make([]conn.Endpoint, 10)
	//
	//		size := make([]int, 10)
	//
	//		n, err := fn(buf, size, ep)
	//		fmt.Println("got size: ", n, err)
	//
	//	}
	//}()
	//
	//for {
	//	time.Sleep(time.Second)
	//	client.Send(context.Background(), sessionId, wrrp.Forward, []byte("hello world"))
	//}
}
