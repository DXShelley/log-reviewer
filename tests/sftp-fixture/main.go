package main

import (
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"log-reviewer/internal/testsupport/sftpfixture"
)

func main() {
	root, err := os.MkdirTemp("", "lr-sftp-fixture-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	os.MkdirAll(filepath.Join(root, "logs", "nested"), 0700)
	os.WriteFile(filepath.Join(root, "logs", "app.log"), []byte("2026-10-14 12:30:00 ERROR java.lang.IllegalStateException: remote failed\n\tat com.example.App.run(App.java:42)\n"), 0600)
	os.WriteFile(filepath.Join(root, "logs", "ignored.csv"), []byte("ignored"), 0600)
	os.WriteFile(filepath.Join(root, "logs", "nested", "worker.log"), []byte("2026-10-14 12:30:01 WARN remote worker warning\n"), 0600)
	server, err := sftpfixture.Start(root)
	if err != nil {
		panic(err)
	}
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	json.NewEncoder(os.Stdout).Encode(map[string]any{"host": "127.0.0.1", "port": number, "user": "tester", "password": "fixture-password", "fingerprint": server.Fingerprint, "private_key": server.PrivateKey, "root": root, "path": "logs"})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}
