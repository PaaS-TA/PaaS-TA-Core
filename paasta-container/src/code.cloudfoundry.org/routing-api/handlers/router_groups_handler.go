package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	"github.com/tedsuo/rata"
)

const portWarning = "Warning: if routes are registered for ports that are not " +
	"in the new range, modifying your load balancer to remove these ports will " +
	"result in backends for those routes becoming inaccessible."

type RouterGroupsHandler struct {
	uaaClient uaaclient.Client
	logger    lager.Logger
	db        db.DB
}

func NewRouteGroupsHandler(uaaClient uaaclient.Client, logger lager.Logger, db db.DB) *RouterGroupsHandler {
	return &RouterGroupsHandler{
		uaaClient: uaaClient,
		logger:    logger,
		db:        db,
	}
}

func (h *RouterGroupsHandler) ListRouterGroups(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("list-router-groups")
	log.Debug("started")
	defer log.Debug("completed")

	err := h.uaaClient.DecodeToken(req.Header.Get("Authorization"), RouterGroupsReadScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	routerGroups, err := h.db.ReadRouterGroups()
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}

	jsonBytes, err := json.Marshal(routerGroups)
	if err != nil {
		log.Error("failed-to-marshal", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("failed-to-write-to-response", err)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
}

func (h *RouterGroupsHandler) UpdateRouterGroup(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("update-router-group")
	log.Debug("started")
	defer log.Debug("completed")
	defer req.Body.Close()

	err := h.uaaClient.DecodeToken(req.Header.Get("Authorization"), RouterGroupsWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	bodyDecoder := json.NewDecoder(req.Body)
	var updatedGroup models.RouterGroup
	err = bodyDecoder.Decode(&updatedGroup)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	guid := rata.Param(req, "guid")
	rg, err := h.db.ReadRouterGroup(guid)
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}

	if rg == (models.RouterGroup{}) {
		handleNotFoundError(w, fmt.Errorf("Router Group '%s' does not exist", guid), log)
		return
	}

	if updatedGroup.ReservablePorts != "" && rg.ReservablePorts != updatedGroup.ReservablePorts {
		rg.ReservablePorts = updatedGroup.ReservablePorts
		err = rg.Validate()
		if err != nil {
			handleProcessRequestError(w, err, log)
			return
		}

		err = h.db.SaveRouterGroup(rg)
		if err != nil {
			handleDBCommunicationError(w, err, log)
			return
		}
	}

	jsonBytes, err := json.Marshal(rg)
	if err != nil {
		log.Error("failed-to-marshal", err)
	}

	addWarningsHeader(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonBytes)
	if err != nil {
		log.Error("failed-to-write-to-response", err)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
}

func addWarningsHeader(w http.ResponseWriter) {
	w.Header().Set("X-Cf-Warnings", url.QueryEscape(portWarning))
}
