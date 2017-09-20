package helpers

import (
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/gomega"
)

func ActiveActualLRPs(logger lager.Logger, client bbs.InternalClient, processGuid string) []models.ActualLRP {
	lrpGroups, err := client.ActualLRPGroupsByProcessGuid(logger, processGuid)
	Expect(err).NotTo(HaveOccurred())

	startedLRPs := make([]models.ActualLRP, 0, len(lrpGroups))
	for _, lrpGroup := range lrpGroups {
		lrp, _ := lrpGroup.Resolve()
		if lrp.State != models.ActualLRPStateUnclaimed {
			startedLRPs = append(startedLRPs, *lrp)
		}
	}

	return startedLRPs
}

func TaskStatePoller(logger lager.Logger, client bbs.InternalClient, taskGuid string, task *models.Task) func() models.Task_State {
	return func() models.Task_State {
		rTask, err := client.TaskByGuid(logger, taskGuid)
		Expect(err).NotTo(HaveOccurred())

		if task != nil {
			*task = *rTask
		}

		return rTask.State
	}
}

func TaskFailedPoller(logger lager.Logger, client bbs.InternalClient, taskGuid string, task *models.Task) func() bool {
	return func() bool {
		rTask, err := client.TaskByGuid(logger, taskGuid)
		Expect(err).NotTo(HaveOccurred())

		if task != nil {
			*task = *rTask
		}

		return rTask.Failed
	}
}

func LRPStatePoller(logger lager.Logger, client bbs.InternalClient, processGuid string, lrp *models.ActualLRP) func() string {
	return func() string {
		lrpGroups, err := client.ActualLRPGroupsByProcessGuid(logger, processGuid)
		Expect(err).NotTo(HaveOccurred())

		if len(lrpGroups) == 0 {
			return ""
		}

		foundLRP, _ := lrpGroups[0].Resolve()

		if lrp != nil {
			*lrp = *foundLRP
		}

		return foundLRP.State
	}
}

func LRPInstanceStatePoller(logger lager.Logger, client bbs.InternalClient, processGuid string, index int, lrp *models.ActualLRP) func() string {
	return func() string {
		lrpGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
		Expect(err).NotTo(HaveOccurred())

		foundLRP, _ := lrpGroup.Resolve()
		if lrp != nil {
			*lrp = *foundLRP
		}

		return foundLRP.State
	}
}
