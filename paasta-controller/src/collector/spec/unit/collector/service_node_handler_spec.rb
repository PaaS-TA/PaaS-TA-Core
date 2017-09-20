require File.expand_path("../../spec_helper", File.dirname(__FILE__))

describe Collector::ServiceNodeHandler do
  it "has the right component type" do
    handler = Collector::ServiceNodeHandler.new(nil, nil)
    handler.component.should == "node"
  end

  describe "#process" do
    it "should call process_healthy_instances_metric" do
      context = Object.new
      handler = Collector::ServiceNodeHandler.new(nil, nil)
      handler.should_receive(:process_healthy_instances_metric).with(context)
      handler.process(context)
    end
  end

  describe :process_healthy_instances_metric do
    it "should report healthy instances percentage metric to TSDB server" do
      historian = double('Historian')
      historian.should_receive(:send_data).
        with({
        key: "services.healthy_instances",
        timestamp: 10_000,
        value: "50.00",
        tags: hash_including({
          component: "node",
          index: 1,
          job: "Test",
          service_type: "unknown"
        })
      })

      varz = {
        "instances" => {
          1 => 'ok',
          2 => 'fail',
          3 => 'fail',
          4 => 'ok'
        }
      }
      context = Collector::HandlerContext.new(1, 10000, varz)
      handler = Collector::ServiceNodeHandler.new(historian, "Test")
      handler.process_healthy_instances_metric(context)
    end
  end
end
