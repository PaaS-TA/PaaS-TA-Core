// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package systemcerts

import (
	"crypto/x509"
	"encoding/pem"
)

// CertPool is a set of certificates.
type CertPool struct {
	bySubjectKeyId map[string][]int
	byName         map[string][]int
	certs          []*Certificate
}

// NewCertPool returns a new, empty CertPool.
func NewCertPool() *CertPool {
	return &CertPool{
		make(map[string][]int),
		make(map[string][]int),
		nil,
	}
}

func (c *Certificate) AsX509Certificate() *x509.Certificate {
	return &x509.Certificate{
		Raw:                         c.Raw,
		RawTBSCertificate:           c.RawTBSCertificate,
		RawSubjectPublicKeyInfo:     c.RawSubjectPublicKeyInfo,
		RawSubject:                  c.RawSubject,
		RawIssuer:                   c.RawIssuer,
		Signature:                   c.Signature,
		SignatureAlgorithm:          c.SignatureAlgorithm,
		PublicKeyAlgorithm:          c.PublicKeyAlgorithm,
		PublicKey:                   c.PublicKey,
		Version:                     c.Version,
		SerialNumber:                c.SerialNumber,
		Issuer:                      c.Issuer,
		Subject:                     c.Subject,
		NotBefore:                   c.NotBefore,
		NotAfter:                    c.NotAfter,
		KeyUsage:                    c.KeyUsage,
		Extensions:                  c.Extensions,
		ExtraExtensions:             c.ExtraExtensions,
		UnhandledCriticalExtensions: c.UnhandledCriticalExtensions,
		ExtKeyUsage:                 c.ExtKeyUsage,
		UnknownExtKeyUsage:          c.UnknownExtKeyUsage,
		BasicConstraintsValid:       c.BasicConstraintsValid,
		IsCA:                        c.IsCA,
		MaxPathLen:                  c.MaxPathLen,
		MaxPathLenZero:              c.MaxPathLenZero,
		SubjectKeyId:                c.SubjectKeyId,
		AuthorityKeyId:              c.AuthorityKeyId,
		OCSPServer:                  c.OCSPServer,
		IssuingCertificateURL:       c.IssuingCertificateURL,
		DNSNames:                    c.DNSNames,
		EmailAddresses:              c.EmailAddresses,
		IPAddresses:                 c.IPAddresses,
		PermittedDNSDomainsCritical: c.PermittedDNSDomainsCritical,
		PermittedDNSDomains:         c.PermittedDNSDomains,
		CRLDistributionPoints:       c.CRLDistributionPoints,
		PolicyIdentifiers:           c.PolicyIdentifiers,
	}
}

func (s *CertPool) AsX509CertPool() *x509.CertPool {
	newCertPool := x509.NewCertPool()

	for _, oldCert := range s.certs {
		newCertPool.AddCert(oldCert.AsX509Certificate())
	}

	return newCertPool
}

// findVerifiedParents attempts to find certificates in s which have signed the
// given certificate. If any candidates were rejected then errCert will be set
// to one of them, arbitrarily, and err will contain the reason that it was
// rejected.
func (s *CertPool) findVerifiedParents(cert *Certificate) (parents []int, errCert *Certificate, err error) {
	if s == nil {
		return
	}
	var candidates []int

	if len(cert.AuthorityKeyId) > 0 {
		candidates = s.bySubjectKeyId[string(cert.AuthorityKeyId)]
	}
	if len(candidates) == 0 {
		candidates = s.byName[string(cert.RawIssuer)]
	}

	for _, c := range candidates {
		if err = cert.CheckSignatureFrom(s.certs[c]); err == nil {
			parents = append(parents, c)
		} else {
			errCert = s.certs[c]
		}
	}

	return
}

// AddCert adds a certificate to a pool.
func (s *CertPool) AddCert(cert *Certificate) {
	if cert == nil {
		panic("adding nil Certificate to CertPool")
	}

	// Check that the certificate isn't being added twice.
	for _, c := range s.certs {
		if c.Equal(cert) {
			return
		}
	}

	n := len(s.certs)
	s.certs = append(s.certs, cert)

	if len(cert.SubjectKeyId) > 0 {
		keyId := string(cert.SubjectKeyId)
		s.bySubjectKeyId[keyId] = append(s.bySubjectKeyId[keyId], n)
	}
	name := string(cert.RawSubject)
	s.byName[name] = append(s.byName[name], n)
}

// AppendCertsFromPEM attempts to parse a series of PEM encoded certificates.
// It appends any certificates found to s and reports whether any certificates
// were successfully parsed.
//
// On many Linux systems, /etc/ssl/cert.pem will contain the system wide set
// of root CAs in a format suitable for this function.
func (s *CertPool) AppendCertsFromPEM(pemCerts []byte) (ok bool) {
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		s.AddCert(cert)
		ok = true
	}

	return
}

// Subjects returns a list of the DER-encoded subjects of
// all of the certificates in the pool.
func (s *CertPool) Subjects() (res [][]byte) {
	res = make([][]byte, len(s.certs))
	for i, c := range s.certs {
		res[i] = c.RawSubject
	}
	return
}
