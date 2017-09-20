require "spec_helper"

describe Collector::Collector do
  let(:collector) do
    Collector::Config.tsdb_host = "dummy"
    Collector::Config.tsdb_port = 14242
    Collector::Config.nats_uri  = ["nats://foo:bar@nats-host:14222"]
    EventMachine.stub(:connect)
    Collector::Collector.new
  end

  before do
    message_bus = CfMessageBus::MockMessageBus.new
    CfMessageBus::MessageBus.stub(:new).and_return(message_bus)
    EM.stub(:add_periodic_timer)
  end

  describe "component discovery" do
    it "should record components when they announce themeselves" do
      create_fake_collector do |collector, _|
        components = collector.instance_eval { @components }
        components.should be_empty

        Time.should_receive(:now).at_least(1).and_return(Time.at(1311979380))

        collector.process_component_discovery({
            "type"        => "Test",
            "index"       => 1,
            "host"        => "test-host:1234",
            "credentials" => ["user", "pass"]
          })

        components.should == {
          "Test" => {
            "test-host" => {
              :host        => "test-host:1234",
              :index       => 1,
              :credentials => ["user", "pass"],
              :timestamp   => 1311979380
            }
          }
        }
      end
    end
  end

  describe "pruning components" do
    it "should prune old components" do
      create_fake_collector do |collector, _, _|
        Collector::Config.prune_interval = 10

        components = collector.instance_eval { @components }
        components.should be_empty

        collector.process_component_discovery({
            "type"        => "Test",
            "index"       => 1,
            "host"        => "test-host-1:1234",
            "credentials" => ["user", "pass"]
          })

        collector.process_component_discovery({
            "type"        => "Test",
            "index"       => 2,
            "host"        => "test-host-2:1234",
            "credentials" => ["user", "pass"]
          })

        components["Test"]["test-host-1"][:timestamp] = 100000
        components["Test"]["test-host-2"][:timestamp] = 100005

        Time.should_receive(:now).at_least(1).and_return(Time.at(100011))

        collector.prune_components

        components.should == {
          "Test" => {
            "test-host-2" => {
              :host        => "test-host-2:1234",
              :index       => 2,
              :credentials => ["user", "pass"],
              :timestamp   => 100005
            }
          }
        }
      end
    end
  end

  describe "fetch varz" do
    before do
      collector.process_component_discovery(
        "type"        => "Test",
        "index"       => 0,
        "host"        => "test-host:1234",
        "credentials" => ["user", "pass"]
      )

      stub_request(:get, "http://user:pass@test-host:1234/varz").
        to_return(body: response_body, status: response_status)

      allow(EM).to receive(:next_tick) { |&blk| blk.call }
    end

    let(:response_body) { '{"foo": "bar"}' }
    let(:response_status) { 200 }

    subject(:fetch_varz) { collector.fetch_varz }

    context "when a normal varz returns succesfully" do
      it "hits the correct endpoint" do
        fetch_varz

        expect(a_request(:get, "http://user:pass@test-host:1234/varz")).to have_been_made
      end

      it "gives the message to the correct handler" do
        Timecop.freeze(Time.now) do
          handler = double(:handler)
          allow(Collector::Handler).to receive(:handler).with(anything, anything).and_return(handler)
          allow(handler).to receive(:do_process)

          fetch_varz

          expect(handler).to have_received(:do_process).with(Collector::HandlerContext.new(0, Time.now.to_i, { "foo" => "bar" }))
        end
      end
    end

    context "when the varz has json errors" do
      let(:response_body) { 'foo' }
      it 'should log the error' do
        allow(Collector::Config.logger).to receive(:error)

        fetch_varz

        expect(Collector::Config.logger).to have_received(:error).with(
            'collector.varz.processing-failed',
            hash_including(
              error:         instance_of(Yajl::ParseError),
              response:      'foo',
              request_uri:   match(/http.*varz/),
              response_code: 200
            )
          )
      end
    end

    context "when the varz does not return succefully" do
      let(:response_status) { 404 }
      let(:response_body) { 'some error' }

      it "should log the failure" do
        allow(Collector::Config.logger).to receive(:warn)

        fetch_varz

        expect(Collector::Config.logger).to have_received(:warn).with(
            'collector.varz.failed',
            hash_including(
              host:  "test-host:1234",
              error: 'some error')
          )
      end
    end

    context "when the varz request raises an exception" do
      before do
        stub_request(:get, "http://user:pass@test-host:1234/varz").to_raise(StandardError.new("error message"))
      end

      it "should log the failure" do
        allow(Collector::Config.logger).to receive(:warn)

        fetch_varz

        expect(Collector::Config.logger).to have_received(:warn).with(
            'collector.varz.failed',
            hash_including(
              host:  "test-host:1234",
              error: "error message")
          )
      end
    end
  end

  describe "fetch healthz" do
    before do
      collector.process_component_discovery(
        "type"        => "Test",
        "index"       => 0,
        "host"        => "test-host:1234",
        "credentials" => ["user", "pass"]
      )

      stub_request(:get, "http://user:pass@test-host:1234/healthz").
        to_return(body: response_body, status: response_status)

      allow(EM).to receive(:next_tick) { |&blk| blk.call }
    end

    let(:response_body) { '{"foo": "bar"}' }
    let(:response_status) { 200 }

    subject(:fetch_healthz) { collector.fetch_healthz }

    context "when a normal healthz returns succesfully" do
      before { allow(Collector::Config).to receive(:deployment_name).and_return("the_deployment") }

      it "hits the correct endpoint" do

        fetch_healthz
        expect(a_request(:get, "http://user:pass@test-host:1234/healthz")).to have_been_made

      end

      context "with bad health" do
        let(:response_body) { 'bad' }

        it "directly sends the bad health out" do
          Timecop.freeze(Time.now) do

            expect_any_instance_of(Collector::Historian).to receive(:send_data).with(
                key:       "healthy",
                timestamp: Time.now.to_i,
                value:     0,
                tags:      { job: "Test", index: 0, deployment: "the_deployment", ip: "test-host" }
              )

            fetch_healthz
          end
        end
      end

      context "with good health" do
        let(:response_body) { 'ok' }

        it "directly sends the good health out" do
          Timecop.freeze(Time.now) do

            expect_any_instance_of(Collector::Historian).to receive(:send_data).with(
                key:       "healthy",
                timestamp: Time.now.to_i,
                value:     1,
                tags:      { job: "Test", index: 0, deployment: "the_deployment", ip: "test-host" }
              )

            fetch_healthz
          end
        end
      end
    end

    context "when the healthz does not return succefully" do
      let(:response_body) { "404 not found" }
      let(:response_status) { 404 }

      it "should log the failure" do
        allow(Collector::Config.logger).to receive(:warn)

        fetch_healthz

        expect(Collector::Config.logger).to have_received(:warn).with(
            "collector.healthz.failed",
            :host => "test-host:1234", :error => "404 not found")
      end
    end
  end

  describe "local metrics" do
    def send_local_metrics
      allow(Time).to receive(:now) { Time.at(1000) }

      create_fake_collector do |collector, _, _|
        collector.process_nats_ping(timestamp: 997)
        collector.process_nats_ping(timestamp: 998)
        collector.process_nats_ping(timestamp: 999)

        handler = double(:Handler)
        yield handler

        Collector::Handler.should_receive(:handler).
          with(kind_of(Collector::Historian), "collector").
          and_return(handler)

        collector.send_local_metrics
      end
    end

    it "should send nats latency rolling metric" do
      send_local_metrics do |handler|
        latency = { :value => 6000, :samples => 3 }
        handler.should_receive(:send_latency_metric).with("nats.latency.1m", latency, kind_of(Collector::HandlerContext))
      end
    end
  end

  describe "authorization headers" do
    it "should correctly encode long credentials (no CR/LF)" do
      create_fake_collector do |collector, _, _|
        collector.authorization_headers({ :credentials => ["A" * 64, "B" * 64] }).
          should == {
          "Authorization" =>
            "Basic QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFB" +
              "QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQTpCQkJCQkJC" +
              "QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJC" +
              "QkJCQkJCQkJCQkJCQkJCQkJCQkJC" }
      end
    end
  end

  describe "nats latency" do
    let(:historian) { double(:historian) }

    it 'should report metrics' do
      Collector::Historian.stub(:build).and_return(historian)
      collector.process_nats_ping(timestamp: (Time.now + 50).to_f.to_s)

      historian.should_receive(:send_data)
      collector.send_local_metrics
    end
  end
end
