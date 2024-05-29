package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
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

const (
	headerKeyMethod = "_method_"
	headerKeyUrl    = "_url_"
	headerKeyVer    = "_ver_"
)

func (p *httpProxy) parseHttpHeader(header []byte) map[string]string {
	headerMap := make(map[string]string)
	lines := strings.Split(string(header), "\r\n")

	if len(lines) == 0 {
		return headerMap
	}

	// menthod url version
	line0Parts := strings.Split(lines[0], " ")
	if len(line0Parts) < 3 {
		return headerMap
	}

	headerMap[headerKeyMethod] = line0Parts[0]
	headerMap[headerKeyUrl] = line0Parts[1]
	headerMap[headerKeyVer] = line0Parts[2]

	return headerMap
}

const (
	bufferSize      = 64
	headerSplitter  = "\r\n\r\n"
	defaultHttpPort = 80
)

func (p *httpProxy) readHttpHeader(conn net.Conn) ([]byte, map[string]string, error) {
	partBody := make([]byte, 0) // 读取了部分body

	// 循环读取 如果读取到了 \r\n\r\n 说明头部读取完毕
	for {
		buf := make([]byte, bufferSize)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, nil, err
		}

		partBody = append(partBody, buf[:n]...)

		if n < bufferSize || bytes.Contains(buf[:n], []byte(headerSplitter)) {
			break
		}
	}
	splitIndex := bytes.Index(partBody, []byte(headerSplitter))
	if splitIndex == -1 {
		// 用 16进制打印出来， 64字节一行
		count := 0
		for _, b := range partBody {
			fmt.Printf("%.2x ", b)
			count++
			if count%16 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		return nil, nil, errors.New("http header format error")
	}

	header := p.parseHttpHeader(partBody[0:splitIndex])
	return partBody, header, nil
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
				body, header, err := p.readHttpHeader(_conn)
				if err != nil {
					log.Printf("read http header error: %v\n", err)
					return
				}

				log.Printf("http header: %v\n", header)

				if header[headerKeyMethod] == "CONNECT" {
					host := header[headerKeyUrl]
					hsp := strings.Split(host, ":")
					if len(hsp) < 2 {
						_conn.Close()
						log.Printf("host port format error\n")
						return
					}

					port, err := strconv.Atoi(hsp[1])
					if err != nil {
						_conn.Close()
						log.Printf("port format error: %v\n", err)
						return
					}
					host = hsp[0]

					log.Printf("accquire http connect: %s:%d\n", host, port)

					socksCli := NewClient(p.socks5Config)
					if err := socksCli.Open(); err != nil {
						log.Printf("open socks5 client error: %v\n", err)
						p.writeHttpConnect(_conn, 502)
						_conn.Close()
						return
					}

					log.Printf("open socks5 client success\n")

					err = socksCli.ConnectDomain(host, port)
					if err != nil {
						log.Printf("connect domain %s:%d error: %v\n", host, port, err)
						p.writeHttpConnect(_conn, 502)
						_conn.Close()
						return
					}

					log.Printf("connect domain success\n")
					p.writeHttpConnect(_conn, 200)
					log.Printf("start transfer\n")
					p.transfer(_conn, socksCli.conn, p.stopCh)
				} else {
					// http proxy
					log.Printf("http proxy\n")
					host := header[headerKeyUrl]
					parsedUrl, err := url.Parse(host)
					if err != nil {
						log.Printf("parse url error: %v\n", err)
						_conn.Close()
						return
					}

					host = parsedUrl.Host
					port, _ := strconv.Atoi(parsedUrl.Port())
					if port == 0 {
						port = defaultHttpPort
					}

					log.Printf("acquire http proxy: %s:%d\n", host, port)

					socksCli := NewClient(p.socks5Config)
					if err := socksCli.Open(); err != nil {
						log.Printf("open socks5 client error: %v\n", err)
						_conn.Close()
						return
					}

					log.Printf("open socks5 client success\n")
					if err := socksCli.ConnectDomain(host, port); err != nil {
						log.Printf("connect domain error: %v\n", err)
						_conn.Close()
						return
					}

					socksCli.conn.Write(body)
					p.transfer(_conn, socksCli.conn, p.stopCh)
				}
			}(conn)
		}
	}

	return nil
}

func (p *httpProxy) transfer(f, t net.Conn, stopCh chan struct{}) {
	go func() {
		_, err := io.Copy(f, t)
		if err != nil {
			_ = f.Close()
			_ = t.Close()
			return
		}
	}()

	_, err := io.Copy(t, f)
	if err != nil {
		_ = f.Close()
		_ = t.Close()
		return
	}
}
