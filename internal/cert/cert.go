package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// LinkerdCerts contains all certificates and keys needed for Linkerd installation
type LinkerdCerts struct {
	TrustAnchorsPEM []byte // CA certificate
	IssuerCertPEM   []byte // Issuer certificate
	IssuerKeyPEM    []byte // Issuer private key
}

// GenerateLinkerdCerts generates all necessary certificates for Linkerd installation
// It returns a LinkerdCerts struct containing the root CA certificate, issuer certificate, and issuer private key
// The validity parameter specifies the duration for which the issuer certificate is valid
func GenerateLinkerdCerts(validity time.Duration) (*LinkerdCerts, error) {
	// Generate root CA
	rootKey, rootCert, err := createRootCA("root.linkerd.cluster.local", 87600*time.Hour) // 10 years
	if err != nil {
		return nil, fmt.Errorf("error creating root CA: %w", err)
	}

	// Generate issuer CA
	issuerKey, issuerCert, err := createIssuerCA("identity.linkerd.cluster.local", rootCert, rootKey, validity)
	if err != nil {
		return nil, fmt.Errorf("error creating issuer CA: %w", err)
	}

	// Encode issuer private key in PEM format
	issuerEcKey, ok := issuerKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("issuer key is not an ECDSA private key")
	}

	keyBytes, err := x509.MarshalECPrivateKey(issuerEcKey)
	if err != nil {
		return nil, err
	}

	issuerKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return &LinkerdCerts{
		TrustAnchorsPEM: rootCert,
		IssuerCertPEM:   issuerCert,
		IssuerKeyPEM:    issuerKeyPEM,
	}, nil
}

// createRootCA generates a root CA certificate and private key
// The cn parameter specifies the common name for the root CA
// The validity parameter specifies the duration for which the root CA certificate is valid
// It returns the private key, the root CA certificate in PEM format, and an error if any
func createRootCA(cn string, validity time.Duration) (crypto.PrivateKey, []byte, error) {
	// Generate EC P-256 key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// Prepare certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(validity)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
	}

	// Self-sign the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate in PEM format
	cert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	return key, cert, nil
}

// createIssuerCA generates an issuer CA certificate and private key signed by the root CA
// The cn parameter specifies the common name for the issuer CA
// The rootCertPEM parameter is the root CA certificate in PEM format
// The rootKey parameter is the root CA private key
// The validity parameter specifies the duration for which the issuer CA certificate is valid
// It returns the private key, the issuer CA certificate in PEM format, and an error if any
func createIssuerCA(cn string, rootCertPEM []byte, rootKey crypto.PrivateKey, validity time.Duration) (crypto.PrivateKey, []byte, error) {
	// Parse root certificate
	block, _ := pem.Decode(rootCertPEM)
	rootCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	// Generate issuer key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// Prepare certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(validity)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	// Sign with root CA (use rootCert as the parent)
	derBytes, err := x509.CreateCertificate(rand.Reader, template, rootCert, &key.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate in PEM format
	cert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	return key, cert, nil
}
