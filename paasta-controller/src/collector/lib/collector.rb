$:.unshift(File.expand_path(".", File.dirname(__FILE__)))

require "base64"
require "set"

require "rubygems"
require "bundler/setup"

require "eventmachine"
require "cf_message_bus/message_bus"
require "vcap/rolling_metric"

require "collector/config"
require "collector/handler"
require "collector/service_handler"
require "collector/service_node_handler"
require "collector/service_gateway_handler"
require "collector/tsdb_connection"
require "collector/historian"

Dir[File.join(File.dirname(__FILE__), "../lib/collector/handlers/*.rb")].each do |file|
  require File.join("collector/handlers", File.basename(file, File.extname(file)))
end

require "collector/components"

module Collector
  # Varz collector
  class Collector
    ANNOUNCE_SUBJECT = "vcap.component.announce".freeze
    DISCOVER_SUBJECT = "vcap.component.discover".freeze
    COLLECTOR_PING = "collector.nats.ping".freeze

    def initialize
      @components = {}
      @historian = ::Collector::Historian.build
      @nats_latency = VCAP::RollingMetric.new(60)

      @nats = CfMessageBus::MessageBus.new(servers: Config.nats_uri, logger: Config.logger)
      # Send initially to discover what's already running
      @nats.subscribe(ANNOUNCE_SUBJECT) { |message| process_component_discovery(message) }

      @inbox = NATS.create_inbox
      @nats.subscribe(@inbox) { |message| process_component_discovery(message) }

      @nats.publish(DISCOVER_SUBJECT, "", @inbox)

      @nats.subscribe(COLLECTOR_PING) { |message| process_nats_ping(message) }

      setup_timers

    end

    # Configures the periodic timers for collecting varzs.
    def setup_timers
      EM.add_periodic_timer(Config.discover_interval) do
        @nats.publish(DISCOVER_SUBJECT, "", @inbox)
      end

      EM.add_periodic_timer(Config.varz_interval) { fetch_varz }
      EM.add_periodic_timer(Config.healthz_interval) { fetch_healthz }
      EM.add_periodic_timer(Config.prune_interval) { prune_components }

      EM.add_periodic_timer(Config.local_metrics_interval) do
        send_local_metrics
      end

      EM.add_periodic_timer(Config.nats_ping_interval) do
        @nats.publish(COLLECTOR_PING, { timestamp: Time.now.to_f.to_s })
      end
    end

    # Processes NATS ping in order to calculate NATS roundtrip latency
    #
    # @param [Float] ping_timestamp UNIX timestamp when the ping was sent
    def process_nats_ping(ping_message)
      @nats_latency << ((Time.now.to_f - ping_message[:timestamp].to_f) * 1000).to_i
    end

    # Processes a discovered component message, recording it's location for
    # varz/healthz probes.
    #
    # @param [Hash] message the discovery message
    def process_component_discovery(message)
      if message["index"]
        Config.logger.debug1("collector.component.discovered", type: message["type"], index: message["index"], host: message["host"])
        instances = (@components[message["type"]] ||= {})
        instances[message["host"].split(":").first] = {
          :host => message["host"],
          :index => message["index"],
          :credentials => message["credentials"],
          :timestamp => Time.now.to_i
        }
      end
    rescue => e
      Config.logger.warn("collector.component.discovery-failure", error: e.message, backtrace: e.backtrace)
    end

    # Prunes components that haven't been heard from in a while
    def prune_components
      @components.each do |_, instances|
        instances.delete_if do |_, component|
          Time.now.to_i - component[:timestamp] > Config.prune_interval
        end
      end

      @components.delete_if { |_, instances| instances.empty? }
    rescue => e
      Config.logger.warn("collector.component.pruning-error", error: e.message, backtrace: e.backtrace)
    end

    # Generates metrics that don't require any interactions with varz or healthz
    def send_local_metrics
      context = HandlerContext.new(Config.index, Time.now.to_i, {})
      handler = Handler.handler(@historian, "collector")
      Config.logger.info("collector.nats-latency.sending")
      handler.send_latency_metric("nats.latency.1m", @nats_latency.value, context)
    end

    # Fetches the varzs from all the components and calls the proper {Handler}
    # to record the metrics in the TSDB server
    def fetch_varz
      fetch(:varz) do |resp, job, instance|
        index = instance[:index]
        varz = Yajl::Parser.parse(resp)
        now = Time.now.to_i

        handler = Handler.handler(@historian, job)
        Config.logger.debug("collector.job.process", job: job, handler: handler.class.name)
        ctx = HandlerContext.new(index, now, varz)
        handler.do_process(ctx)
      end
    end

    # Fetches the healthz from all the components and calls the proper {Handler}
    # to record the metrics in the TSDB server
    def fetch_healthz
      fetch(:healthz) do |resp, job, instance|
        index = instance[:index]
        host = instance[:host]
        is_healthy = resp.strip.downcase == "ok" ? 1 : 0
        send_healthz_metric(is_healthy, job, index, host)
      end
    end

    def credentials_ok?(job, instance)
      unless instance[:credentials].kind_of?(Array)
        Config.logger.warn("collector.credentials.invalid", job: job, instance: instance)
        return false
      end
      true
    end

    # Generates the authorization headers for a specific instance
    #
    # @param [Hash] instance hash
    # @return [Hash] headers
    def authorization_headers(instance)
      credentials = Base64.strict_encode64(instance[:credentials].join(":"))

      {
        "Authorization" => "Basic #{credentials}"
      }
    end

    private

    def fetch(type)
      @components.each do |job, instances|
        instances.each do |index, instance|
          next unless credentials_ok?(job, instance)

          host = instance[:host]
          uri  = "http://#{host}/#{type}"

          Config.logger.debug(
            "collector.#{type}.update",
            :host     => host, :index => index, :uri => uri,
            :instance => instance.inspect)

          EM.defer do
            begin
              response = HTTParty.get(uri, headers: authorization_headers(instance))
            rescue => e
              log_fetch_error(type, host, e.message)
              next
            end

            if response.ok?
              EM.next_tick do
                begin
                  yield response.body, job, instance
                rescue => e
                  Config.logger.error(
                    "collector.#{type}.processing-failed",
                    error:         e,
                    backtrace:     e.backtrace,
                    request_uri:   uri,
                    response:      response.body,
                    response_code: response.code
                  )
                end
              end
            else
              log_fetch_error(type, host, response.body)
            end
          end
        end
      end
    end

    def log_fetch_error(type, host, message)
      Config.logger.warn(
        "collector.#{type}.failed",
        :host => host, :error => message)
    end

    def send_healthz_metric(is_healthy, job, index, host)
      Config.logger.info("collector.healthz-metrics.sending", job: job, index: index)
      @historian.send_data({
        key: "healthy",
        timestamp: Time.now.to_i,
        value: is_healthy,
        tags: Components.get_job_tags(job).merge({job: job, index: index, deployment: Config.deployment_name, ip: host.split(":").first})
      })
    end

  end
end
