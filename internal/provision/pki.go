package provision

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type authority struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	pem         []byte
}

func ensureAuthority(dir string) (authority, error) {
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if certErr == nil && keyErr == nil {
		certBlock, _ := pem.Decode(certPEM)
		keyBlock, _ := pem.Decode(keyPEM)
		if certBlock == nil || keyBlock == nil {
			return authority{}, errors.New("invalid provisioning CA")
		}
		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return authority{}, err
		}
		key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			return authority{}, err
		}
		if cert.IsCA == false || time.Now().After(cert.NotAfter) {
			return authority{}, errors.New("provisioning CA is invalid or expired")
		}
		return authority{cert, key, certPEM}, nil
	}
	if os.IsNotExist(certErr) == false || os.IsNotExist(keyErr) == false {
		return authority{}, errors.New("provisioning CA is incomplete or unreadable")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return authority{}, err
	}
	serial, err := certificateSerial()
	if err != nil {
		return authority{}, err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Granger private CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return authority{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return authority{}, err
	}
	if err := writeAtomic(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		return authority{}, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := writeAtomic(certPath, certPEM, 0600); err != nil {
		return authority{}, err
	}
	cert, err := x509.ParseCertificate(der)
	return authority{cert, key, certPEM}, err
}

func ensureCertificate(dir, name, host string, client bool) ([]byte, []byte, error) {
	ca, err := ensureAuthority(dir)
	if err != nil {
		return nil, nil, err
	}
	certPath := filepath.Join(dir, safeName(name)+".crt")
	keyPath := filepath.Join(dir, safeName(name)+".key")
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if certErr == nil && keyErr == nil {
		block, _ := pem.Decode(certPEM)
		if block == nil {
			return nil, nil, errors.New("invalid existing certificate")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		if time.Now().Before(cert.NotAfter.Add(-24 * time.Hour)) {
			if client || cert.VerifyHostname(host) == nil {
				return certPEM, keyPEM, nil
			}
		}
	}
	if certErr != nil && os.IsNotExist(certErr) == false {
		return nil, nil, certErr
	}
	if keyErr != nil && os.IsNotExist(keyErr) == false {
		return nil, nil, keyErr
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := certificateSerial()
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	if client == false {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = []net.IP{ip}
		}
		if net.ParseIP(host) == nil {
			template.DNSNames = []string{host}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := writeAtomic(keyPath, keyPEM, 0600); err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, writeAtomic(certPath, certPEM, 0600)
}

func certificateSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}
