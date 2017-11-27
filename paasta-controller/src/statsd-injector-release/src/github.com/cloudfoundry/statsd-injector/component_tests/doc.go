//go:generate hel
package component_tests

import (
	v2 "github.com/cloudfoundry/statsd-injector/internal/plumbing/v2"
)

type MetronIngressServer interface {
	v2.IngressServer
}
