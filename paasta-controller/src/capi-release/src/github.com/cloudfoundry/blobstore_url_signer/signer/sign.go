package signer

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"strings"
)

type Signer interface {
	Sign(string, string) string
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
	str := fmt.Sprintf("%s/read%s %s", expire, path, s.secret)

	h := md5.New()
	h.Write([]byte(str))

	base64Str := base64.StdEncoding.EncodeToString(h.Sum(nil))
	finalMd5 := SanitizeString(base64Str)

	return fmt.Sprintf("http://blobstore.service.cf.internal/read%s?md5=%s&expires=%s", path, finalMd5, expire)
}

func SanitizeString(input string) string {
	str := strings.Replace(input, "/", "_", -1)
	str = strings.Replace(str, "+", "-", -1)
	str = strings.Replace(str, "=", "", -1)
	return str
}
