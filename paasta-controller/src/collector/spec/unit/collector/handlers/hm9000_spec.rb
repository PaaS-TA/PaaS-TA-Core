require 'spec_helper'

describe Collector::Handler::HM9000 do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::HM9000.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process" do
    let(:varz) do
      {
          "name" => "HM9000",
          "numCPUS" => 1,
          "numGoRoutines" => 1,
          "tags" => {
              "ip" => "10.10.10.10"
          },
          "memoryStats" => {
            "numBytesAllocatedHeap" => 1024,
            "numBytesAllocatedStack" => 4096,
            "numBytesAllocated" => 2048,
            "numMallocs" => 3,
            "numFrees" => 10,
            "lastGCPauseTimeNS" => 1000
          },
          "contexts" => [
              {
                "name" => "null_metric",
                "metrics" => nil
              },
              {
                "name" => "HM9000",
                "metrics" => [
                   {"name" => "NumberOfAppsWithAllInstancesReporting", "value" => 120},
                   {"name" => "NumberOfAppsWithMissingInstances", "value" => 17},
                   {"name" => "NumberOfUndesiredRunningApps", "value" => 2},
                   {"name" => "NumberOfRunningInstances", "value" => 3},
                   {"name" => "NumberOfMissingIndices", "value" => 10},
                   {"name" => "NumberOfCrashedInstances", "value" => 8},
                   {"name" => "NumberOfCrashedIndices", "value" => 22}
                ]
              },
          ]
      }
    end


    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("HM9000.numCpus", 1)
      historian.should have_sent_data("HM9000.numGoRoutines", 1)
      historian.should have_sent_data("HM9000.memoryStats.numBytesAllocatedHeap", 1024)
      historian.should have_sent_data("HM9000.memoryStats.numBytesAllocatedStack", 4096)
      historian.should have_sent_data("HM9000.memoryStats.numBytesAllocated", 2048)
      historian.should have_sent_data("HM9000.memoryStats.numMallocs", 3)
      historian.should have_sent_data("HM9000.memoryStats.numFrees", 10)
      historian.should have_sent_data("HM9000.memoryStats.lastGCPauseTimeNS", 1000)
      historian.should have_sent_data("HM9000.HM9000.NumberOfAppsWithAllInstancesReporting", 120)
      historian.should have_sent_data("HM9000.HM9000.NumberOfAppsWithMissingInstances", 17)
      historian.should have_sent_data("HM9000.HM9000.NumberOfUndesiredRunningApps", 2)
      historian.should have_sent_data("HM9000.HM9000.NumberOfRunningInstances", 3)
      historian.should have_sent_data("HM9000.HM9000.NumberOfMissingIndices", 10)
      historian.should have_sent_data("HM9000.HM9000.NumberOfCrashedInstances", 8)
      historian.should have_sent_data("HM9000.HM9000.NumberOfCrashedIndices", 22)
    end
  end
end
