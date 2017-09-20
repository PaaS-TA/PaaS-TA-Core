require File.expand_path("../../../spec_helper", File.dirname(__FILE__))

describe Collector::Historian::Tsdb do
  describe "initialization" do
    it "connects to EventMachine" do
      EventMachine.should_receive(:connect).with("host", 9999, Collector::TsdbConnection)

      described_class.new("host", 9999)
    end
  end

  describe "sending data to TSDB" do
    let(:connection) { double('EventMachine connection') }

    before do
      EventMachine.stub(:connect).and_return(connection)
    end

    it "converts the properties hash into a tsdb 'put' command" do
      tsdb_historian = described_class.new("host", 9999)

      connection.should_receive(:send_data).with("put some_key 10000 2 component=unknown foo=bar foo=baz index=1 job=Test service_type=unknown tag=value\n")
      tsdb_historian.send_data({
              key: "some_key",
              timestamp: 10_000,
              value: 2,
              tags: {
                  index: 1,
                  component: "unknown",
                  service_type: "unknown",
                  job: "Test",
                  tag: "value",
                  foo: %w(bar baz)
              }
          })
    end
  end
end