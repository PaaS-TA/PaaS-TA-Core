package cc_conv

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

func StateFor(state, placementError string) cc_messages.LRPInstanceState {
	switch state {
	case models.ActualLRPStateUnclaimed:
		if placementError == "" {
			return cc_messages.LRPInstanceStateStarting
		}
		return cc_messages.LRPInstanceStateDown
	case models.ActualLRPStateClaimed:
		return cc_messages.LRPInstanceStateStarting
	case models.ActualLRPStateRunning:
		return cc_messages.LRPInstanceStateRunning
	case models.ActualLRPStateCrashed:
		return cc_messages.LRPInstanceStateCrashed
	default:
		return cc_messages.LRPInstanceStateUnknown
	}
}
