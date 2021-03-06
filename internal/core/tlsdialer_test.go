package connector

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDial(t *testing.T) {

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("world"))
	})

	cert, err := tls.LoadX509KeyPair("../../server.crt", "../../server.key")
	if err != nil {
		log.Println(err)
		return
	}
	restServer := httptest.NewUnstartedServer(mux)
	connector := NewConnector("localhost:8443", "localhost:8080")
	restServer.Listener = connector

	restServer.StartTLS()

	idleConns := make(chan net.Conn)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err := tls.Listen("tcp", ":8443", config)
		if err != nil {
			log.Println(err)
			return
		}

		defer ln.Close()

		fmt.Println("listening on 8443")
		wg.Done()

		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println(err)
				return
			}

			fmt.Println("accepted incoming conn")
			idleConns <- conn
		}
	}()

	dialTLSContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		go connector.Dial()
		select {
		case <-ctx.Done():
			fmt.Println("timeout")
			return nil, ctx.Err()
		case c := <-idleConns:
			return c, nil
		}
	}
	hc := &http.Client{
		Timeout: 100 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: dialTLSContext,
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				c, err := dialTLSContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}

				// Need to wrap with a tls.Client because StartTLS expects a plain tcp connection from
				// Listener.Accept. Then it performs TLS handshake on top of this
				// raw connection. Since we already return a TLS connection from Listener.Accept,
				// StartTLS will be doing a TLS handshake over a TLS connection.
				// And this is the client end of the outer tls connection.
				// Look at http/server.go, it wraps the Listener with a tls.NewListener
				return tls.Client(c, &tls.Config{
					InsecureSkipVerify: true,
				}), nil
			},
		},
	}

	wg.Wait()
	resp, err := hc.Get("https://localhost:8080/hello")
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.Status)
	fmt.Println(string(body))
}
