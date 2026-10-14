package sftpfixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Server is a loopback-only fixture for exercising actual SSH and SFTP traffic.
type Server struct {
	Listener     net.Listener
	Fingerprint  string
	PrivateKey   string
	EncryptedKey string
	HostKey      ssh.PublicKey
	mu           sync.Mutex
	connections  map[net.Conn]bool
	done         chan struct{}
	jobs         sync.WaitGroup
}

func Start(root string) (*Server, error) {
	_, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(hostPrivate)
	if err != nil {
		return nil, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(private, "fixture")
	if err != nil {
		return nil, err
	}
	encrypted, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", []byte("fixture-passphrase"))
	if err != nil {
		return nil, err
	}
	accepted, err := ssh.NewPublicKey(public)
	if err != nil {
		return nil, err
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if c.User() == "tester" && string(password) == "fixture-password" {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication rejected")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() == "tester" && bytes.Equal(key.Marshal(), accepted.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication rejected")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &Server{Listener: listener, Fingerprint: ssh.FingerprintSHA256(signer.PublicKey()), PrivateKey: string(pem.EncodeToMemory(block)), EncryptedKey: string(pem.EncodeToMemory(encrypted)), HostKey: signer.PublicKey(), connections: map[net.Conn]bool{}, done: make(chan struct{})}
	go func() {
		defer close(server.done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			server.mu.Lock()
			server.connections[conn] = true
			server.jobs.Add(1)
			server.mu.Unlock()
			go server.serve(conn, config, root)
		}
	}()
	return server, nil
}
func (s *Server) serve(conn net.Conn, config *ssh.ServerConfig, root string) {
	defer s.jobs.Done()
	defer conn.Close()
	defer func() { s.mu.Lock(); delete(s.connections, conn); s.mu.Unlock() }()
	transport, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer transport.Close()
	go ssh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			channel.Reject(ssh.UnknownChannelType, "session required")
			continue
		}
		stream, requests, err := channel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer stream.Close()
			for request := range requests {
				var payload struct{ Name string }
				if request.Type != "subsystem" || ssh.Unmarshal(request.Payload, &payload) != nil || payload.Name != "sftp" {
					request.Reply(false, nil)
					continue
				}
				request.Reply(true, nil)
				subsystem, err := sftp.NewServer(stream, sftp.WithServerWorkingDirectory(root))
				if err != nil {
					return
				}
				subsystem.Serve()
				subsystem.Close()
				return
			}
		}()
	}
}
func (s *Server) Close() {
	s.Listener.Close()
	<-s.done
	s.mu.Lock()
	for conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()
	s.jobs.Wait()
}
