package internal

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"
	"os"
	"syscall"
	"wireflow/pkg/log"
)

const (
	utunControlName = "com.apple.net.utun_control"
	utunPrefix      = "utun"
)

func CreateTUN(mtu int, logger *log.Logger) (string, tun.Device, error) {
	ifIndex := 0
	var name string
	var fd int
	var err error
	for {
		if ifIndex > 15 {
			return "", nil, errors.New("create utun device failed")
		}
		name = fmt.Sprintf("%s%d", utunPrefix, ifIndex)

		fd, err = socketCloexec(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2)
		if err != nil {
			return "", nil, err
		}

		ctlInfo := &unix.CtlInfo{}
		copy(ctlInfo.Name[:], []byte(utunControlName))
		err = unix.IoctlCtlInfo(fd, ctlInfo)
		if err != nil {
			unix.Close(fd)
			return "", nil, fmt.Errorf("IoctlGetCtlInfo: %w", err)
		}

		sc := &unix.SockaddrCtl{
			ID:   ctlInfo.Id,
			Unit: uint32(ifIndex) + 1,
		}

		err = unix.Connect(fd, sc)
		if err != nil {
			unix.Close(fd)
			logger.Errorf("connect fd failed: %v, index: %d", err, sc.Unit)
			ifIndex++
			continue
		}

		err = unix.SetNonblock(fd, true)
		if err != nil {
			unix.Close(fd)
			logger.Infof("set non block failed:%v", err)
			ifIndex++
			continue
		}

		break
	}

	err = unix.SetNonblock(int(fd), true)
	if err != nil {
		return "", nil, err
	}

	tun, err := tun.CreateTUNFromFile(os.NewFile(uintptr(fd), ""), mtu)
	if err != nil {
		return "", nil, err
	}
	logger.Verbosef("create tun %s success", name)
	return name, tun, nil
}

func socketCloexec(family, sotype, proto int) (fd int, err error) {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()

	fd, err = unix.Socket(family, sotype, proto)
	return
}
