package instruments

import (
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instrumentation"
)

type Leader struct {
	statsEndpoint string
	logger        lager.Logger
	getter        getter
}

var ErrRedirected = errors.New("redirected to leader")

func NewLeader(getter getter, etcdAddr string, logger lager.Logger) *Leader {
	return &Leader{
		statsEndpoint: fmt.Sprintf("%s/v2/stats/leader", etcdAddr),
		logger:        logger,
		getter:        getter,
	}
}

func (leader *Leader) Emit() instrumentation.Context {
	context := instrumentation.Context{
		Name:    "leader",
		Metrics: []instrumentation.Metric{},
	}

	var stats RaftFollowersStats

	resp, err := leader.getter.Get(leader.statsEndpoint)
	if err != nil {
		leader.logger.Error("failed-to-collect-leader-stats", err)
		return context
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		leader.logger.Error("failed-to-unmarshal-leader-stats", err)
		return context
	}

	context.Metrics = []instrumentation.Metric{
		{
			Name:  "Followers",
			Value: len(stats.Followers),
		},
	}

	for name, follower := range stats.Followers {
		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "Latency",
			Value: follower.Latency.Current,
			Tags: map[string]interface{}{
				"follower": name,
			},
		})
	}

	return context
}
