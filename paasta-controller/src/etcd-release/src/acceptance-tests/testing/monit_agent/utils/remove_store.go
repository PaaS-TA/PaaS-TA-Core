package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

type RemoveStore struct{}

func NewRemoveStore() RemoveStore {
	return RemoveStore{}
}

func (r RemoveStore) DeleteContents(dir string) error {
	dirContents, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, fileInfo := range dirContents {
		err = os.RemoveAll(filepath.Join(dir, fileInfo.Name()))
		if err != nil {
			// Not tested
			return err
		}
	}

	return nil
}
