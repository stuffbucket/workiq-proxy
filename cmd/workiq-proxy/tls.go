package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// tlsCertPaths returns the file paths for the persisted self-signed
// cert and key inside the XDG state directory.
func tlsCertPaths() (certPath, keyPath string) {
	d := stateDir()
	if d == "" {
		return "", ""
	}
	return filepath.Join(d, "localhost.crt"), filepath.Join(d, "localhost.key")
}

// loadOrGenerateTLSConfig returns a tls.Config with a self-signed
// certificate for localhost / 127.0.0.1. It persists the cert in the
// XDG state directory so the user only needs to trust it once.
func loadOrGenerateTLSConfig() (*tls.Config, error) {
	certPath, keyPath := tlsCertPaths()

	// Try loading an existing cert.
	if certPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			if leaf, parseErr := x509.ParseCertificate(cert.Certificate[0]); parseErr == nil {
				if time.Until(leaf.NotAfter) > 24*time.Hour {
					return makeTLSConfig(cert), nil
				}
			}
		}
	}

	// Generate a new ECDSA P-256 key.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate TLS key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "workiq-proxy localhost"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	// Persist to state directory.
	if certPath != "" {
		_ = os.MkdirAll(filepath.Dir(certPath), 0o700)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		keyDER, _ := x509.MarshalECPrivateKey(key)
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
		_ = os.WriteFile(certPath, certPEM, 0o600)
		_ = os.WriteFile(keyPath, keyPEM, 0o600)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
	return makeTLSConfig(cert), nil
}

func makeTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}
