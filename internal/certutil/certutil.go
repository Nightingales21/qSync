// Package certutil generates a self-signed TLS certificate for local QUIC testing.
package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

// GenerateTLSConfig creates an in-memory self-signed certificate and returns
// a *tls.Config suitable for use as a QUIC server's TLS configuration.
// nextProtos should match the ALPN identifier used by the client (e.g. "qsync").
func GenerateTLSConfig(nextProtos ...string) (*tls.Config, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"qSync"}, CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}

	if len(nextProtos) == 0 {
		nextProtos = []string{"qsync"}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   nextProtos,
	}, nil
}

// ClientTLSConfig returns a *tls.Config for the client that trusts any server
// certificate (fine for local/dev use over a private network) and advertises
// the given ALPN protocols.
func ClientTLSConfig(nextProtos ...string) *tls.Config {
	if len(nextProtos) == 0 {
		nextProtos = []string{"qsync"}
	}
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         nextProtos,
	}
}
