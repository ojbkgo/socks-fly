package pkg

import (
	"fmt"
	"io"
	"net"
	"time"

	socks5 "github.com/ojbkgo/socks5-protocol"
)

type serverCmdConnect struct {
	cliConn    net.Conn
	cmd        *socks5.Socks5CmdRequest
	remoteConn net.Conn
	stopCh     chan struct{}
}

func (s *serverCmdConnect) response(rep socks5.Socks5Rep) error {
	resp := &socks5.Socks5CmdResponse{
		Ver:  socks5.Socks5Version5,
		Rep:  rep,
		Rsv:  0,
		Atyp: s.cmd.Atyp,
	}

	_, err := s.cliConn.Write(resp.Serialize())
	return err
}

func (s *serverCmdConnect) connectRemote() error {
	switch s.cmd.Atyp {
	case socks5.Socks5AddrTypeIPv4:
		remoteConn, err := net.DialTimeout(
			"tcp",
			fmt.Sprintf("%s:%d", s.cmd.Addr.Addr, s.cmd.Addr.Port),
			time.Second*5)

		if err != nil {
			return err
		}

		s.remoteConn = remoteConn
	case socks5.Socks5AddrTypeIPv6:
		// unsupported
	case socks5.Socks5AddrTypeDomainName:
		remoteConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", s.cmd.Addr.Addr, s.cmd.Addr.Port))
		if err != nil {
			return err
		}

		s.remoteConn = remoteConn
	}

	return nil
}

func (s *serverCmdConnect) close() {
	if s.cliConn != nil {
		_ = s.cliConn.Close()
	}

	if s.remoteConn != nil {
		_ = s.remoteConn.Close()
	}
}

// run transfer
func (s *serverCmdConnect) run() {
	go func() {
		for {
			select {
			case <-s.stopCh:
				s.close()
				return
			default:
				_, err := io.Copy(s.cliConn, s.remoteConn)
				if err != nil {
					s.close()
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-s.stopCh:
				s.close()
				return
			default:
				_, err := io.Copy(s.remoteConn, s.cliConn)
				if err != nil {
					s.close()
					return
				}
			}
		}
	}()
}
