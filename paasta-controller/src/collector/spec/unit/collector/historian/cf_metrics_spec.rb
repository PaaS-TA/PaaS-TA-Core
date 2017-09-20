require File.expand_path("../../../spec_helper", File.dirname(__FILE__))

class FakeResponse
  attr_accessor :success

  def initialize(success)
    @success = success
  end

  def success?
    @success
  end
end

class FakeHttpClient
  attr_reader :last_put
  attr_accessor :respond_successfully

  def initialize
    @respond_successfully = true
  end

  def put(path, options)
    @last_put = {
        path: path,
        options: options
    }

    FakeResponse.new(@respond_successfully)
  end

  def reset
    @last_put = nil
  end

  def parsed_put_body
    Yajl::Parser.parse(@last_put[:options][:body])
  end
end

describe Collector::Historian::CfMetrics do
  describe "sending data to cf metrics" do
    let(:data_threshold) { 50 }
    let(:time_threshold) { 20 }
    let(:fake_http_client) { FakeHttpClient.new }
    let(:cf_metrics_historian) do
      Timecop.freeze(Time.at(time)) do
        described_class.new("https://api.metrics.example.com", fake_http_client)
      end
    end
    let(:time) { Time.now.to_i }
    let(:cf_metric_payload) do
      {
          key: "some_metric.some_key",
          timestamp: time,
          value: 2,
          tags: {
              job: "Test",
              index: 1,
              component: "unknown",
              service_type: "unknown",
              tag: "value",
              foo: %w(bar baz)
          }
      }
    end
    let(:expected_tags) { %w[job:Test index:1 component:unknown service_type:unknown tag:value foo:bar foo:baz] }

    before do
      ::Collector::Config.stub(:deployment_name).and_return("dev114cw")

      @counter = 0
    end


    it "converts the properties hash into a cf metric metric" do
      ::Collector::Config.logger.should_not_receive(:warn)
      ::Collector::Config.logger.should_receive(:info).with("collector.emit-cfmetrics.success", number_of_metrics: 1, lag_in_seconds: 0)

      Timecop.freeze(Time.at(time + time_threshold)) do
        cf_metrics_historian.send_data(cf_metric_payload)
      end

      expected_json_hash = {"value" => 2,
                            "job" => "Test",
                            "index" =>  1,
                            "component" =>  "unknown",
                            "service_type" =>  "unknown",
                            "tag" => "value",
                            "foo" => %w(bar baz),
                            "deployment" => "dev114cw"
      }

      fake_http_client.last_put[:path].should == "https://api.metrics.example.com/metrics/some_metric.some_key/values"
      fake_http_client.last_put[:options][:headers].should == {"Content-type" => "application/json"}
      fake_http_client.parsed_put_body.should == expected_json_hash
    end

    context "when the api request fails" do
      it "logs" do
        fake_http_client.respond_successfully = false
        ::Collector::Config.logger.should_not_receive(:info)
        ::Collector::Config.logger.should_receive(:warn).with("collector.emit-cfmetrics.fail", number_of_metrics: 1, lag_in_seconds: 0)

        Timecop.freeze(Time.at(time + time_threshold)) do
          cf_metrics_historian.send_data(cf_metric_payload)
        end
      end
    end
  end
end
