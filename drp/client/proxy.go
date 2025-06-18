package client

import (
	"context"
	"encoding/json"
	"errors"
	"golang.zx2c4.com/wireguard/conn"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/pkg/log"
	"net/netip"
)

// Proxy will send data to local engine
type Proxy struct {
	logger *log.Logger
	// Address is the address of the proxy server
	Addr          netip.AddrPort
	outBoundQueue chan *drpgrpc.DrpMessage
	inBoundQueue  chan *drpgrpc.DrpMessage
	drpClient     *Client
	offerHandler  internal.OfferHandler
}

type ProxyConfig struct {
	OfferHandler internal.OfferHandler
	DrpClient    *Client
}

func NewProxy(cfg *ProxyConfig) *Proxy {
	return &Proxy{
		outBoundQueue: make(chan *drpgrpc.DrpMessage, 10000),
		inBoundQueue:  make(chan *drpgrpc.DrpMessage, 10000), // Buffered channel to handle messages
		logger:        log.NewLogger(log.Loglevel, "proxy"),
		offerHandler:  cfg.OfferHandler,
		drpClient:     cfg.DrpClient,
	}
}

func (p *Proxy) SetOfferHandler(offerHandler internal.OfferHandler) {
	p.offerHandler = offerHandler
}

// ReceiveMessage receive message from drp server
func (p *Proxy) ReceiveMessage(msg *drpgrpc.DrpMessage) error {
	var err error
	if msg.Body == nil {
		return errors.New("body is nil")
	}
	if err = json.Unmarshal(msg.Body, msg); err != nil {
		return err
	}

	p.logger.Verbosef("receive from signaling service, srcPubKey: %v, dstPubKey: %v", msg.From, msg.To)

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageForwardType:
	case drpgrpc.MessageType_MessageDRPType:
		// write data
		p.inBoundQueue <- msg
	default:
		go func() {
			if err := p.offerHandler.ReceiveOffer(msg); err != nil {
				p.logger.Errorf("handle response failed: %v", err)
			}
		}()
	}

	return nil
}

func (p *Proxy) Receive() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		msg := <-p.inBoundQueue
		// wirte to local engine
		p.logger.Verbosef("msg is: %v", msg)
		sizes[1] = 1
		bufs[0] = msg.Body
		eps[0] = &internal.RemoteEndpoint{
			IsDrp:    true,
			AddrPort: p.Addr,
		}
		return 1, nil
	}

}

// WriteMessage will send actual message to data channel
func (p *Proxy) WriteMessage(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	p.outBoundQueue <- msg
	return nil
}

func (p *Proxy) ReadMessage(ctx context.Context) error {
	return nil
}
