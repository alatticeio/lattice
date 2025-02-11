package drp

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"linkany/internal"
)

// drp is a protocol for relaying packets between two peers, except stun service, drp just forward.
// as all nodes will join to the drp node，what drp do just is auth check and forward.
// Header: 5byte=1 byte for frame type,4 bytes for frame length

// ProtocolVersion is the version of the protocol
const (
	ProtocolVersion = 1
)

const MAX_PACKET_SIZE = 64 << 10

var (
	ErrClientExist = errors.New("client exist")
)

// writeFrame writes a frame header and payload to bw.
func writeFrame(data []byte, b []byte) (int, error) {

	if len(b) > MAX_PACKET_SIZE {
		return 0, fmt.Errorf("unreasonably large frame write")
	}

	copy(data[1:5], b[:4])
	return len(b), nil
}

var bin = binary.BigEndian

func writeUint32(data []byte, frameLen uint32) int {
	var b [4]byte
	bin.PutUint32(b[:], frameLen)
	copy(data[0:4], b[:])
	return 4
}

func ReadFrameHeader(br *bufio.Reader, b []byte) (t internal.FrameType, frameLen uint32, err error) {
	t, frameLen, err = readFrameHeader(br, b)
	if err != nil {
		return 0, 0, err
	}
	return
}

func ReadFrame(br *bufio.Reader, start, end int, b []byte) (n int, err error) {
	n, err = io.ReadFull(br, b[start:end])
	if err != nil {
		return 0, err
	}

	return
}

// ReadKey reads a key from the reader when data is forward data.
func ReadKey(bf *bufio.Reader, fl uint32) (*wgtypes.Key, *wgtypes.Key, []byte, error) {
	var b [4]byte
	_, err := io.ReadFull(bf, b[:])
	if err != nil {
		return nil, nil, nil, err
	}

	srcKey := wgtypes.Key(b[:])

	_, err = io.ReadFull(bf, b[:0])
	if err != nil {
		return nil, nil, nil, err
	}

	dstKey := wgtypes.Key(b[:])

	content := make([]byte, fl)
	_, err = io.ReadFull(bf, content)
	if err != nil {
		return nil, nil, nil, err
	}
	return &srcKey, &dstKey, content, nil
}

func readFrameHeader(br *bufio.Reader, b []byte) (t internal.FrameType, frameLen uint32, err error) {
	_, err = br.Read(b[:1])
	if err != nil {
		return 0, 0, err
	}

	t = internal.FrameType(b[0])

	frameLen, err = readUint32(br, b)
	if err != nil {
		return 0, 0, err
	}
	return
}

func readUint32(br *bufio.Reader, b []byte) (uint32, error) {
	_, err := io.ReadFull(br, b[1:5])
	if err != nil {
		return 0, err
	}

	return bin.Uint32(b[1:5]), nil
}

// DrpReceiveFunc is a function that receives packets from the network when linkany use drp protocol for relay.
func DrpReceiveFunc(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {

	return 0, nil
}

// NodeInfo will send to drp server when use drp,
// server will cache the node info
type NodeInfo struct {
	wgtypes.Key
}
