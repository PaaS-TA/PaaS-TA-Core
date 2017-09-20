require_relative "./historian/cloud_watch"
require_relative "./historian/cf_metrics"
require_relative "./historian/data_dog"
require_relative "./historian/tsdb"
require_relative "./historian/graphite"
require "httparty"

module Collector
  class Historian
    def self.build
      historian = new

      if Config.tsdb
        historian.add_adapter(Historian::Tsdb.new(Config.tsdb_host, Config.tsdb_port))
        Config.logger.info("collector.historian-adapter.added-opentsdb", host: Config.tsdb_host)
      end

      if Config.aws_cloud_watch
        historian.add_adapter(Historian::CloudWatch.new(Config.aws_access_key_id, Config.aws_secret_access_key))
        Config.logger.info("collector.historian-adapter.added-cloudwatch")
      end

      if Config.datadog
        historian.add_adapter(Historian::DataDog.new(Config.datadog_api_key, HTTParty))
        Config.logger.info("collector.historian-adapter.added-datadog")
      end

      if Config.cf_metrics
        historian.add_adapter(Historian::CfMetrics.new(Config.cf_metrics_api_host, HTTParty))
        Config.logger.info("collector.historian-adapter.added-cfmetrics")
      end

      if Config.graphite
        historian.add_adapter(Historian::Graphite.new(Config.graphite_host, Config.graphite_port))
        Config.logger.info("collector.historian-adapter.added-graphite", host: Config.graphite_host)
      end

      historian
    end

    def initialize
      @adapters = []
    end

    def send_data(data)
      @adapters.each do |adapter|
        begin
          adapter.send_data(data)
        rescue => e
          Config.logger.warn("collector.historian-adapter.sending-data-error", adapter: adapter.class.name, error: e, backtrace: e.backtrace)
        end
      end
    end

    def add_adapter(adapter)
      @adapters << adapter
    end
  end
end
