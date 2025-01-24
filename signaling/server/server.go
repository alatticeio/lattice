package server

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/management/mapper"
	"linkany/pkg/drp"
	"net"
	"net/http"
)

type Server struct {
	*gin.Engine
	listen      string
	userService mapper.UserInterface
	indexTable  *drp.IndexTable
}

type ServerConfig struct {
	Listen      string
	UserService mapper.UserInterface
	Table       *drp.IndexTable
}

func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	return &Server{Engine: e, listen: cfg.Listen, userService: cfg.UserService, indexTable: cfg.Table}
}

func (s *Server) Add(key wgtypes.Key, conn net.Conn, brw *bufio.ReadWriter) error {
	return s.PutClientset(key, conn, brw)
}

// Lookup will return a clientset by key, if not found return nil
func (s *Server) Lookup(key wgtypes.Key) *drp.Clientset {
	s.indexTable.Lock()
	defer s.indexTable.Unlock()
	return s.indexTable.Clients[key.String()]
}

// PutClientset will put a clientset to index table
func (s *Server) PutClientset(key wgtypes.Key, conn net.Conn, brw *bufio.ReadWriter) error {
	s.indexTable.Lock()
	defer s.indexTable.Unlock()
	s.indexTable.Clients[key.String()] = &drp.Clientset{
		PubKey: key,
		Conn:   conn,
		Brw:    brw,
	}
	return nil
}

func (s *Server) initRoute() {
	s.POST("/api/v1/drp", s.upgrade())
}

func (s *Server) Start() error {
	return s.Run(s.listen)
}

func (s *Server) upgrade() gin.HandlerFunc {
	return func(c *gin.Context) {
		klog.Infof("get header upgrade: %v, connection: %v", c.GetHeader("Upgrade"), c.GetHeader("Connection"))
		if c.GetHeader("Upgrade") != "drp" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Upgrade header not set to drp"})
			return
		}
		if c.GetHeader("Connection") != "upgrade" {
			//http.Error(w, "Connection header not set to Upgrade", http.StatusBadRequest)
			c.JSON(http.StatusBadRequest, gin.H{"message": "Connection header not set to Upgrade"})
			return
		}
		h, ok := c.Writer.(http.Hijacker)
		if !ok {
			//http.Error(c.Writer, "server does not support hijacking", http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "server does not support hijacking"})
			return
		}
		conn, brw, err := h.Hijack()
		if err != nil {
			//http.Error(w, "hijack failed", http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "hijack failed"})
			return
		}

		klog.Infof("got connection from %v", conn)

		// write 101 to tell client that upgrade is successful
		fmt.Fprintf(brw.Writer, "HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: DRP\r\n"+
			"Connection: Upgrade\r\n"+
			"Drp-Version: %v\r\n"+
			"Drp-Public-Key: %s\r\n\r\n",
			"v1", "fdsafdxxxx===") //TODO change to real public key
		brw.Flush()
		s.Accept(conn, brw, c.Request.RemoteAddr)
	}
}

func (s *Server) Accept(conn net.Conn, brw *bufio.ReadWriter, remoteAddr string) error {
	//add to indexTable
	return s.accept(conn, brw, remoteAddr)
}

func (s *Server) accept(conn net.Conn, brw *bufio.ReadWriter, remoteAddr string) error {

	for {
		b := make([]byte, 1024)
		ft, fl, err := drp.ReadFrameHeader(brw.Reader, b[:])
		if err != nil {
			if err == io.EOF {
				continue
			} else {
				klog.Errorf("read from remote failed: %v", err)
			}
		}

		n, err := drp.ReadFrame(brw.Reader, 5, int(fl+5), b)
		if err != nil {
			return err
		}

		if n != int(fl) {
			return errors.New("read frame failed")
		}

		klog.Infof("got frame type: %v, frame len: %v, content: %v", ft, fl, b[:])

		switch ft {
		case internal.MessageForwardType:
			// forward message
			// get the key
			srcKey, dstKey, content, err := drp.ReadKey(brw.Reader, fl)
			klog.Infof("forward message from %v to %v, content: %v", srcKey, dstKey, content)
			if err != nil {
				klog.Errorf("invalid frame: %v", err)
				continue
			}

			// get the clientset
			clientset := s.Lookup(*dstKey)
			if clientset != nil {
				klog.Errorf("clientset not found, may be node has not been joined.")
				continue
			}

			n, er := clientset.Brw.Writer.Write(content)
			if n != len(content) || er != nil {
				klog.Errorf("write to clientset failed: %v", er)
				continue
			}

		case internal.MessageDirectOfferType, internal.MessageRelayOfferType, internal.MessageRelayOfferResponseType:
			klog.Infof("got offer info packet: %v, length: %d", b[:], fl)
			srcKey := wgtypes.Key(b[5:37])
			klog.Infof("srcKey: %s", srcKey.String())
			if indexConn := s.Lookup(srcKey); indexConn == nil || indexConn.Conn != conn {
				klog.Infof("add or update conn to index table: %v, conn: %v", srcKey.String(), conn.LocalAddr().String())
				s.Add(srcKey, conn, brw)
			}

			dstKey := wgtypes.Key(b[37:69])
			clientset := s.Lookup(dstKey)
			if clientset == nil {
				klog.Errorf("dst node not found: %v", dstKey)
				continue
			}

			klog.Infof("clientset is: %v", clientset)
			// forward to dst node
			if _, err := clientset.Brw.Write(b[:fl+5]); err != nil {
				klog.Errorf("forward to dst node failed: %v", err)
				continue
			}

			if err := clientset.Brw.Flush(); err != nil {
				klog.Errorf("flush error", err)
			}

			klog.Infof("forward offer to dst node success: %v, content: %v", dstKey, b[:fl+5])
		}

	}
}
