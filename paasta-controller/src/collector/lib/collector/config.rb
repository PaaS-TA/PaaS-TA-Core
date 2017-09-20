require "steno"

module Collector
  # Singleton config used throughout
  class Config
    class << self
      attr_accessor :index, :tsdb_host, :tsdb_port, :aws_access_key_id,
        :aws_secret_access_key, :datadog_api_key, :datadog_application_key,
        :nats_uri, :discover_interval, :varz_interval, :healthz_interval,
        :prune_interval, :nats_ping_interval, :local_metrics_interval,
        :deployment_name, :datadog_data_threshold, :datadog_time_threshold_in_seconds, :cf_metrics_api_host,
        :graphite_host, :graphite_port

      def tsdb
        tsdb_host && tsdb_port
      end

      def aws_cloud_watch
        aws_access_key_id && aws_secret_access_key
      end

      def datadog
        datadog_api_key
      end

      def cf_metrics
        cf_metrics_api_host
      end

      def graphite
        graphite_host && graphite_port
      end

      def logger
        raise "logger was used without being configured" unless @logging_configured
        @logger ||= Steno.logger("collector")
      end

      def setup_logging(config={})
        log_counter = Steno::Sink::Counter.new
        cfg = Steno::Config.from_hash(config)
        cfg.sinks << log_counter
        Steno.init(cfg)
        @logging_configured = true
        logger.info("collector.started")
      end

      # Configures the various attributes
      #
      # @param [Hash] config the config Hash
      def configure(config)
        @index = config["index"].to_i
        setup_logging(config["logging"])

        @deployment_name = config["deployment_name"] || "untitled_dev"

        tsdb_config = config["tsdb"] || {}
        @tsdb_host = tsdb_config["host"]
        @tsdb_port = tsdb_config["port"]

        aws_config = config["aws_cloud_watch"] || {}
        @aws_access_key_id = aws_config["access_key_id"]
        @aws_secret_access_key = aws_config["secret_access_key"]

        datadog_config = config["datadog"] || {}
        @datadog_api_key = datadog_config["api_key"]
        @datadog_application_key = datadog_config["application_key"]
        @datadog_data_threshold = datadog_config.fetch("data_threshold", 1000).to_i
        @datadog_time_threshold_in_seconds = datadog_config.fetch("time_threshold_in_seconds", 10).to_i

        cf_metrics_config = config["cf_metrics"] || {}
        @cf_metrics_api_host = cf_metrics_config["host"]

        graphite_config = config["graphite"] || {}
        @graphite_host = graphite_config["host"]
        @graphite_port = graphite_config["port"]

        @nats_uri = config["message_bus_uris"]

        intervals = config["intervals"]

        @discover_interval = intervals["discover"] || 60
        @varz_interval = intervals["varz"] || 10
        @healthz_interval = intervals["healthz"] || 5
        @prune_interval = intervals["prune"] || 300
        @nats_ping_interval = intervals["nats_ping"] || 10
        @local_metrics_interval = intervals["local_metrics"] || 10
      end
    end
  end
end
