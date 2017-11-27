package main

import (
	"flag"
	"fmt"
	"log"

	"code.cloudfoundry.org/auction/simulation/simulationrep"
	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	rephandlers "code.cloudfoundry.org/rep/handlers"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"
)

var memoryMB = flag.Int("memoryMB", 100, "total available memory in MB")
var diskMB = flag.Int("diskMB", 100, "total available disk in MB")
var containers = flag.Int("containers", 100, "total available containers")
var repGuid = flag.String("repGuid", "", "rep-guid")
var httpAddr = flag.String("httpAddr", "", "http server addres")
var stack = flag.String("stack", "", "stack")
var zone = flag.String("zone", "Z0", "availability zone")

func main() {
	lagerflags.AddFlags(flag.CommandLine)
	flag.Parse()

	if *repGuid == "" {
		panic("need rep-guid")
	}

	if *httpAddr == "" {
		panic("need http addr")
	}

	simulationRep := simulationrep.New(*stack, *zone, rep.Resources{
		MemoryMB:   int32(*memoryMB),
		DiskMB:     int32(*diskMB),
		Containers: *containers,
	}, []string{})

	logger, _ := lagerflags.New("repnode-http")

	fakeExecutorClient := new(executorfakes.FakeClient)
	fakeEvacuatable := new(fake_evacuation_context.FakeEvacuatable)

	handlers := rephandlers.New(simulationRep, fakeExecutorClient, fakeEvacuatable, logger.Session(*repGuid))
	router, err := rata.NewRouter(rep.Routes, handlers)
	if err != nil {
		log.Fatalln("failed to make router:", err)
	}
	httpServer := http_server.New(*httpAddr, router)

	monitor := ifrit.Invoke(sigmon.New(httpServer))
	fmt.Println("rep node listening")
	err = <-monitor.Wait()
	if err != nil {
		println("EXITED WITH ERROR: ", err.Error())
	}
}
