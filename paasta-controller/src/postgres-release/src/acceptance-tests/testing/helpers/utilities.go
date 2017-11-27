package helpers

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
)

func WriteFile(data string) (string, error) {
	tempFile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(tempFile.Name(), []byte(data), os.ModePerm)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}
func GetUUID() string {
	guid := "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

	b := make([]byte, 16)
	_, err := rand.Read(b[:])
	if err == nil {
		guid = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}
	return guid
}
