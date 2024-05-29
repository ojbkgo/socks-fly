package main

import (
	"flag"
	"os"
	"os/signal"

	socks5 "github.com/ojbkgo/socks5-protocol"

	"github.com/ojbkgo/socks-fly/pkg"
)

// 服务端启动
// 参数: 监听地址 监听端口 鉴权用户名 鉴权密码

var listenAddr string
var listenPort int
var authUser string
var authPass string

// init
func init() {
	flag.StringVar(&listenAddr, "listen", "0.0.0.0", "socks5 server listen address")
	flag.IntVar(&listenPort, "port", 1080, "socks5 server listen port")
	flag.StringVar(&authUser, "user", "admin", "socks5 server auth user")
	flag.StringVar(&authPass, "pass", "admin", "socks5 server auth pass")
}

func main() {
	flag.Parse()
	// 创建一个新的socks服务器
	server := pkg.NewServer(&pkg.ServerConfig{
		AuthMethod: socks5.Socks5MethodUserPass,
		User:       authUser,
		Password:   authPass,
		Mode:       pkg.ServerMode_Socks,
		Addr:       listenAddr,
		Port:       listenPort,
	})
	// signal
	osSignal := make(chan os.Signal, 1)
	// 监听信号
	signal.Notify(osSignal, os.Interrupt, os.Kill)
	go func() {
		select {
		case <-osSignal:
			server.Stop()
			return
		}
	}()

	if err := server.Serve(); err != nil {
		panic(err)
	}
}
