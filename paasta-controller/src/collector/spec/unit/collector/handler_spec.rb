require "spec_helper"

describe Collector::Handler do
  describe "#handler" do
    it "should return the default handler when none registered" do
      Collector::Handler.handler(nil, "Test").should be_kind_of(Collector::Handler)
    end
  end

  describe "#do_process" do
    it "calls #process defined by the subclass" do
      context = Collector::HandlerContext.new(nil, nil, {})
      handler = Collector::Handler.new(nil, nil)
      handler.should_receive(:process).with(context)
      handler.do_process(context)
    end

    %w(mem_bytes mem_used_bytes mem_free_bytes cpu_load_avg).each do |stat|
      it "sends out '#{stat}' if specified" do
        context = Collector::HandlerContext.new(nil, nil, {stat => 2048})
        handler = Collector::Handler.new(nil, nil)
        handler.should_receive(:send_metric).with(stat, 2048, context)
        handler.do_process(context)
      end
    end

    it "sends out 'uptime' if specified" do
      context = Collector::HandlerContext.new(nil, nil, {"uptime" => "3d:3h:3m:56s"})
      handler = Collector::Handler.new(nil, nil)

      uptime_in_seconds = 56 + (60 * 3) + (60 * 60 * 3) + (60 * 60 * 24 * 3)
      handler.should_receive(:send_metric).with("uptime_in_seconds", uptime_in_seconds, context)

      handler.do_process(context)
    end

    it "sends out log counts if specified" do
      context = Collector::HandlerContext.new(nil, nil, {"log_counts" => { "fatal" => 5, "error" => 4, "warn" => 3, "info" => 2, "debug" => 1}})
      handler = Collector::Handler.new(nil, nil)
      handler.should_receive(:send_metric).with("log_count", 5, context, {"level" => "fatal"})
      handler.should_receive(:send_metric).with("log_count", 4, context, {"level" => "error"})
      handler.should_receive(:send_metric).with("log_count", 3, context, {"level" => "warn"})

      handler.do_process(context)
    end

    describe "sent tags" do
      let(:historian) { double }
      let(:handler) { Collector::Handler.new(historian, "DEA") }
      let(:context) { Collector::HandlerContext.new(0, nil, {"cpu_load_avg" => "42"}) }

      it "adds extra tags when specified" do
        handler.stub(:additional_tags => {foo: "bar"})
        historian.should_receive(:send_data).with(hash_including(
          tags: hash_including({
            foo: "bar"
          })
        ))
        handler.do_process(context)
      end

      it "sends the common tags" do
        historian.should_receive(:send_data).with(hash_including(
          tags: hash_including({
            job: "DEA",
            index: 0,
            role: "core"
          })
        ))
        handler.do_process(context)
      end
    end
  end

  describe "#send_metric" do
    it "should send the metric to the Historian" do
      historian = double('Historian')
      historian.should_receive(:send_data).with(
        key: "some_key",
        timestamp: 10000,
        value: 2,
        tags: {index: 1, job: "Test", name: "Test/1", deployment: "untitled_dev", foo: "bar"}
      )

      context = Collector::HandlerContext.new(1, 10000, {})
      handler = Collector::Handler.handler(historian, "Test")
      handler.send_metric("some_key", 2, context, {foo: "bar"})
    end

    it "should not allow additional_tags to override base tags" do
      historian = double('Historian')
      historian.should_receive(:send_data).with(
        key: "some_key",
        timestamp: 10000,
        value: 2,
        tags: {index: 1, job: "DEA", name: "DEA/1", deployment: "untitled_dev", role: "core"}
      )

      context = Collector::HandlerContext.new(1, 10000, {})
      handler = Collector::Handler.handler(historian, "DEA")
      handler.stub(:additional_tags => {
        job: "foo",
        index: "foo",
        deployment: "foo",
        role: "foo"
      })
      handler.send_metric("some_key", 2, context)
    end

    context 'when the component provides its own tags' do
      it 'does not overwrite them' do
        historian = double('Historian')
        historian.should_receive(:send_data).with(
            key: "some_key",
            timestamp: 10000,
            value: 2,
            tags: {index: 2, job: "provided", name: "custom-name", deployment: "untitled_dev", role: "core"}
        )

        context = Collector::HandlerContext.new(1, 10000, {})
        handler = Collector::Handler.handler(historian, "DEA")
        handler.stub(:additional_tags => {
                         job: "foo",
            index: "foo",
            name: "foo",
            deployment: "foo",
            role: "foo"
        })

        provided_tags = {
            index: 2,
            job: "provided",
            name: "custom-name"
        }
        handler.send_metric("some_key", 2, context, provided_tags)
      end

      it 'stringifies all keys before computing tags' do
        historian = double('Historian')
        historian.should_receive(:send_data).with(
            key: "some_key",
            timestamp: 10000,
            value: 2,
            tags: {index: 2, job: "provided", name: "provided/2", deployment: "untitled_dev", role: "core"}
        )

        context = Collector::HandlerContext.new(1, 10000, {})
        handler = Collector::Handler.handler(historian, "DEA")
        handler.stub(:additional_tags => {
                         job: "foo",
            index: "foo",
            deployment: "foo",
            role: "foo"
        })

        provided_tags = {
            "index" => 2,
            "job" => "provided"
        }
        handler.send_metric("some_key", 2, context, provided_tags)
      end
    end

    it "should not send metrics without a value" do
      historian = double('Historian')
      historian.should_not_receive(:send_data)
      Collector::Config.logger.should_receive(:warn).with("Received no value for some_key")

      handler = Collector::Handler.handler(historian, "DEA")
      handler.send_metric("some_key", nil , nil)
    end
  end

  describe "#send_latency_metric" do
    it "should send the metric to the historian with string keys" do
      historian = double("historian")
      historian.should_receive(:send_data).with(
        key: "latency_key",
        timestamp: 10000,
        value: 5,
        tags: hash_including({index: 1, job: "Test", foo: "bar"})
      )
      context = Collector::HandlerContext.new(1, 10000, {})
      handler = Collector::Handler.handler(historian, "Test")
      handler.send_latency_metric("latency_key", {"value" => 10, "samples" => 2}, context, {foo: "bar"})
    end

    it "should send the metric to the historian with symbolized keys" do
      historian = double("historian")
      historian.should_receive(:send_data).with(
        key: "latency_key",
        timestamp: 10000,
        value: 5,
        tags: hash_including({index: 1, job: "Test", foo: "bar"})
      )
      context = Collector::HandlerContext.new(1, 10000, {})
      handler = Collector::Handler.handler(historian, "Test")
      handler.send_latency_metric("latency_key", {:value => 10, :samples => 2}, context, {foo: "bar"})
    end
  end
end
