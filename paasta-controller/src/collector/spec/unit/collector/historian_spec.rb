require File.expand_path("../../spec_helper", File.dirname(__FILE__))

describe Collector::Historian do
  let(:historian) { Collector::Historian.build }

  before do
    EventMachine.stub(:connect)
    Collector::Config.configure(config_override)
  end

  describe "setting up a historian with tsdb data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          'tsdb' => {
              'port' => 4242,
              'host' => "localhost"
          }
      }
    end

    it "builds a historian that logs to TSDB" do
      historian.should respond_to :send_data
    end
  end

  describe "configuring with aws data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          'aws_cloud_watch' => {
              'access_key_id' => "AWS_ACCESS_KEY12345",
              'secret_access_key' => "AWS_SECRET_ACCESS_KEY98765"
          }
      }
    end

    before do
      Aws.config.should_receive :update
      Collector::Config.configure(config_override)
    end

    it "builds a historian that logs to cloud watch" do
      historian = Collector::Historian.build
      historian.should respond_to :send_data
    end
  end

  describe "configuring with datadog data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          "datadog" => {
              "api_key" => "DATADOG_API_KEY",
              "application_key" => "DATADOG_APPLICATION_KEY"
          }
      }
    end

    let(:dog_historian) { double('DataDog historian') }

    before do
      Collector::Config.configure(config_override)
      Collector::Historian::DataDog.should_receive(:new).with("DATADOG_API_KEY", HTTParty).and_return(dog_historian)
    end

    it "builds a historian that logs to DataDog" do
      historian = Collector::Historian.build
      dog_historian.should_receive(:send_data)
      historian.send_data({tags:{}})
    end
  end

  describe "configuring with cf metrics data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          "cf_metrics" => {
              "host" => "api.metrics.example.com"
          }
      }
    end

    let(:cfmetrics_historian) { double('CfMetrics historian') }

    before do
      Collector::Config.configure(config_override)
      Collector::Historian::CfMetrics.should_receive(:new).with("api.metrics.example.com", HTTParty).and_return(cfmetrics_historian)
    end

    it "builds a historian that logs to Cf Metrics" do
      historian = Collector::Historian.build
      cfmetrics_historian.should_receive(:send_data)
      historian.send_data({tags:{}})
    end
  end

  describe "configuring with graphite data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          "graphite" => {
              "host" => "graphite.host.domain.com",
              "port" => 1234
          }
      }
    end

    it "builds a historian that logs to graphite" do
      historian.should respond_to :send_data
    end
  end


  describe "configuring with all" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          'aws_cloud_watch' => {
              'access_key_id' => "AWS_ACCESS_KEY12345",
              'secret_access_key' => "AWS_SECRET_ACCESS_KEY98765"
          },
          'tsdb' => {
              'port' => 4242,
              'host' => "localhost"
          },
          "datadog" => {
              "api_key" => "DATADOG_API_KEY"
          },
          "cf_metrics" => {
              "host" => "api.metrics.example.com"
          }
      }
    end

    before do
      Collector::Config.configure(config_override)
    end

    it "builds a historian that logs to all services" do
      Aws::Credentials.should_receive(:new).with("AWS_ACCESS_KEY12345", "AWS_SECRET_ACCESS_KEY98765")
      Aws.config.should_receive(:update).with({region: 'us-east-1',
                                               credentials: anything(),})
      EventMachine.should_receive(:connect).with("localhost", 4242, Collector::TsdbConnection)
      Collector::Historian::DataDog.should_receive(:new).with("DATADOG_API_KEY", HTTParty)
      Collector::Historian::CfMetrics.should_receive(:new).with("api.metrics.example.com", HTTParty)

      historian.should respond_to :send_data
    end
  end

  describe "when sending data" do
    let(:config_override) do
      {
          "intervals" => {},
          "logging" => {},
          'aws_cloud_watch' => {
              'access_key_id' => "AWS_ACCESS_KEY12345",
              'secret_access_key' => "AWS_SECRET_ACCESS_KEY98765"
          },
          'tsdb' => {
              'port' => 4242,
              'host' => "localhost"
          },
          "datadog" => {
              "api_key" => "DATADOG_API_KEY",
              "application_key" => "DATADOG_APPLICATION_KEY"
          },
          "cf_metrics" => {
              "host" => "api.metrics.example.com"
          }
      }
    end

    let(:connection) { double('Connection') }
    let(:cloud_watch) { double('Cloud Watch') }
    let(:dog_historian) { double('DataDog historian') }
    let(:cfmetrics_historian) { double('CfMetrics historian') }

    before do
      Aws.config.stub(:update)
      Aws::CloudWatch::Client.stub(:new).and_return(cloud_watch)
      EventMachine.stub(:connect).and_return(connection)
      Collector::Historian::DataDog.should_receive(:new).and_return(dog_historian)
      Collector::Historian::CfMetrics.should_receive(:new).with("api.metrics.example.com", HTTParty).and_return(cfmetrics_historian)
    end

    context "when one of the historians fail" do
      before { connection.should_receive(:send_data).and_raise("FAIL") }

      it "should still send data to the other historians" do
        cloud_watch.should_receive(:put_metric_data)
        dog_historian.should_receive(:send_data)
        cfmetrics_historian.should_receive(:send_data)

        expect { historian.send_data({tags: {}}) }.to_not raise_error
      end
    end
  end
end
