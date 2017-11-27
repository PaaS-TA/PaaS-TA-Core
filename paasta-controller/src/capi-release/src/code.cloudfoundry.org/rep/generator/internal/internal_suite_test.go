package internal_test

import (
	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
)

var (
	sqlRunner    sqlrunner.SQLRunner
	sqlProcess   ifrit.Process
	consulRunner *consulrunner.ClusterRunner
	bbsConfig    bbsconfig.BBSConfig
	bbsBinPath   string
	bbsRunner    *ginkgomon.Runner
	bbsProcess   ifrit.Process
	bbsClient    bbs.InternalClient
)

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}
