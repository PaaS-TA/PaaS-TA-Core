package commands

import (
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
)

func List(client routing_api.Client) ([]models.Route, error) {
	return client.Routes()
}
