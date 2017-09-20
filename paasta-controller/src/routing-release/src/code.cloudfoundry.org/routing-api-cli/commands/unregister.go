package commands

import (
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
)

func UnRegister(client routing_api.Client, routes []models.Route) error {
	return client.DeleteRoutes(routes)
}
