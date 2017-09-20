package store

import (
	"path"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"

	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
)

type AppServiceStoreWatcher struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- appservice.AppService
	cache                     cache.AppServiceWatcherCache
	logger                    *gosteno.Logger

	done chan struct{}
}

func NewAppServiceStoreWatcher(adapter storeadapter.StoreAdapter, cache cache.AppServiceWatcherCache, logger *gosteno.Logger) (*AppServiceStoreWatcher, <-chan appservice.AppService, <-chan appservice.AppService) {
	outAddChan := make(chan appservice.AppService)
	outRemoveChan := make(chan appservice.AppService)
	return &AppServiceStoreWatcher{
		adapter:       adapter,
		outAddChan:    outAddChan,
		outRemoveChan: outRemoveChan,
		cache:         cache,
		logger:        logger,
		done:          make(chan struct{}),
	}, outAddChan, outRemoveChan
}

func (w *AppServiceStoreWatcher) Add(appService appservice.AppService) {
	if !w.cache.Exists(appService) {
		w.cache.Add(appService)
		w.outAddChan <- appService
	}
}

func (w *AppServiceStoreWatcher) Remove(appService appservice.AppService) {
	if w.cache.Exists(appService) {
		w.cache.Remove(appService)
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) RemoveApp(appId string) []appservice.AppService {
	appServices := w.cache.RemoveApp(appId)
	for _, appService := range appServices {
		w.outRemoveChan <- appService
	}
	return appServices
}

func (w *AppServiceStoreWatcher) Get(appId string) []appservice.AppService {
	return w.cache.Get(appId)
}

func (w *AppServiceStoreWatcher) Exists(appService appservice.AppService) bool {
	return w.cache.Exists(appService)
}

func (w *AppServiceStoreWatcher) Stop() {
	close(w.done)
}

func (w *AppServiceStoreWatcher) Run() {
	defer func() {
		close(w.outAddChan)
		close(w.outRemoveChan)
	}()

	events, stopChan, errChan := w.adapter.Watch("/loggregator/services")

	w.registerExistingServicesFromStore()
	for {
		for {
			select {
			case <-w.done:
				close(stopChan)
				return
			case err, ok := <-errChan:
				if !ok {
					return
				}
				w.logger.Errorf("AppStoreWatcher: Got error while waiting for ETCD events: %s", err.Error())
				events, stopChan, errChan = w.adapter.Watch("/loggregator/services")
			case event, ok := <-events:
				if !ok {
					return
				}

				w.logger.Debugf("AppStoreWatcher: Got an event from store %s", event.Type)
				switch event.Type {
				case storeadapter.CreateEvent, storeadapter.UpdateEvent:
					if event.Node.Dir || len(event.Node.Value) == 0 {
						// we can ignore any directory nodes (app or other namespace additions)
						continue
					}
					w.Add(appServiceFromStoreNode(event.Node))
				case storeadapter.DeleteEvent:
					w.deleteEvent(event.PrevNode)
				case storeadapter.ExpireEvent:
					w.deleteEvent(event.PrevNode)
				}
			}
		}
	}
}

func (w *AppServiceStoreWatcher) registerExistingServicesFromStore() {
	w.logger.Debug("AppStoreWatcher: Ensuring existing services are registered")
	services, _ := w.adapter.ListRecursively("/loggregator/services/")
	for _, node := range services.ChildNodes {
		for _, node := range node.ChildNodes {
			appService := appServiceFromStoreNode(&node)
			w.Add(appService)
		}
	}
	w.logger.Debug("AppStoreWatcher: Existing services all registered")
}

func appServiceFromStoreNode(node *storeadapter.StoreNode) appservice.AppService {
	key := node.Key
	appId := path.Base(path.Dir(key))
	serviceUrl := string(node.Value)
	appService := appservice.AppService{AppId: appId, Url: serviceUrl}
	return appService
}

func (w *AppServiceStoreWatcher) deleteEvent(node *storeadapter.StoreNode) {
	if node.Dir {
		key := node.Key
		appId := path.Base(key)
		w.RemoveApp(appId)
	} else {
		w.Remove(appServiceFromStoreNode(node))
	}
}
