package containerstore

import (
	"sync"
	"time"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
)

type nodeMap struct {
	nodes map[string]*storeNode
	lock  *sync.RWMutex

	remainingResources *executor.ExecutorResources
}

func newNodeMap(totalCapacity *executor.ExecutorResources) *nodeMap {
	capacity := totalCapacity.Copy()
	return &nodeMap{
		nodes:              make(map[string]*storeNode),
		lock:               &sync.RWMutex{},
		remainingResources: &capacity,
	}
}

func (n *nodeMap) Contains(guid string) bool {
	n.lock.RLock()
	defer n.lock.RUnlock()
	_, ok := n.nodes[guid]
	return ok
}

func (n *nodeMap) RemainingResources() executor.ExecutorResources {
	n.lock.RLock()
	defer n.lock.RUnlock()
	return n.remainingResources.Copy()
}

func (n *nodeMap) Add(node *storeNode) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	info := node.Info()
	if _, ok := n.nodes[info.Guid]; ok {
		return executor.ErrContainerGuidNotAvailable
	}

	ok := n.remainingResources.Subtract(&info.Resource)
	if !ok {
		return executor.ErrInsufficientResourcesAvailable
	}

	n.nodes[info.Guid] = node

	return nil
}

func (n *nodeMap) Remove(guid string) {
	n.lock.Lock()
	defer n.lock.Unlock()

	node, ok := n.nodes[guid]
	if !ok {
		return
	}

	n.remove(node)
}

func (n *nodeMap) remove(node *storeNode) {
	info := node.Info()
	n.remainingResources.Add(&info.Resource)
	delete(n.nodes, info.Guid)
}

func (n *nodeMap) Get(guid string) (*storeNode, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	node, ok := n.nodes[guid]
	if !ok {
		return nil, executor.ErrContainerNotFound
	}

	return node, nil
}

func (n *nodeMap) List() []*storeNode {
	n.lock.RLock()
	defer n.lock.RUnlock()

	list := make([]*storeNode, 0, len(n.nodes))
	for _, node := range n.nodes {
		list = append(list, node)
	}
	return list
}

func (n *nodeMap) CompleteExpired(logger lager.Logger, now time.Time) {
	n.lock.Lock()
	defer n.lock.Unlock()

	for i := range n.nodes {
		node := n.nodes[i]
		expired := node.Expire(logger, now)
		if expired {
			logger.Info("expired-container", lager.Data{"guid": node.Info().Guid})
		}
	}
}

func (n *nodeMap) CompleteMissing(logger lager.Logger, existingHandles map[string]struct{}) {
	n.lock.Lock()
	defer n.lock.Unlock()

	for i := range n.nodes {
		node := n.nodes[i]
		info := node.Info()

		_, ok := existingHandles[info.Guid]
		if !ok {
			reaped := node.Reap(logger)
			if reaped {
				logger.Info("reaped-missing-container", lager.Data{"guid": info.Guid})
			}
		}
	}
}
