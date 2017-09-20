require 'spec_helper'

describe Collector::Handler::Runtime do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::Runtime.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process" do
    let(:varz) { fixture(:runtime) }

    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("Runtime.metricA.SomeValueA", 40)
      historian.should have_sent_data("Runtime.metricA.SomeValueB", 41, foo: "bar")
      historian.should have_sent_data("Runtime.metricB.SomeValueA", 40)
      historian.should have_sent_data("Runtime.metricB.SomeValueB", 41, foo: "bar")
    end
  end
end
