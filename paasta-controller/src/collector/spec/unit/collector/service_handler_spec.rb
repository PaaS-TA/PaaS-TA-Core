require File.expand_path("../../spec_helper", File.dirname(__FILE__))

describe Collector::ServiceHandler do
  let(:context) { Collector::HandlerContext.new(1, 10000, {}) }

  describe "send_metric" do
    it "should send the metric to the TSDB server with service & component tag" do
      historian = double("Historian")
      historian.should_receive(:send_data).with(
        hash_including(
          tags: hash_including(
            component: "unknown",
            service_type: "unknown",
          )
        )
      )
      handler = Collector::ServiceHandler.new(historian, "Test")
      handler.send_metric("some_key", 2, context)
    end
  end
end
