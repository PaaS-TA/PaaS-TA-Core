package rep

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"code.cloudfoundry.org/bbs/models"
)

var ErrorIncompatibleRootfs = errors.New("rootfs not found")

type CellState struct {
	RootFSProviders        RootFSProviders
	AvailableResources     Resources
	TotalResources         Resources
	LRPs                   []LRP
	Tasks                  []Task
	StartingContainerCount int
	Zone                   string
	Evacuating             bool
	VolumeDrivers          []string
	PlacementTags          []string
	OptionalPlacementTags  []string
}

func NewCellState(
	root RootFSProviders,
	avail Resources,
	total Resources,
	lrps []LRP,
	tasks []Task,
	zone string,
	startingContainerCount int,
	isEvac bool,
	volumeDrivers []string,
	placementTags []string,
	optionalPlacementTags []string,
) CellState {
	return CellState{
		RootFSProviders:    root,
		AvailableResources: avail,
		TotalResources:     total,
		LRPs:               lrps,
		Tasks:              tasks,
		Zone:               zone,
		StartingContainerCount: startingContainerCount,
		Evacuating:             isEvac,
		VolumeDrivers:          volumeDrivers,
		PlacementTags:          placementTags,
		OptionalPlacementTags:  optionalPlacementTags,
	}
}

func (c *CellState) AddLRP(lrp *LRP) {
	c.AvailableResources.Subtract(&lrp.Resource)
	c.StartingContainerCount += 1
	c.LRPs = append(c.LRPs, *lrp)
}

func (c *CellState) AddTask(task *Task) {
	c.AvailableResources.Subtract(&task.Resource)
	c.StartingContainerCount += 1
	c.Tasks = append(c.Tasks, *task)
}

func (c *CellState) ResourceMatch(res *Resource) error {
	problems := map[string]struct{}{}

	if c.AvailableResources.DiskMB < res.DiskMB {
		problems["disk"] = struct{}{}
	}
	if c.AvailableResources.MemoryMB < res.MemoryMB {
		problems["memory"] = struct{}{}
	}
	if c.AvailableResources.Containers < 1 {
		problems["containers"] = struct{}{}
	}
	if len(problems) == 0 {
		return nil
	}

	return InsufficientResourcesError{Problems: problems}
}

type InsufficientResourcesError struct {
	Problems map[string]struct{}
}

func (i InsufficientResourcesError) Error() string {
	if len(i.Problems) == 0 {
		return "insufficient resources"
	}

	keys := []string{}
	for key, _ := range i.Problems {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return fmt.Sprintf("insufficient resources: %s", strings.Join(keys, ", "))
}

func (c CellState) ComputeScore(res *Resource, startingContainerWeight float64) float64 {
	remainingResources := c.AvailableResources.Copy()
	remainingResources.Subtract(res)
	startingContainerScore := float64(c.StartingContainerCount) * startingContainerWeight
	return remainingResources.ComputeScore(&c.TotalResources) + startingContainerScore
}

func (c *CellState) MatchRootFS(rootfs string) bool {
	rootFSURL, err := url.Parse(rootfs)
	if err != nil {
		return false
	}

	return c.RootFSProviders.Match(*rootFSURL)
}

func (c *CellState) MatchVolumeDrivers(volumeDrivers []string) bool {
	for _, requestedDriver := range volumeDrivers {
		found := false

		for _, actualDriver := range c.VolumeDrivers {
			if requestedDriver == actualDriver {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func (c *CellState) MatchPlacementTags(desiredPlacementTags []string) bool {
	desiredTags := toSet(desiredPlacementTags)
	optionalTags := toSet(c.OptionalPlacementTags)
	requiredTags := toSet(c.PlacementTags)
	allTags := requiredTags.union(optionalTags)

	return requiredTags.isSubset(desiredTags) && desiredTags.isSubset(allTags)
}

type placementTagSet map[string]struct{}

func (set placementTagSet) union(other placementTagSet) placementTagSet {
	tags := placementTagSet{}
	for k := range set {
		tags[k] = struct{}{}
	}
	for k := range other {
		tags[k] = struct{}{}
	}
	return tags
}

func (set placementTagSet) isSubset(other placementTagSet) bool {
	for k := range set {
		if _, ok := other[k]; !ok {
			return false
		}
	}
	return true
}

func toSet(slice []string) placementTagSet {
	tags := placementTagSet{}
	for _, k := range slice {
		tags[k] = struct{}{}
	}
	return tags
}

type Resources struct {
	MemoryMB   int32
	DiskMB     int32
	Containers int
}

func NewResources(memoryMb, diskMb int32, containerCount int) Resources {
	return Resources{memoryMb, diskMb, containerCount}
}

func (r *Resources) Copy() Resources {
	return *r
}

func (r *Resources) Subtract(res *Resource) {
	r.MemoryMB -= res.MemoryMB
	r.DiskMB -= res.DiskMB
	r.Containers -= 1
}

func (r *Resources) ComputeScore(total *Resources) float64 {
	fractionUsedMemory := 1.0 - float64(r.MemoryMB)/float64(total.MemoryMB)
	fractionUsedDisk := 1.0 - float64(r.DiskMB)/float64(total.DiskMB)
	fractionUsedContainers := 1.0 - float64(r.Containers)/float64(total.Containers)
	return (fractionUsedMemory + fractionUsedDisk + fractionUsedContainers) / 3.0
}

type Resource struct {
	MemoryMB int32
	DiskMB   int32
	MaxPids  int32
}

func NewResource(memoryMb, diskMb int32, maxPids int32) Resource {
	return Resource{MemoryMB: memoryMb, DiskMB: diskMb, MaxPids: maxPids}
}

func (r *Resource) Valid() bool {
	return r.DiskMB >= 0 && r.MemoryMB >= 0
}

func (r *Resource) Copy() Resource {
	return NewResource(r.MemoryMB, r.DiskMB, r.MaxPids)
}

type PlacementConstraint struct {
	PlacementTags []string
	VolumeDrivers []string
	RootFs        string
}

func NewPlacementConstraint(rootFs string, placementTags, volumeDrivers []string) PlacementConstraint {
	return PlacementConstraint{PlacementTags: placementTags, VolumeDrivers: volumeDrivers, RootFs: rootFs}
}

func (p *PlacementConstraint) Valid() bool {
	return p.RootFs != ""
}

type LRP struct {
	models.ActualLRPKey
	PlacementConstraint
	Resource
}

func NewLRP(key models.ActualLRPKey, res Resource, pc PlacementConstraint) LRP {
	return LRP{key, pc, res}
}

func (lrp *LRP) Identifier() string {
	return fmt.Sprintf("%s.%d", lrp.ProcessGuid, lrp.Index)
}

func (lrp *LRP) Copy() LRP {
	return NewLRP(lrp.ActualLRPKey, lrp.Resource, lrp.PlacementConstraint)
}

type Task struct {
	TaskGuid string
	Domain   string
	PlacementConstraint
	Resource
}

func NewTask(guid string, domain string, res Resource, pc PlacementConstraint) Task {
	return Task{guid, domain, pc, res}
}

func (task *Task) Identifier() string {
	return task.TaskGuid
}

func (task Task) Copy() Task {
	return task
}

type Work struct {
	LRPs  []LRP
	Tasks []Task
}

type StackPathMap map[string]string

func UnmarshalStackPathMap(payload []byte) (StackPathMap, error) {
	stackPathMap := StackPathMap{}
	err := json.Unmarshal(payload, &stackPathMap)
	return stackPathMap, err
}
