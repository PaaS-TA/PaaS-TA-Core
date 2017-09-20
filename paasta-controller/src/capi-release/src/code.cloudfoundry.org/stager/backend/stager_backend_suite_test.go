package backend_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func actionsFromTaskDef(taskDef *models.TaskDefinition) []*models.Action {
	timeoutAction := taskDef.Action.GetTimeoutAction()
	Expect(timeoutAction).NotTo(BeNil())
	serialAction := timeoutAction.Action.GetSerialAction()
	Expect(serialAction).NotTo(BeNil())

	return serialAction.Actions
}

func TestBackend(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Backend Suite")
}
