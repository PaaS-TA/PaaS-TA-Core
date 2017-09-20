require 'spec_helper'

describe Collector::Handler::Golang do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::Golang.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process" do
    let(:varz) do
      {
          "name" => "MetronAgent",
          "numCPUS" => 1,
          "numGoRoutines" => 1,
          "memoryStats" => {
            "numBytesAllocatedHeap" => 1024,
            "numBytesAllocatedStack" => 4096,
            "numBytesAllocated" => 2048,
            "numMallocs" => 3,
            "numFrees" => 10,
            "lastGCPauseTimeNS" => 1000
          },
          "contexts" => []
      }
    end

    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("MetronAgent.numCpus", 1)
      historian.should have_sent_data("MetronAgent.numGoRoutines", 1)
      historian.should have_sent_data("MetronAgent.memoryStats.numBytesAllocatedHeap", 1024)
      historian.should have_sent_data("MetronAgent.memoryStats.numBytesAllocatedStack", 4096)
      historian.should have_sent_data("MetronAgent.memoryStats.numBytesAllocated", 2048)
      historian.should have_sent_data("MetronAgent.memoryStats.numMallocs", 3)
      historian.should have_sent_data("MetronAgent.memoryStats.numFrees", 10)
      historian.should have_sent_data("MetronAgent.memoryStats.lastGCPauseTimeNS", 1000)
    end
  end
end
