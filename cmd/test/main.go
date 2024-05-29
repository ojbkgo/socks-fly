package main

// 参数： 监听端口 监听地址

import (
	"flag"

	socks5 "github.com/ojbkgo/socks5-protocol"
)

var listenAddr string

// init
func init() {
	flag.StringVar(&listenAddr, "listen", "0.0.0.0:1080", "socks5 server listen address")
}

func main() {
	flag.Parse()
	// 创建一个新的socks服务器

	shakeReq := &socks5.HandshakeReq{}
	shakeReq.Ver = socks5.Socks5Version5
	shakeReq.NMethods = 1
	shakeReq.Methods = []socks5.Socks5Method{socks5.Socks5MethodNoAuth}

}
