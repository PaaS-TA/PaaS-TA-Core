require 'spec_helper'

describe Collector::Handler::Contexts do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::Contexts.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process" do
    let(:varz) do
      {
          "name" => "MetronAgent",
          "tags" => {
              "ip" => "10.10.10.10"
          },
          "contexts" => [
              {"name" => "null_metric",
                "metrics" => nil},
              {"name" => "agentListener",
               "metrics" => [
                   {"name" => "currentBufferCount", "value" => 12},
                   {"name" => "receivedMessageCount", "value" => 45},
                   {"name" => "receivedByteCount", "value" => 6}]},
          ]
      }
    end

    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("MetronAgent.agentListener.currentBufferCount", 12)
      historian.should have_sent_data("MetronAgent.agentListener.receivedMessageCount", 45)
      historian.should have_sent_data("MetronAgent.agentListener.receivedByteCount", 6)
    end
  end
end
