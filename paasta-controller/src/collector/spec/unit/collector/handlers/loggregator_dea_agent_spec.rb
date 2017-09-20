require 'spec_helper'

describe Collector::Handler::Router do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::LoggregatorDeaAgent.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process" do
    let(:varz) do
      {
          "name" => "LoggregatorDeaAgent",
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
              {"name" => "context1",
               "metrics" => [
                   {"name" => "metric1", "value" => 12},
                   {"name" => "metric2", "value" => 45},
                   {"name" => "metric3", "value" => 6}]},
              {"name" => "context2",
               "metrics" => [
                   {"name" => "metric4", "value" => 9, "tags" => {"tag1" => "tagValue1", "tag2" => "tagValue2"}}
               ]
              }
          ]
      }
    end


    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("LoggregatorDeaAgent.numCpus", 1)
      historian.should have_sent_data("LoggregatorDeaAgent.numGoRoutines", 1)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.numBytesAllocatedHeap", 1024)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.numBytesAllocatedStack", 4096)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.numBytesAllocated", 2048)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.numMallocs", 3)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.numFrees", 10)
      historian.should have_sent_data("LoggregatorDeaAgent.memoryStats.lastGCPauseTimeNS", 1000)
      historian.should have_sent_data("LoggregatorDeaAgent.context1.metric1", 12)
      historian.should have_sent_data("LoggregatorDeaAgent.context1.metric2", 45)
      historian.should have_sent_data("LoggregatorDeaAgent.context1.metric3", 6)

      historian.should have_sent_data("LoggregatorDeaAgent.context2.metric4", 9, {ip: "10.10.10.10", tag1: "tagValue1", tag2: "tagValue2"})
    end
  end
end
