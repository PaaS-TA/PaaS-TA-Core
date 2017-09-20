package chaperon

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"
)

type KeyringRemover struct {
	path   string
	logger logger
}

func NewKeyringRemover(path string, logger logger) KeyringRemover {
	return KeyringRemover{
		path:   path,
		logger: logger,
	}
}

func (r KeyringRemover) Execute() error {
	r.logger.Info("keyring-remover.execute", lager.Data{
		"keyring": r.path,
	})

	if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
		err = errors.New(err.Error())
		r.logger.Error("keyring-remover.execute.failed", err, lager.Data{
			"keyring": r.path,
		})

		return err
	}

	r.logger.Info("keyring-remover.execute.success", lager.Data{
		"keyring": r.path,
	})

	return nil
}
