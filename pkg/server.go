package pkg

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	socks5 "github.com/ojbkgo/socks5-protocol"
)

type ServerMode uint8

const (
	ServerMode_Socks ServerMode = iota + 1
	ServerMode_UDP
)

type ServerConfig struct {
	AuthMethod socks5.Socks5Method
	User       string
	Password   string
	Mode       ServerMode
	Addr       string
	Port       int
}

type server struct {
	config      *ServerConfig
	stopCh      chan struct{}
	clientConns map[string]*net.TCPConn
	logger      *log.Logger
	listener    net.Listener
}

func NewServer(config *ServerConfig) *server {
	return &server{
		config:      config,
		stopCh:      make(chan struct{}),
		clientConns: make(map[string]*net.TCPConn),
		logger:      log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (s *server) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.config.Addr, s.config.Port)
	s.logger.Printf("socks5 server listen on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = l

	defer l.Close()

	for {
		select {
		case <-s.stopCh:
			// 关闭
			for _, conn := range s.clientConns {
				_ = conn.Close()
			}
			s.logger.Printf("server stop")
			return nil
		default:
			s.logger.Printf("server running, waiting for client")
			conn, err := l.Accept()
			if err != nil {
				continue
			}
			s.logger.Printf("client connected: %s", conn.RemoteAddr().String())

			go func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Printf("panic: %v", r)
					}
				}()

				method, err := s.handshake(conn)
				if err != nil {
					s.logger.Printf("handshake error: %v", err)
					_ = conn.Close()
					return
				}

				if method == socks5.Socks5MethodUserPass {
					if err := s.authUserPassword(conn); err != nil {
						s.logger.Printf("auth error: %v", err)
						return
					}
				}

				if err := s.cmdExec(conn); err != nil {
					s.logger.Printf("cmd exec error: %v", err)
					return
				}
			}()
		}
	}

	return nil
}

func (s *server) handshake(conn net.Conn) (socks5.Socks5Method, error) {
	req := &socks5.HandshakeReq{}
	if err := req.ReadIO(conn); err != nil {
		return 0, err
	}

	if req.Ver != socks5.Socks5Version5 {
		_ = conn.Close()
		return 0, errors.New("socks version not support")
	}
	if req.NMethods == 0 {
		_ = conn.Close()
		return 0, errors.New("no auth method")
	}

	var support bool
	for _, m := range req.Methods {
		if m == s.config.AuthMethod {
			support = true
			break
		}
	}

	if !support {
		_ = conn.Close()
		return 0, errors.New("auth method not support")
	}
	rsp := &socks5.HandshakeResp{}
	rsp.Ver = socks5.Socks5Version5
	rsp.Method = s.config.AuthMethod

	if err := rsp.WriteIO(conn); err != nil {
		_ = conn.Close()
		return 0, err
	}

	s.logger.Printf("handshake success, auth method: %d", s.config.AuthMethod)

	return s.config.AuthMethod, nil
}

func (s *server) authUserPassword(conn net.Conn) error {
	req := &socks5.AuthUserPasswordReq{}
	if err := req.ReadIO(conn); err != nil {
		return err
	}

	if s.config.User != req.Uname || s.config.Password != req.Passwd {
		_ = conn.Close()
		return errors.New("username or password error")
	}

	rsp := &socks5.AuthUserPasswordResp{}
	rsp.Ver = socks5.Socks5Version5
	rsp.Status = 0
	if err := rsp.WriteIO(conn); err != nil {
		_ = conn.Close()
		return err
	}

	s.logger.Printf("auth success, username: %s", s.config.User)
	return nil
}

func (s *server) cmdExec(conn net.Conn) error {
	req := &socks5.Socks5CmdRequest{}
	if err := req.ReadIO(conn); err != nil {
		return err
	}

	if req.Ver != socks5.Socks5Version5 {
		_ = conn.Close()
		return errors.New("socks version not support")
	}

	switch req.Cmd {
	case socks5.Socks5CmdConnect:
		cmd := &serverCmdConnect{
			cliConn: conn,
			cmd:     req,
			stopCh:  s.stopCh,
		}

		if err := cmd.connectRemote(); err != nil {
			_ = conn.Close()
			return err
		}

		cmd.run()

		if err := cmd.response(); err != nil {
			cmd.close()
			return err
		}
	case socks5.Socks5CmdBind:
		// bind
	case socks5.Socks5CmdUdpAssociate:
		// udp associate
	}
	return nil
}

func (s *server) Stop() error {
	close(s.stopCh)
	_ = s.listener.Close()
	return nil
}
