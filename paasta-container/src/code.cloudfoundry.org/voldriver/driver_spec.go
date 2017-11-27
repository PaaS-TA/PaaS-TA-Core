package voldriver

import (
	"os"

	"code.cloudfoundry.org/lager"
)

func WriteDriverSpec(logger lager.Logger, pluginsDirectory string, driver string, extension string, contents []byte) error {
	err := os.MkdirAll(pluginsDirectory, 0666)
	if err != nil {
		logger.Error("error-creating-directory", err)
		return err
	}

	f, err := os.Create(pluginsDirectory + "/" + driver + "." + extension)
	if err != nil {
		logger.Error("error-creating-file ", err)
		return err
	}
	defer f.Close()
	_, err = f.Write(contents)
	if err != nil {
		logger.Error("error-writing-file ", err)
		return err
	}
	f.Sync()
	return nil
}
