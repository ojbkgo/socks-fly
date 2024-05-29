package pkg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

type httpProxy struct {
	stopCh   chan struct{}
	listener net.Listener
	// socks client
	socksClient  *client
	socks5Config *ClientConfig
}

func NewHttpProxy(config *ClientConfig) *httpProxy {
	return &httpProxy{
		socksClient:  nil,
		socks5Config: config,
	}
}

func (p *httpProxy) Stop() {
	close(p.stopCh)
	p.listener.Close()
}

func (p *httpProxy) writeHttpConnect(conn net.Conn, status int) error {
	_, err := conn.Write([]byte(fmt.Sprintf("HTTP/1.1 %d Connection established\r\n\r\n", status)))
	if err != nil {
		log.Printf("write http connect error: %v\n", err)
		return err
	}
	return nil
}

func (p *httpProxy) readHttpConnect(conn net.Conn) (string, int, error) {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", 0, err
	}

	// parse connect packet
	lines := strings.Split(string(buf[:n]), "\r\n")
	if len(lines) < 1 {
		return "", 0, errors.New("http connect packet is empty")
	}

	for _, line := range lines {
		fmt.Printf("line: %s\n", line)
	}

	// 解析第一行
	// 格式 CONNECT www.baidu.com:443 HTTP/1.1
	parts := strings.Split(lines[0], " ")
	if len(parts) < 3 {
		return "", 0, errors.New("http connect packet format error")
	}

	// 解析域名和端口
	hostPort := strings.Split(parts[1], ":")
	if len(hostPort) < 2 {
		return "", 0, errors.New("http connect packet host port format error")
	}

	host := hostPort[0]
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}

func (p *httpProxy) printHttpPacket(conn net.Conn) error {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("read error: %v\n", err)
		return err
	}
	log.Printf("http packet:\n %s\n", string(buf[:n]))
	return nil
}

func (p *httpProxy) Start(addr string, ch chan struct{}) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	p.listener = lis
	p.stopCh = ch

	var conn net.TCPConn
	conn.Close()

	for {
		select {
		case <-p.stopCh:
			return nil
		default:
			conn, err := lis.Accept()
			if err != nil {
				log.Printf("accept error: %v\n", err)
				continue
			}

			log.Printf("client connected: %s\n", conn.RemoteAddr().String())

			go func(_conn net.Conn) {
				defer func() {
					log.Printf("client disconnected: %s\n", _conn.RemoteAddr().String())
					_conn.Close()
					if r := recover(); r != nil {
						log.Printf("panic: %v\n", r)
					}
				}()

				host, port, err := p.readHttpConnect(_conn)
				if err != nil {
					log.Printf("read http connect error: %v\n", err)
					return
				}

				log.Printf("accquire http connect: %s:%d\n", host, port)

				socksCli := NewClient(p.socks5Config)
				if err := socksCli.Open(); err != nil {
					log.Printf("open socks5 client error: %v\n", err)
					p.writeHttpConnect(_conn, 502)
					return
				}

				log.Printf("open socks5 client success\n")

				err = socksCli.ConnectDomain(host, port)
				if err != nil {
					log.Printf("connect domain %s:%d error: %v\n", host, port, err)
					p.writeHttpConnect(_conn, 502)
					return
				}

				log.Printf("connect domain success\n")
				p.writeHttpConnect(_conn, 200)
				log.Printf("start transfer\n")
				p.transfer(_conn, socksCli.conn, p.stopCh)

				// handle connection

			}(conn)
		}
	}

	return nil
}

func (p *httpProxy) transfer(f, t net.Conn, stopCh chan struct{}) {
	go func() {
		for {
			select {
			case <-stopCh:
				_ = f.Close()
				_ = t.Close()
				return
			default:
				_, err := io.Copy(f, t)
				if err != nil {
					_ = f.Close()
					_ = t.Close()
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-stopCh:
				_ = f.Close()
				_ = t.Close()
				return
			default:
				_, err := io.Copy(t, f)
				if err != nil {
					_ = f.Close()
					_ = t.Close()
					return
				}
			}
		}
	}()
}
