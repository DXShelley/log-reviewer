package main

import "testing"

func TestAccessURL(t *testing.T) {
	for _, c := range []struct{ host, want string }{
		{"192.168.1.2", "https://192.168.1.2:8765/#abc"},
		{"2001:db8::1", "https://[2001:db8::1]:8765/#abc"},
		{"fe80::1%eth0", "https://[fe80::1%25eth0]:8765/#abc"},
	} {
		if got := accessURL(c.host, "8765", "abc"); got != c.want {
			t.Fatalf("%s != %s", got, c.want)
		}
	}
}

func TestInvalidPort(t *testing.T) {
	for _, port := range []string{"-1", "65536"} {
		if err := gui([]string{"--no-browser", "--port", port}); err == nil {
			t.Fatal("invalid port accepted", port)
		}
		if err := gui([]string{"--no-browser", "--https-port", port}); err == nil {
			t.Fatal("invalid HTTPS port accepted", port)
		}
	}
}
