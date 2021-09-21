package connector

import (
	"crypto/tls"
	"fmt"
	"net"
)

type DialerListener interface {
	net.Listener
	Dial() error
}

type TlsDialerListener struct {
	CrankerAddr string
	ServiceAddr string
	Config      *tls.Config
	connections chan *tls.Conn
}

func NewConnector(crankerAddr, serviceAddr string) DialerListener {
	return &TlsDialerListener{
		CrankerAddr: crankerAddr,
		ServiceAddr: serviceAddr,
		Config: &tls.Config{
			InsecureSkipVerify: true,
		},
		connections: make(chan *tls.Conn),
	}
}

func (d *TlsDialerListener) Dial() error {
	fmt.Println("dialing")
	conn, err := tls.Dial("tcp", d.CrankerAddr, d.Config)
	if err != nil {
		return err
	}

	fmt.Println("connected")
	d.connections <- conn

	return nil
}

func (d *TlsDialerListener) Accept() (net.Conn, error) {
	c := <-d.connections

	fmt.Println("accepted")
	return c, nil
}

func (d *TlsDialerListener) Close() error {
	close(d.connections)

	return nil
}

func (d *TlsDialerListener) Addr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", d.ServiceAddr)

	return addr
}
