package commands

import (
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
)

func Register(client routing_api.Client, routes []models.Route) error {
	return client.UpsertRoutes(routes)
}
