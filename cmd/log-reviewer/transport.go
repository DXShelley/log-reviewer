package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"

	"log-reviewer/internal/server"
)

type guiEndpoint struct {
	listener net.Listener
	server   *http.Server
}

func listenGUI(s *server.Server, host string, port int, hosts []string, tlsConfig *tls.Config) ([]guiEndpoint, error) {
	l, srv, err := server.ServeHost(s, host, port, hosts)
	if err != nil && port != 0 {
		fmt.Fprintf(os.Stderr, "Cannot listen on %s:%d (%v); trying an automatically assigned port.\n", host, port, err)
		l, srv, err = server.ServeHost(s, host, 0, hosts)
	}
	if err != nil {
		return nil, err
	}
	listeners := []net.Listener{l}
	_, actualPort, _ := net.SplitHostPort(l.Addr().String())
	if host == "0.0.0.0" {
		ipv6, err := net.Listen("tcp6", net.JoinHostPort("::", actualPort))
		if err != nil {
			fmt.Printf("IPv6 unavailable on port %s: %v\n", actualPort, err)
		} else {
			listeners = append(listeners, ipv6)
		}
	}
	var endpoints []guiEndpoint
	for _, listener := range listeners {
		if tlsConfig != nil {
			listener = tls.NewListener(listener, tlsConfig)
		}
		endpoints = append(endpoints, guiEndpoint{listener: listener, server: srv})
	}
	return endpoints, nil
}
