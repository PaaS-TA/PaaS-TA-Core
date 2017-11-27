package fakeroutingtable

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routingtable"
)

type FakeRoutingTable struct {
	SetRoutesStub        func(beforeLRP, afterLRP *models.DesiredLRPSchedulingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	setRoutesMutex       sync.RWMutex
	setRoutesArgsForCall []struct {
		beforeLRP *models.DesiredLRPSchedulingInfo
		afterLRP  *models.DesiredLRPSchedulingInfo
	}
	setRoutesReturns struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	setRoutesReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	RemoveRoutesStub        func(desiredLRP *models.DesiredLRPSchedulingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	removeRoutesMutex       sync.RWMutex
	removeRoutesArgsForCall []struct {
		desiredLRP *models.DesiredLRPSchedulingInfo
	}
	removeRoutesReturns struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	removeRoutesReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	AddEndpointStub        func(actualLRP *routingtable.ActualLRPRoutingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	addEndpointMutex       sync.RWMutex
	addEndpointArgsForCall []struct {
		actualLRP *routingtable.ActualLRPRoutingInfo
	}
	addEndpointReturns struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	addEndpointReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	RemoveEndpointStub        func(actualLRP *routingtable.ActualLRPRoutingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	removeEndpointMutex       sync.RWMutex
	removeEndpointArgsForCall []struct {
		actualLRP *routingtable.ActualLRPRoutingInfo
	}
	removeEndpointReturns struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	removeEndpointReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	SwapStub        func(t routingtable.RoutingTable, domains models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	swapMutex       sync.RWMutex
	swapArgsForCall []struct {
		t       routingtable.RoutingTable
		domains models.DomainSet
	}
	swapReturns struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	swapReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	GetRoutingEventsStub        func() (routingtable.TCPRouteMappings, routingtable.MessagesToEmit)
	getRoutingEventsMutex       sync.RWMutex
	getRoutingEventsArgsForCall []struct{}
	getRoutingEventsReturns     struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	getRoutingEventsReturnsOnCall map[int]struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}
	HasExternalRoutesStub        func(actual *routingtable.ActualLRPRoutingInfo) bool
	hasExternalRoutesMutex       sync.RWMutex
	hasExternalRoutesArgsForCall []struct {
		actual *routingtable.ActualLRPRoutingInfo
	}
	hasExternalRoutesReturns struct {
		result1 bool
	}
	hasExternalRoutesReturnsOnCall map[int]struct {
		result1 bool
	}
	HTTPEndpointCountStub        func() int
	hTTPEndpointCountMutex       sync.RWMutex
	hTTPEndpointCountArgsForCall []struct{}
	hTTPEndpointCountReturns     struct {
		result1 int
	}
	hTTPEndpointCountReturnsOnCall map[int]struct {
		result1 int
	}
	TCPRouteCountStub        func() int
	tCPRouteCountMutex       sync.RWMutex
	tCPRouteCountArgsForCall []struct{}
	tCPRouteCountReturns     struct {
		result1 int
	}
	tCPRouteCountReturnsOnCall map[int]struct {
		result1 int
	}
	TableSizeStub        func() int
	tableSizeMutex       sync.RWMutex
	tableSizeArgsForCall []struct{}
	tableSizeReturns     struct {
		result1 int
	}
	tableSizeReturnsOnCall map[int]struct {
		result1 int
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRoutingTable) SetRoutes(beforeLRP *models.DesiredLRPSchedulingInfo, afterLRP *models.DesiredLRPSchedulingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.setRoutesMutex.Lock()
	ret, specificReturn := fake.setRoutesReturnsOnCall[len(fake.setRoutesArgsForCall)]
	fake.setRoutesArgsForCall = append(fake.setRoutesArgsForCall, struct {
		beforeLRP *models.DesiredLRPSchedulingInfo
		afterLRP  *models.DesiredLRPSchedulingInfo
	}{beforeLRP, afterLRP})
	fake.recordInvocation("SetRoutes", []interface{}{beforeLRP, afterLRP})
	fake.setRoutesMutex.Unlock()
	if fake.SetRoutesStub != nil {
		return fake.SetRoutesStub(beforeLRP, afterLRP)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.setRoutesReturns.result1, fake.setRoutesReturns.result2
}

func (fake *FakeRoutingTable) SetRoutesCallCount() int {
	fake.setRoutesMutex.RLock()
	defer fake.setRoutesMutex.RUnlock()
	return len(fake.setRoutesArgsForCall)
}

func (fake *FakeRoutingTable) SetRoutesArgsForCall(i int) (*models.DesiredLRPSchedulingInfo, *models.DesiredLRPSchedulingInfo) {
	fake.setRoutesMutex.RLock()
	defer fake.setRoutesMutex.RUnlock()
	return fake.setRoutesArgsForCall[i].beforeLRP, fake.setRoutesArgsForCall[i].afterLRP
}

func (fake *FakeRoutingTable) SetRoutesReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.SetRoutesStub = nil
	fake.setRoutesReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) SetRoutesReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.SetRoutesStub = nil
	if fake.setRoutesReturnsOnCall == nil {
		fake.setRoutesReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.setRoutesReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) RemoveRoutes(desiredLRP *models.DesiredLRPSchedulingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.removeRoutesMutex.Lock()
	ret, specificReturn := fake.removeRoutesReturnsOnCall[len(fake.removeRoutesArgsForCall)]
	fake.removeRoutesArgsForCall = append(fake.removeRoutesArgsForCall, struct {
		desiredLRP *models.DesiredLRPSchedulingInfo
	}{desiredLRP})
	fake.recordInvocation("RemoveRoutes", []interface{}{desiredLRP})
	fake.removeRoutesMutex.Unlock()
	if fake.RemoveRoutesStub != nil {
		return fake.RemoveRoutesStub(desiredLRP)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.removeRoutesReturns.result1, fake.removeRoutesReturns.result2
}

func (fake *FakeRoutingTable) RemoveRoutesCallCount() int {
	fake.removeRoutesMutex.RLock()
	defer fake.removeRoutesMutex.RUnlock()
	return len(fake.removeRoutesArgsForCall)
}

func (fake *FakeRoutingTable) RemoveRoutesArgsForCall(i int) *models.DesiredLRPSchedulingInfo {
	fake.removeRoutesMutex.RLock()
	defer fake.removeRoutesMutex.RUnlock()
	return fake.removeRoutesArgsForCall[i].desiredLRP
}

func (fake *FakeRoutingTable) RemoveRoutesReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.RemoveRoutesStub = nil
	fake.removeRoutesReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) RemoveRoutesReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.RemoveRoutesStub = nil
	if fake.removeRoutesReturnsOnCall == nil {
		fake.removeRoutesReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.removeRoutesReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) AddEndpoint(actualLRP *routingtable.ActualLRPRoutingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.addEndpointMutex.Lock()
	ret, specificReturn := fake.addEndpointReturnsOnCall[len(fake.addEndpointArgsForCall)]
	fake.addEndpointArgsForCall = append(fake.addEndpointArgsForCall, struct {
		actualLRP *routingtable.ActualLRPRoutingInfo
	}{actualLRP})
	fake.recordInvocation("AddEndpoint", []interface{}{actualLRP})
	fake.addEndpointMutex.Unlock()
	if fake.AddEndpointStub != nil {
		return fake.AddEndpointStub(actualLRP)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.addEndpointReturns.result1, fake.addEndpointReturns.result2
}

func (fake *FakeRoutingTable) AddEndpointCallCount() int {
	fake.addEndpointMutex.RLock()
	defer fake.addEndpointMutex.RUnlock()
	return len(fake.addEndpointArgsForCall)
}

func (fake *FakeRoutingTable) AddEndpointArgsForCall(i int) *routingtable.ActualLRPRoutingInfo {
	fake.addEndpointMutex.RLock()
	defer fake.addEndpointMutex.RUnlock()
	return fake.addEndpointArgsForCall[i].actualLRP
}

func (fake *FakeRoutingTable) AddEndpointReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.AddEndpointStub = nil
	fake.addEndpointReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) AddEndpointReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.AddEndpointStub = nil
	if fake.addEndpointReturnsOnCall == nil {
		fake.addEndpointReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.addEndpointReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) RemoveEndpoint(actualLRP *routingtable.ActualLRPRoutingInfo) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.removeEndpointMutex.Lock()
	ret, specificReturn := fake.removeEndpointReturnsOnCall[len(fake.removeEndpointArgsForCall)]
	fake.removeEndpointArgsForCall = append(fake.removeEndpointArgsForCall, struct {
		actualLRP *routingtable.ActualLRPRoutingInfo
	}{actualLRP})
	fake.recordInvocation("RemoveEndpoint", []interface{}{actualLRP})
	fake.removeEndpointMutex.Unlock()
	if fake.RemoveEndpointStub != nil {
		return fake.RemoveEndpointStub(actualLRP)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.removeEndpointReturns.result1, fake.removeEndpointReturns.result2
}

func (fake *FakeRoutingTable) RemoveEndpointCallCount() int {
	fake.removeEndpointMutex.RLock()
	defer fake.removeEndpointMutex.RUnlock()
	return len(fake.removeEndpointArgsForCall)
}

func (fake *FakeRoutingTable) RemoveEndpointArgsForCall(i int) *routingtable.ActualLRPRoutingInfo {
	fake.removeEndpointMutex.RLock()
	defer fake.removeEndpointMutex.RUnlock()
	return fake.removeEndpointArgsForCall[i].actualLRP
}

func (fake *FakeRoutingTable) RemoveEndpointReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.RemoveEndpointStub = nil
	fake.removeEndpointReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) RemoveEndpointReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.RemoveEndpointStub = nil
	if fake.removeEndpointReturnsOnCall == nil {
		fake.removeEndpointReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.removeEndpointReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) Swap(t routingtable.RoutingTable, domains models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.swapMutex.Lock()
	ret, specificReturn := fake.swapReturnsOnCall[len(fake.swapArgsForCall)]
	fake.swapArgsForCall = append(fake.swapArgsForCall, struct {
		t       routingtable.RoutingTable
		domains models.DomainSet
	}{t, domains})
	fake.recordInvocation("Swap", []interface{}{t, domains})
	fake.swapMutex.Unlock()
	if fake.SwapStub != nil {
		return fake.SwapStub(t, domains)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.swapReturns.result1, fake.swapReturns.result2
}

func (fake *FakeRoutingTable) SwapCallCount() int {
	fake.swapMutex.RLock()
	defer fake.swapMutex.RUnlock()
	return len(fake.swapArgsForCall)
}

func (fake *FakeRoutingTable) SwapArgsForCall(i int) (routingtable.RoutingTable, models.DomainSet) {
	fake.swapMutex.RLock()
	defer fake.swapMutex.RUnlock()
	return fake.swapArgsForCall[i].t, fake.swapArgsForCall[i].domains
}

func (fake *FakeRoutingTable) SwapReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.SwapStub = nil
	fake.swapReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) SwapReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.SwapStub = nil
	if fake.swapReturnsOnCall == nil {
		fake.swapReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.swapReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) GetRoutingEvents() (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
	fake.getRoutingEventsMutex.Lock()
	ret, specificReturn := fake.getRoutingEventsReturnsOnCall[len(fake.getRoutingEventsArgsForCall)]
	fake.getRoutingEventsArgsForCall = append(fake.getRoutingEventsArgsForCall, struct{}{})
	fake.recordInvocation("GetRoutingEvents", []interface{}{})
	fake.getRoutingEventsMutex.Unlock()
	if fake.GetRoutingEventsStub != nil {
		return fake.GetRoutingEventsStub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getRoutingEventsReturns.result1, fake.getRoutingEventsReturns.result2
}

func (fake *FakeRoutingTable) GetRoutingEventsCallCount() int {
	fake.getRoutingEventsMutex.RLock()
	defer fake.getRoutingEventsMutex.RUnlock()
	return len(fake.getRoutingEventsArgsForCall)
}

func (fake *FakeRoutingTable) GetRoutingEventsReturns(result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.GetRoutingEventsStub = nil
	fake.getRoutingEventsReturns = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) GetRoutingEventsReturnsOnCall(i int, result1 routingtable.TCPRouteMappings, result2 routingtable.MessagesToEmit) {
	fake.GetRoutingEventsStub = nil
	if fake.getRoutingEventsReturnsOnCall == nil {
		fake.getRoutingEventsReturnsOnCall = make(map[int]struct {
			result1 routingtable.TCPRouteMappings
			result2 routingtable.MessagesToEmit
		})
	}
	fake.getRoutingEventsReturnsOnCall[i] = struct {
		result1 routingtable.TCPRouteMappings
		result2 routingtable.MessagesToEmit
	}{result1, result2}
}

func (fake *FakeRoutingTable) HasExternalRoutes(actual *routingtable.ActualLRPRoutingInfo) bool {
	fake.hasExternalRoutesMutex.Lock()
	ret, specificReturn := fake.hasExternalRoutesReturnsOnCall[len(fake.hasExternalRoutesArgsForCall)]
	fake.hasExternalRoutesArgsForCall = append(fake.hasExternalRoutesArgsForCall, struct {
		actual *routingtable.ActualLRPRoutingInfo
	}{actual})
	fake.recordInvocation("HasExternalRoutes", []interface{}{actual})
	fake.hasExternalRoutesMutex.Unlock()
	if fake.HasExternalRoutesStub != nil {
		return fake.HasExternalRoutesStub(actual)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.hasExternalRoutesReturns.result1
}

func (fake *FakeRoutingTable) HasExternalRoutesCallCount() int {
	fake.hasExternalRoutesMutex.RLock()
	defer fake.hasExternalRoutesMutex.RUnlock()
	return len(fake.hasExternalRoutesArgsForCall)
}

func (fake *FakeRoutingTable) HasExternalRoutesArgsForCall(i int) *routingtable.ActualLRPRoutingInfo {
	fake.hasExternalRoutesMutex.RLock()
	defer fake.hasExternalRoutesMutex.RUnlock()
	return fake.hasExternalRoutesArgsForCall[i].actual
}

func (fake *FakeRoutingTable) HasExternalRoutesReturns(result1 bool) {
	fake.HasExternalRoutesStub = nil
	fake.hasExternalRoutesReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeRoutingTable) HasExternalRoutesReturnsOnCall(i int, result1 bool) {
	fake.HasExternalRoutesStub = nil
	if fake.hasExternalRoutesReturnsOnCall == nil {
		fake.hasExternalRoutesReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.hasExternalRoutesReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakeRoutingTable) HTTPAssociationsCount() int {
	fake.hTTPEndpointCountMutex.Lock()
	ret, specificReturn := fake.hTTPEndpointCountReturnsOnCall[len(fake.hTTPEndpointCountArgsForCall)]
	fake.hTTPEndpointCountArgsForCall = append(fake.hTTPEndpointCountArgsForCall, struct{}{})
	fake.recordInvocation("HTTPEndpointCount", []interface{}{})
	fake.hTTPEndpointCountMutex.Unlock()
	if fake.HTTPEndpointCountStub != nil {
		return fake.HTTPEndpointCountStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.hTTPEndpointCountReturns.result1
}

func (fake *FakeRoutingTable) HTTPEndpointCountCallCount() int {
	fake.hTTPEndpointCountMutex.RLock()
	defer fake.hTTPEndpointCountMutex.RUnlock()
	return len(fake.hTTPEndpointCountArgsForCall)
}

func (fake *FakeRoutingTable) HTTPEndpointCountReturns(result1 int) {
	fake.HTTPEndpointCountStub = nil
	fake.hTTPEndpointCountReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) HTTPEndpointCountReturnsOnCall(i int, result1 int) {
	fake.HTTPEndpointCountStub = nil
	if fake.hTTPEndpointCountReturnsOnCall == nil {
		fake.hTTPEndpointCountReturnsOnCall = make(map[int]struct {
			result1 int
		})
	}
	fake.hTTPEndpointCountReturnsOnCall[i] = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) TCPAssociationsCount() int {
	fake.tCPRouteCountMutex.Lock()
	ret, specificReturn := fake.tCPRouteCountReturnsOnCall[len(fake.tCPRouteCountArgsForCall)]
	fake.tCPRouteCountArgsForCall = append(fake.tCPRouteCountArgsForCall, struct{}{})
	fake.recordInvocation("TCPRouteCount", []interface{}{})
	fake.tCPRouteCountMutex.Unlock()
	if fake.TCPRouteCountStub != nil {
		return fake.TCPRouteCountStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.tCPRouteCountReturns.result1
}

func (fake *FakeRoutingTable) TCPRouteCountCallCount() int {
	fake.tCPRouteCountMutex.RLock()
	defer fake.tCPRouteCountMutex.RUnlock()
	return len(fake.tCPRouteCountArgsForCall)
}

func (fake *FakeRoutingTable) TCPRouteCountReturns(result1 int) {
	fake.TCPRouteCountStub = nil
	fake.tCPRouteCountReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) TCPRouteCountReturnsOnCall(i int, result1 int) {
	fake.TCPRouteCountStub = nil
	if fake.tCPRouteCountReturnsOnCall == nil {
		fake.tCPRouteCountReturnsOnCall = make(map[int]struct {
			result1 int
		})
	}
	fake.tCPRouteCountReturnsOnCall[i] = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) TableSize() int {
	fake.tableSizeMutex.Lock()
	ret, specificReturn := fake.tableSizeReturnsOnCall[len(fake.tableSizeArgsForCall)]
	fake.tableSizeArgsForCall = append(fake.tableSizeArgsForCall, struct{}{})
	fake.recordInvocation("TableSize", []interface{}{})
	fake.tableSizeMutex.Unlock()
	if fake.TableSizeStub != nil {
		return fake.TableSizeStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.tableSizeReturns.result1
}

func (fake *FakeRoutingTable) TableSizeCallCount() int {
	fake.tableSizeMutex.RLock()
	defer fake.tableSizeMutex.RUnlock()
	return len(fake.tableSizeArgsForCall)
}

func (fake *FakeRoutingTable) TableSizeReturns(result1 int) {
	fake.TableSizeStub = nil
	fake.tableSizeReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) TableSizeReturnsOnCall(i int, result1 int) {
	fake.TableSizeStub = nil
	if fake.tableSizeReturnsOnCall == nil {
		fake.tableSizeReturnsOnCall = make(map[int]struct {
			result1 int
		})
	}
	fake.tableSizeReturnsOnCall[i] = struct {
		result1 int
	}{result1}
}

func (fake *FakeRoutingTable) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.setRoutesMutex.RLock()
	defer fake.setRoutesMutex.RUnlock()
	fake.removeRoutesMutex.RLock()
	defer fake.removeRoutesMutex.RUnlock()
	fake.addEndpointMutex.RLock()
	defer fake.addEndpointMutex.RUnlock()
	fake.removeEndpointMutex.RLock()
	defer fake.removeEndpointMutex.RUnlock()
	fake.swapMutex.RLock()
	defer fake.swapMutex.RUnlock()
	fake.getRoutingEventsMutex.RLock()
	defer fake.getRoutingEventsMutex.RUnlock()
	fake.hasExternalRoutesMutex.RLock()
	defer fake.hasExternalRoutesMutex.RUnlock()
	fake.hTTPEndpointCountMutex.RLock()
	defer fake.hTTPEndpointCountMutex.RUnlock()
	fake.tCPRouteCountMutex.RLock()
	defer fake.tCPRouteCountMutex.RUnlock()
	fake.tableSizeMutex.RLock()
	defer fake.tableSizeMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRoutingTable) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ routingtable.RoutingTable = new(FakeRoutingTable)
