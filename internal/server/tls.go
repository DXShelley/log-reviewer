package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TLSConfig uses an explicit certificate pair or a locally persisted self-signed bundle.
func TLSConfig(certFile, keyFile, bundleFile string, names []string) (*tls.Config, error) {
	if (certFile == "") != (keyFile == "") {
		return nil, errors.New("tls-cert and tls-key must be supplied together")
	}
	var cert tls.Certificate
	var err error
	if certFile != "" {
		cert, err = loadTLSCertificate(certFile, keyFile)
	} else {
		names = certificateNames(names)
		cert, err = loadTLSCertificate(bundleFile, bundleFile)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load TLS bundle: %w", err)
		}
		renew := os.IsNotExist(err)
		if err == nil {
			renew = cert.Leaf.NotAfter.Before(time.Now().Add(24 * time.Hour))
			for _, name := range names {
				if cert.Leaf.VerifyHostname(name) != nil {
					renew = true
				}
			}
		}
		if renew {
			if err := createTLSBundle(bundleFile, names); err != nil {
				return nil, err
			}
			cert, err = loadTLSCertificate(bundleFile, bundleFile)
		}
	}
	if err != nil {
		return nil, err
	}
	if time.Now().Before(cert.Leaf.NotBefore) || !time.Now().Before(cert.Leaf.NotAfter) {
		return nil, errors.New("TLS certificate is not currently valid")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
}

func loadTLSCertificate(certFile, keyFile string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err == nil && cert.Leaf == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	}
	return cert, err
}

func certificateNames(names []string) []string {
	unique := map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		if ip, _, err := net.ParseCIDR(address.String()); err == nil && !ip.IsUnspecified() {
			unique[ip.String()] = true
		}
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if ip := net.ParseIP(name); ip != nil && ip.IsUnspecified() {
			continue
		}
		if name != "" {
			unique[name] = true
		}
	}
	var result []string
	for name := range unique {
		result = append(result, name)
	}
	return result
}

func createTLSBundle(path string, names []string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Log Reviewer local HTTPS"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	data := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})...)
	f, err := os.CreateTemp(filepath.Dir(path), ".log-reviewer-tls-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
