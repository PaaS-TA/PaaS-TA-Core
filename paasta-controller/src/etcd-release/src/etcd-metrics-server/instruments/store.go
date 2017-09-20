package instruments

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instrumentation"
)

type Store struct {
	statsEndpoint string
	keysEndpoint  string
	getter        getter
	logger        lager.Logger
}

func NewStore(getter getter, etcdAddr string, logger lager.Logger) *Store {
	return &Store{
		statsEndpoint: fmt.Sprintf("%s/v2/stats/store", etcdAddr),
		keysEndpoint:  fmt.Sprintf("%s/v2/keys/", etcdAddr),
		getter:        getter,
		logger:        logger,
	}
}

func (store *Store) Emit() instrumentation.Context {
	context := instrumentation.Context{
		Name: "store",
	}

	var stats map[string]uint64

	statsResp, err := store.getter.Get(store.statsEndpoint)
	if err != nil {
		store.logger.Error("failed-to-collect-store-stats", err)
		return context
	}

	defer statsResp.Body.Close()

	err = json.NewDecoder(statsResp.Body).Decode(&stats)
	if err != nil {
		store.logger.Error("failed-to-unmarshal-store-stats", err)
		return context
	}

	keysResp, err := store.getter.Get(store.keysEndpoint)
	if err != nil {
		store.logger.Error("failed-to-read-from-store", err)
		return context
	}

	defer keysResp.Body.Close()

	etcdIndexHeader := keysResp.Header.Get("X-Etcd-Index")
	raftIndexHeader := keysResp.Header.Get("X-Raft-Index")
	raftTermHeader := keysResp.Header.Get("X-Raft-Term")

	etcdIndex, err := strconv.ParseUint(etcdIndexHeader, 10, 0)
	if err != nil {
		store.logger.Error("failed-to-parse-etcd-index", err, lager.Data{
			"index": etcdIndexHeader,
		})
		return context
	}

	raftIndex, err := strconv.ParseUint(raftIndexHeader, 10, 0)
	if err != nil {
		store.logger.Error("failed-to-parse-raft-index", err, lager.Data{
			"index": raftIndexHeader,
		})
		return context
	}

	raftTerm, err := strconv.ParseUint(raftTermHeader, 10, 0)
	if err != nil {
		store.logger.Error("failed-to-parse-raft-term", err, lager.Data{
			"term": raftTermHeader,
		})
		return context
	}

	context.Metrics = []instrumentation.Metric{
		{
			Name:  "EtcdIndex",
			Value: etcdIndex,
		},
		{
			Name:  "RaftIndex",
			Value: raftIndex,
		},
		{
			Name:  "RaftTerm",
			Value: raftTerm,
		},
	}

	for name, val := range stats {
		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  strings.ToUpper(name[0:1]) + name[1:],
			Value: val,
		})
	}

	return context
}
