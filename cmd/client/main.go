package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	socks5 "github.com/ojbkgo/socks5-protocol"

	"github.com/ojbkgo/socks-fly/pkg"
)

// 参数 本地http监听地址
var (
	HTTPAddr   string
	HTTPPort   int
	RemoteAddr string
	RemotePort int
	Username   string
	Password   string
	AuthMethod int

	RemoteConfig = pkg.ClientConfig{}
)

func init() {
	flag.StringVar(&HTTPAddr, "http", "0.0.0.0", "http server listen address")
	flag.IntVar(&HTTPPort, "port", 18080, "http server listen port")
	flag.StringVar(&RemoteAddr, "remote-addr", "127.0.0.1", "remote server address")
	flag.IntVar(&RemotePort, "remote-port", 1080, "remote server port")
	flag.StringVar(&Username, "username", "", "remote server username")
	flag.StringVar(&Password, "password", "", "remote server password")
	flag.IntVar(&AuthMethod, "auth-method", int(socks5.Socks5MethodUserPass), "remote server auth method")
	flag.Parse()
	RemoteConfig = pkg.ClientConfig{
		RemoteAddr: RemoteAddr,
		RemotePort: RemotePort,
		Username:   Username,
		Password:   Password,
		AuthMethod: socks5.Socks5Method(AuthMethod),
	}
}

func main() {

	// check RemoteAddr
	if RemoteAddr == "" {
		log.Printf("remote server address is empty, exit.\n")
		return
	}

	// check RemotePort
	if RemotePort == 0 {
		log.Printf("remote server port is empty, exit.\n")
		return
	}

	// check Username
	if Username == "" {
		log.Printf("remote server username is empty, exit.\n")
		return
	}

	// check Password
	if Password == "" {
		log.Printf("remote server password is empty, exit.\n")
		return
	}

	// check AuthMethod
	if AuthMethod == 0 {
		log.Printf("remote server auth method is empty, exit.\n")
		return
	}

	// listen signal
	sig := make(chan os.Signal, 1)
	osSignal := []os.Signal{os.Interrupt, os.Kill}
	signal.Notify(sig, osSignal...)
	proxy := pkg.NewHttpProxy(&RemoteConfig)
	ch := make(chan struct{})
	go func() {
		if err := proxy.Start(fmt.Sprintf("%s:%d", HTTPAddr, HTTPPort), ch); err != nil {
			panic(err)
		}
	}()

	select {
	case <-sig:
		close(ch)
	}
}
