package signer

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"strings"
)

//go:generate counterfeiter -o fakes/fake_signer.go . Signer
type Signer interface {
	Sign(string, string) string
	SignForPut(string, string) string
}

type signer struct {
	secret string
}

func NewSigner(secret string) Signer {
	return &signer{
		secret: secret,
	}
}

func (s *signer) Sign(expire, path string) string {
	signature := generateSignature(fmt.Sprintf("%s/read%s %s", expire, path, s.secret))
	return fmt.Sprintf("http://blobstore.service.cf.internal/read%s?md5=%s&expires=%s", path, signature, expire)
}

func (s *signer) SignForPut(expire, path string) string {
	signature := generateSignature(fmt.Sprintf("%s/write%s %s", expire, path, s.secret))
	return fmt.Sprintf("http://blobstore.service.cf.internal/write%s?md5=%s&expires=%s", path, signature, expire)
}

func generateSignature(str string) string {
	md5sum := md5.Sum([]byte(str))
	return SanitizeString(base64.StdEncoding.EncodeToString(md5sum[:]))
}

func SanitizeString(input string) string {
	str := strings.Replace(input, "/", "_", -1)
	str = strings.Replace(str, "+", "-", -1)
	str = strings.Replace(str, "=", "", -1)
	return str
}
