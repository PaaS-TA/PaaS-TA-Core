package matchers

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func MatchDesiredLRPCreatedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPCreatedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPCreatedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPCreatedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPCreatedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPCreatedEventMatcher matcher expects a models.DesiredLRPCreatedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	return event.DesiredLrp.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPCreatedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPCreatedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPCreatedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPCreatedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchDesiredLRPChangedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPChangedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPChangedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPChangedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPChangedEventMatcher matcher expects a models.DesiredLRPChangedEvent.  Got:\n%s", format.Object(actual, 1))
	}

	return event.After.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchDesiredLRPRemovedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPRemovedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPRemovedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPRemovedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPRemovedEventMatcher matcher expects a models.DesiredLRPRemovedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	return event.DesiredLrp.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchActualLRPCreatedEvent(processGuid string, index int) gomega.OmegaMatcher {
	return &ActualLRPCreatedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

type ActualLRPCreatedEventMatcher struct {
	ProcessGuid string
	Index       int
}

func (matcher *ActualLRPCreatedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPCreatedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPCreatedEventMatcher matcher expects a models.ActualLRPCreatedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	actualLRP, _ := event.ActualLrpGroup.Resolve()
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index), nil
}

func (matcher *ActualLRPCreatedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPCreatedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPCreatedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPCreatedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

//

func MatchActualLRPChangedEvent(processGuid string, index int, state string) gomega.OmegaMatcher {
	return &ActualLRPChangedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
	}
}

type ActualLRPChangedEventMatcher struct {
	ProcessGuid string
	Index       int
	State       string
}

func (matcher *ActualLRPChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPChangedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPChangedEventMatcher matcher expects a models.ActualLRPChangedEvent.  Got:\n%s", format.Object(actual, 1))
	}

	actualLRP, _ := event.After.Resolve()
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index) && actualLRP.State == matcher.State, nil
}

func (matcher *ActualLRPChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

func (matcher *ActualLRPChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

//

func MatchActualLRPRemovedEvent(processGuid string, index int) gomega.OmegaMatcher {
	return &ActualLRPRemovedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

type ActualLRPRemovedEventMatcher struct {
	ProcessGuid string
	Index       int
}

func (matcher *ActualLRPRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPRemovedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPRemovedEventMatcher matcher expects a models.ActualLRPRemovedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	actualLRP, _ := event.ActualLrpGroup.Resolve()
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index), nil
}

func (matcher *ActualLRPRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}
