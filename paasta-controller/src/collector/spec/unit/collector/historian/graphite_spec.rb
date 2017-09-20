require File.expand_path("../../../spec_helper", File.dirname(__FILE__))

describe Collector::Historian::Graphite do
  describe "initialization" do
    it "connects to EventMachine" do
      EventMachine.should_receive(:connect).with("host", 9999, Collector::GraphiteConnection)

      described_class.new("host", 9999)
    end
  end

  describe "sending data to Graphite" do
    let(:connection) { double('EventMachine connection') }
    let(:metric_payload) do
      {
        key: "some_key",
        timestamp: 1234568912,
        value: 2,
        tags: {
          :ip => "1.2.3.4",
          :deployment => "CF",
          :job => "Blurgh",
          :index => 0
        }
      }
    end

    before do
      EventMachine.stub(:connect).and_return(connection)
    end

    it "converts the properties hash graphite data" do
      graphite_historian = described_class.new("host", 9999)

      connection.should_receive(:send_data).with("CF.Blurgh.0.1-2-3-4.some_key 2 1234568912\n")
      graphite_historian.send_data(metric_payload)
    end

    it "converts the properties hash graphite data even though the type of the ip field is a string" do
      metric_payload_with_string_type = {
        key: "some_key",
        timestamp: 1234568912,
        value: 2,
        tags: {
          "ip" => "1.2.3.4",
          :deployment => "CF",
          :job => "Blurgh",
          :index => 0
        }
      }
      graphite_historian = described_class.new("host", 9999)
      connection.should_receive(:send_data).with("CF.Blurgh.0.1-2-3-4.some_key 2 1234568912\n")
      graphite_historian.send_data(metric_payload_with_string_type)
    end


    it "send the event even if ip tags is missing" do
      metric_payload_with_string_type = {
        key: "some_key",
        timestamp: 1234568912,
        value: 2,
        tags: {
          :deployment => "CF",
          :job => "Blurgh",
          :index => 0
        }
      }
      graphite_historian = described_class.new("host", 9999)
      connection.should_receive(:send_data).with("CF.Blurgh.0.nil.some_key 2 1234568912\n")
      graphite_historian.send_data(metric_payload_with_string_type)
    end

    it "Should send router responses status code" do
      metric_payload_with_string_type = {
        key: "router.responses.2xx",
        timestamp: 1234568912,
        value: 2,
        tags: {
          "ip" => "1.2.3.4",
          :deployment => "CF",
          :job => "Router",
          :index => 0
        }
      }
      graphite_historian = described_class.new("host", 9999)
      connection.should_receive(:send_data).with("CF.Router.0.1-2-3-4.router.responses.2xx 2 1234568912\n")
      graphite_historian.send_data(metric_payload_with_string_type)
    end

    it "Should send router reponse status by component " do
      metric_payload_with_string_type = {
        key: "router.responses",
        timestamp: 1234568912,
        value: 2,
        tags: {
          "ip" => "1.2.3.4",
          :deployment => "CF",
          :job => "Router",
          :index => 0,
          :component => "CC",
          :status => "2xx"
        }
      }
      graphite_historian = described_class.new("host", 9999)
      connection.should_receive(:send_data).with("CF.Router.0.1-2-3-4.router.responses.CC.2xx 2 1234568912\n")
      graphite_historian.send_data(metric_payload_with_string_type)
    end

    context "when the passed in data is missing a timestamp" do
      it "uses now" do
        graphite_historian = described_class.new("host", 9999)
        metric_payload.delete(:timestamp)
        Timecop.freeze Time.now.to_i do
          connection.should_receive(:send_data).with("CF.Blurgh.0.1-2-3-4.some_key 2 #{Time.now.to_i}\n")
          graphite_historian.send_data(metric_payload)
        end
      end
    end


    context "when the passed in data has wrongly formatted timestamp" do
      it "uses now" do
        graphite_historian = described_class.new("host", 9999)

        metric_payload.update(:timestamp => "BLURGh!!11")
        Timecop.freeze Time.now.to_i do
          connection.should_receive(:send_data).with("CF.Blurgh.0.1-2-3-4.some_key 2 #{Time.now.to_i}\n")
          graphite_historian.send_data(metric_payload)
        end
      end
    end

    context "when the value is not a int or float" do
      it "should log and not do anything" do
        graphite_historian = described_class.new("host", 9999)
        metric_payload.update(:value => "BLURGh!!11")
        ::Collector::Config.logger.should_receive(:error).with("collector.emit-graphite.fail: Value is not a float or int, got: BLURGh!!11")
        graphite_historian.send_data(metric_payload)
      end
    end

    context "when there is a missing field from the properties" do
      it "cannot create a metrics key and should log the error" do
        graphite_historian = described_class.new("host", 9999)
        metric_payload.delete(:key)
        ::Collector::Config.logger.should_receive(:error).with("collector.create-graphite-key.fail: Could not create metrics name from fields tags.deployment, tags.job, tags.index or key.")
        graphite_historian.send_data(metric_payload)
      end
    end

  end
end
