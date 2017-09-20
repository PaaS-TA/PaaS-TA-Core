package migration

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type V1EtcdMigration struct {
	etcdCfg *config.Etcd
	done    chan struct{}
	logger  lager.Logger
}

var _ Migration = new(V1EtcdMigration)

func NewV1EtcdMigration(etcdCfg *config.Etcd, done chan struct{}, logger lager.Logger) *V1EtcdMigration {
	return &V1EtcdMigration{etcdCfg: etcdCfg, done: done, logger: logger}
}

func (v *V1EtcdMigration) Version() int {
	return 1
}

func (v *V1EtcdMigration) Run(sqlDB *db.SqlDB) error {
	if sqlDB == nil {
		return nil
	}

	if len(v.etcdCfg.NodeURLS) == 0 {
		return nil
	}

	etcd, err := db.NewETCD(v.etcdCfg)
	if err != nil {
		return err
	}

	etcdRouterGroups, err := etcd.ReadRouterGroups()
	if err != nil {
		return err
	}
	for _, rg := range etcdRouterGroups {
		routerGroup := models.NewRouterGroupDB(rg)
		_, err := sqlDB.Client.Create(&routerGroup)
		if err != nil {
			return err
		}
	}

	v.watchForRouterGroupChanges(etcd, sqlDB)

	etcdRoutes, err := etcd.ReadRoutes()
	if err != nil {
		return err
	}
	for _, route := range etcdRoutes {
		r, err := models.NewRouteWithModel(route)
		if err != nil {
			return err
		}
		r.ExpiresAt = route.ExpiresAt

		_, err = sqlDB.Client.Create(&r)
		if err != nil {
			return err
		}
	}

	v.watchForHTTPEvents(etcd, sqlDB)

	etcdTcpRoutes, err := etcd.ReadTcpRouteMappings()
	if err != nil {
		return err
	}
	for _, route := range etcdTcpRoutes {
		r, err := models.NewTcpRouteMappingWithModel(route)
		if err != nil {
			return err
		}
		r.ExpiresAt = route.ExpiresAt

		_, err = sqlDB.Client.Create(&r)
		if err != nil {
			return err
		}
	}

	v.watchForTCPEvents(etcd, sqlDB)
	return nil
}

func (v *V1EtcdMigration) watchForHTTPEvents(etcd db.DB, sqlDB db.DB) {

	events, errs, cancel := etcd.WatchChanges(db.HTTP_WATCH)
	go func() {
		for {
			select {
			case event := <-events:
				var httpRoute models.Route
				switch event.Type {
				case db.CreateEvent, db.UpdateEvent:
					err := json.Unmarshal([]byte(event.Value), &httpRoute)
					if err != nil {
						v.logger.Error("failed-to-unmarshal-http-event", err)
					}
					err = sqlDB.SaveRoute(httpRoute)
					if err != nil {
						v.logger.Error("failed-to-save-http-route", err)
					}
				case db.DeleteEvent:
					err := json.Unmarshal([]byte(event.Value), &httpRoute)
					if err != nil {
						v.logger.Error("failed-to-unmarshal-http-event", err)
					}
					err = sqlDB.DeleteRoute(httpRoute)
					if err != nil {
						v.logger.Error("failed-to-delete-http-route", err)
					}
				default:
					v.logger.Info("unknown-event-type", lager.Data{"event-type": event.Type})
				}
			case err := <-errs:
				v.logger.Error("received-error-from-etcd-watch", err)
				return
			case <-v.done:
				cancel()
				return
			}
		}
	}()
}

func (v *V1EtcdMigration) watchForRouterGroupChanges(etcd db.DB, sqlDB db.DB) {

	events, errs, cancel := etcd.WatchChanges(db.ROUTER_GROUP_WATCH)
	go func() {
		for {
			select {
			case event := <-events:
				var routerGroup models.RouterGroup
				switch event.Type {
				case db.UpdateEvent:
					err := json.Unmarshal([]byte(event.Value), &routerGroup)
					if err != nil {
						v.logger.Error("failed-to-unmarshal-router-group-event", err)
					}
					rg, err := sqlDB.ReadRouterGroup(routerGroup.Guid)
					if err != nil {
						v.logger.Error("failed-to-read-router-group", err)
					}
					if rg.Guid == routerGroup.Guid {
						err := sqlDB.SaveRouterGroup(routerGroup)
						if err != nil {
							v.logger.Error("failed-to-save-router-group", err)
						}
					}
				default:
					v.logger.Info("unknown-event-type", lager.Data{"event-type": event.Type})
				}
			case err := <-errs:
				v.logger.Error("received-error-from-etcd-watch", err)
				return
			case <-v.done:
				cancel()
				return
			}
		}
	}()
}
func (v *V1EtcdMigration) watchForTCPEvents(etcd db.DB, sqlDB db.DB) {

	events, errs, cancel := etcd.WatchChanges(db.TCP_WATCH)
	go func() {
		for {
			select {
			case event := <-events:
				var tcpRoute models.TcpRouteMapping
				switch event.Type {
				case db.CreateEvent, db.UpdateEvent:
					err := json.Unmarshal([]byte(event.Value), &tcpRoute)
					if err != nil {
						v.logger.Error("failed-to-unmarshal-tcp-event", err)
					}
					err = sqlDB.SaveTcpRouteMapping(tcpRoute)
					if err != nil {
						v.logger.Error("failed-to-save-tcp-route", err)
					}
				case db.DeleteEvent:
					err := json.Unmarshal([]byte(event.Value), &tcpRoute)
					if err != nil {
						v.logger.Error("failed-to-unmarshal-tcp-event", err)
					}
					err = sqlDB.DeleteTcpRouteMapping(tcpRoute)
					if err != nil {
						v.logger.Error("failed-to-delete-tcp-route", err)
					}
				default:
					v.logger.Info("unknown-event-type", lager.Data{"event-type": event.Type})
				}
			case err := <-errs:
				v.logger.Error("received-error-from-etcd-watch", err)
				return
			case <-v.done:
				cancel()
				return
			}
		}
	}()
}
