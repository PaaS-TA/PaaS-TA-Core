package config

import "os"

func SetCreateFile(f func(string) (*os.File, error)) {
	createFile = f
}

func ResetCreateFile() {
	createFile = os.Create
}

func SetSyncFile(f func(*os.File) error) {
	syncFile = f
}

func ResetSyncFile() {
	syncFile = syncFileFn
}
