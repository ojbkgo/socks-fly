package pkg

import (
	"log"
	"net"
)

type httpProxy struct {
	listenAddr string
	stopCh     chan struct{}
	listener   net.Listener
	// socks client
	socksClient *client
}

func NewHttpProxy(listenAddr string) *httpProxy {
	return &httpProxy{
		listenAddr:  listenAddr,
		socksClient: nil,
	}
}

func (p *httpProxy) Stop() {
	close(p.stopCh)
	p.listener.Close()
}

func (p *httpProxy) printHttpPacket(conn net.Conn) error {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("read error: %v\n", err)
		return err
	}
	log.Printf("http packet: %s\n", string(buf[:n]))
	return nil
}

func (p *httpProxy) Start(ch chan struct{}, socksClient *client) error {
	lis, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return err
	}

	p.socksClient = socksClient

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

			go func(_conn net.Conn) {
				defer func() {
					log.Printf("client disconnected: %s\n", _conn.RemoteAddr().String())
					_conn.Close()
					if r := recover(); r != nil {
						log.Printf("panic: %v\n", r)
					}
				}()

				for {
					select {
					case <-p.stopCh:
						return
					default:
						if err := p.printHttpPacket(_conn); err != nil {
							log.Printf("print http packet error: %v\n", err)
							return
						}
					}
				}

				// handle connection

			}(conn)
		}
	}

	return nil
}
