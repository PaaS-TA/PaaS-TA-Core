$:.unshift(File.expand_path("../lib", File.dirname(__FILE__)))

ENV["BUNDLE_GEMFILE"] ||= File.expand_path("../Gemfile", File.dirname(__FILE__))
require "rubygems"
require "bundler"
Bundler.setup(:default, :test)

if ENV["CODECLIMATE_REPO_TOKEN"]
  require "codeclimate-test-reporter"
  CodeClimate::TestReporter.start
end

require "rspec/core"
require "timecop"

require "collector/config"
require "cf_message_bus/mock_message_bus"
require "webmock/rspec"

require "collector"

def set_collector_base_config
  Collector::Config.configure(
    "logging" => {"level" => ENV["DEBUG"] ? "debug2" : "fatal"},
    "tsdb" => {},
    "intervals" => {}
  )
end

RSpec.configure do |c|
  c.before do
    allow(EventMachine).to receive(:defer).and_yield
    Collector::Handler.instance_map = {}
    set_collector_base_config
  end

  c.before(:all) { WebMock.disable_net_connect!(:allow => "codeclimate.com") }
  c.after(:all) { WebMock.disable_net_connect!(:allow => "codeclimate.com") }
end

class FakeHistorian
  attr_reader :data_lake

  def initialize
    @data_lake = []
  end

  def send_data(data)
    @data_lake << data
  end

  def has_sent_data?(key, value, tags={})
    @data_lake.any? do |data|
      data[:key] == key && data[:value] == value &&
        tags.all? { |k, v| data[:tags][k] == v }
    end
  end
end

def create_fake_collector
  Collector::Config.tsdb_host = "dummy"
  Collector::Config.tsdb_port = 14242
  Collector::Config.nats_uri = "nats://foo:bar@nats-host:14222"

  EventMachine.should_receive(:connect).
    with("dummy", 14242, Collector::TsdbConnection)

  nats_connection = CfMessageBus::MockMessageBus.new
  CfMessageBus::MessageBus.should_receive(:new).
    with(:servers => "nats://foo:bar@nats-host:14222", :logger => Collector::Config.logger).
    and_return(nats_connection)

  yield Collector::Collector.new, nats_connection
end

def fixture(name)
  Yajl::Parser.parse(File.read(File.expand_path("../fixtures/#{name}.json", __FILE__)))
end


def silence_warnings(&blk)
  warn_level = $VERBOSE
  $VERBOSE = nil
  blk.call
ensure
  $VERBOSE = warn_level
end
