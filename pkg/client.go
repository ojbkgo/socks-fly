package pkg

import (
	"fmt"
	"log"
	"net"

	socks5 "github.com/ojbkgo/socks5-protocol"
)

type client struct {
	config     *ClientConfig
	conn       net.Conn
	authMethod socks5.Socks5Method
	logger     *log.Logger
}

type ClientConfig struct {
	RemoteAddr string
	RemotePort int
	Username   string
	Password   string
	AuthMethod socks5.Socks5Method
}

func NewClient(config *ClientConfig) *client {
	return &client{
		config: config,
		logger: log.New(log.Writer(), "", log.LstdFlags),
	}
}

func (c *client) Open() error {
	// dial
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.config.RemoteAddr, c.config.RemotePort))
	if err != nil {
		return err
	}

	c.conn = conn

	// handshake
	if err := c.handshake(); err != nil {
		_ = conn.Close()
		return err
	}

	// auth
	if err := c.authUserPassword(); err != nil {
		log.Printf("auth failed: %v", err)
		_ = conn.Close()
		return err
	}

	return nil
}

func (c *client) Close() {
	_ = c.conn.Close()
}

func (c *client) Send() error {
	return nil
}

func (c *client) Recv() error {
	return nil
}

func (c *client) handshake() error {
	req := &socks5.HandshakeReq{}
	req.Ver = socks5.Socks5Version5
	req.NMethods = 1
	req.Methods = []socks5.Socks5Method{socks5.Socks5MethodUserPass}

	_, err := c.conn.Write(req.Serialize())
	if err != nil {
		return err
	}

	resp := &socks5.HandshakeResp{}
	if err := resp.ReadIO(c.conn); err != nil {
		return err
	}

	fmt.Printf("resp: %v\n", resp)

	if resp.Ver != socks5.Socks5Version5 {
		return fmt.Errorf("socks version not support")
	}

	if resp.Method != socks5.Socks5MethodUserPass {
		return fmt.Errorf("auth method not support")
	}

	c.authMethod = resp.Method

	c.logger.Printf("handshake success, auth method: %d", c.authMethod)

	return nil
}

func (c *client) authUserPassword() error {
	req := &socks5.AuthUserPasswordReq{}
	req.Ver = 0x01
	req.Ulen = uint8(len(c.config.Username))
	req.Uname = c.config.Username
	req.Plen = uint8(len(c.config.Password))
	req.Passwd = c.config.Password

	fmt.Printf("req: %v\n", req)

	err := req.WriteIO(c.conn)
	if err != nil {
		return err
	}

	resp := &socks5.AuthUserPasswordResp{}
	if err := resp.ReadIO(c.conn); err != nil {
		return err
	}

	if resp.Ver != 0x01 {
		return fmt.Errorf("socks version not support")
	}

	if resp.Status != 0 {
		return fmt.Errorf("auth failed")
	}

	c.logger.Printf("auth success, username: %s", c.config.Username)

	return nil
}

func (c *client) ConnectDomain(addr string, port int) error {
	req := &socks5.Socks5CmdRequest{}
	req.Ver = socks5.Socks5Version5
	req.Cmd = socks5.Socks5CmdConnect
	req.Rsv = 0
	req.Atyp = socks5.Socks5AddrTypeDomainName
	req.Addr = socks5.Socks5Addr{
		Addr: addr,
		Port: uint16(port),
	}

	err := req.WriteIO(c.conn)
	if err != nil {
		return err
	}

	resp := &socks5.Socks5CmdResponse{}
	if err := resp.ReadIO(c.conn); err != nil {
		return err
	}

	if resp.Ver != socks5.Socks5Version5 {
		return fmt.Errorf("socks version not support")
	}

	if resp.Rep != socks5.Socks5RepSuccess {
		return fmt.Errorf("connect failed: " + socks5.GetRepMessage(resp.Rep))
	}

	return nil
}

func (c *client) ConnectIPV4(addr string, port int) error {
	req := &socks5.Socks5CmdRequest{}
	req.Ver = socks5.Socks5Version5
	req.Cmd = socks5.Socks5CmdConnect
	req.Rsv = 0
	req.Atyp = socks5.Socks5AddrTypeIPv4
	req.Addr = socks5.Socks5Addr{
		Addr: addr,
		Port: uint16(port),
	}

	err := req.WriteIO(c.conn)
	if err != nil {
		return err
	}

	resp := &socks5.Socks5CmdResponse{}
	if err := resp.ReadIO(c.conn); err != nil {
		return err
	}

	if resp.Ver != socks5.Socks5Version5 {
		return fmt.Errorf("socks version not support")
	}

	if resp.Rep != socks5.Socks5RepSuccess {
		return fmt.Errorf("connect failed: " + socks5.GetRepMessage(resp.Rep))
	}

	return nil
}

func (c *client) udpAssociate() error {
	return nil
}

func (c *client) bind() error {
	return nil
}
