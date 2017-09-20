require "time"
require "collector/config"

module Collector
  class Historian
    class DataDog
      def initialize(api_key, http_client)
        @api_key = api_key
        @http_client = http_client

        @metrics = []
        @timestamp_of_last_post = Time.now
      end

      def send_data(data)
        @metrics << formatted_metric_for_data(data)

        time_since_last_post = Time.now - @timestamp_of_last_post
        if @metrics.size >= Config.datadog_data_threshold || time_since_last_post >= Config.datadog_time_threshold_in_seconds
          send_metrics(@metrics.dup)

          @metrics.clear
          @timestamp_of_last_post = Time.now
        end
      end

      private

      def metric_for_data(data)
        "cf.collector.#{data[:key].to_s}"
      end

      def formatted_metric_for_data(data)
        metric = metric_for_data(data)
        points = [[data.fetch(:timestamp, Time.now.to_i), data[:value]]]
        tags = data[:tags].flat_map do |key, value|
          Array(value).map do |v|
            "#{key}:#{v}"
          end
        end

        {
          metric: metric,
          points: points,
          type: "gauge",
          tags: tags
        }
      end

      def send_metrics(metrics)
        start = Time.now
        EM.defer do
          Config.logger.debug("Sending metrics to datadog: [#{metrics.inspect}]")
          body = Yajl::Encoder.encode({ series: metrics })
          response = @http_client.post("https://app.datadoghq.com/api/v1/series", query: {api_key: @api_key}, body: body, headers: {"Content-type" => "application/json"})
          if response.success?
            Config.logger.info("collector.emit-datadog.success", number_of_metrics: metrics.count, lag_in_seconds: Time.now - start)
          else
            Config.logger.warn("collector.emit-datadog.fail", number_of_metrics: metrics.count, lag_in_seconds: Time.now - start)
          end
        end
      end
    end
  end
end
