# Auction

**Note**: This repository should be imported as `code.cloudfoundry.org/auction`.

####Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry/diego-design-notes)

The `auction` package in this repository encodes the details behind Diego's scheduling mechanism.  There are two components in Diego that participate in auctions:

- The [Auctioneer](https://github.com/cloudfoundry/auctioneer) is responsible for holding auctions whenever a Task or LongRunningProcess needs to be scheduled.
- The [Rep](https://github.com/cloudfoundry/rep) represents a Diego Cell in the auction by making bids and, if picked as the winner, running the Task or LongRunningProcess.

The Auctioneers run on the Diego "Brain" nodes, and there is only ever one active Auctioneer at a time (determined by acquiring a lock in [Consul](https://github.com/cloudfoundry-incubator/consul-release)). There is one Rep running on every Diego Cell.

The Auctioneer communicates with Reps on all Cells when holding an auction.

## The Auction Runner

The `auctionrunner` package provides an [*ifrit* process runner](https://github.com/tedsuo/ifrit/blob/master/runner.go) which consumes an incoming stream of requested auction work, batches it up, communicates with the Cell reps, picks winners, and then instructs the Cells to perform the work.

## The Simulation

The `simulation` package contains a Ginkgo test suite that describes a number of scheduling scenarios.  These scenarios can be run in a number of different modes, all controlled by passing flags to the test suite.  The `simulation` generates comprehensive output to the command line, and an SVG describing, visually, the results of the simulation run.

### In-Process Communication

By default, the simulation runs with an "in-process" communication model.  In this mode, the simulation spins up a number of in-process [`SimulationRep`](https://github.com/cloudfoundry/auction/blob/master/simulation/simulationrep/simulation_rep.go)s.  They implement the [Rep client interface](https://github.com/cloudfoundry-incubator/rep/blob/master/client.go#L41-L54).

This in-process communication mode allows us to isolate the algorithmic details from the communication details.  It allows us to iterate on the scoring math and scheduling details quickly and efficiently.

### HTTP Communication

The in-process model outlined above provides us with a starting point for analyzing the auction.  To understand the impact of HTTP communication, and ensure the HTTP layer works correctly, we can run the simulation with `ginkgo -- --communicationMode=http`.

When `communicationMode` is set to `http`, the simulation will spin up 100 `simulation/repnode` external processes.   The simulation then runs in-process auctions that communicate with these external processes via http.

### Running on Diego

Instead of running the simulations by running `ginkgo` locally, you can run the Diego scheduling simulations on a Diego deployment itself!  See the [Diego Cluster Simulations repository](https://github.com/pivotal-cf-experimental/diego-cluster-simulations).
