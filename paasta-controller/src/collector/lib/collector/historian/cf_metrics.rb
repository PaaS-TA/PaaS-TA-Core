module Collector
  class Historian
    class CfMetrics
      def initialize(api_host, http_client)
        @api_host = api_host
        @http_client = http_client
        @timestamp_of_last_post = Time.now
      end

      def send_data(data)
        name, metric_data = formatted_metric_for_data(data)
        send_metrics(name, metric_data)
      end

      private

      def formatted_metric_for_data(data)
        name = data[:key]

        metrics = { deployment: ::Collector::Config.deployment_name}

        metrics[:value] = data[:value]

        data[:tags].each do |k,v|
          metrics[k] = v
        end

        [ name, metrics]
      end

      def send_metrics(metric_name, metric_data)
        Config.logger.debug("Sending metrics to cf-metrics: [#{metric_data.inspect}]")
        body = Yajl::Encoder.encode(metric_data)
        response = @http_client.put("#{@api_host}/metrics/#{metric_name}/values", body: body, headers: {"Content-type" => "application/json"})
        if response.success?
          Config.logger.info("collector.emit-cfmetrics.success", number_of_metrics: 1, lag_in_seconds: 0)
        else
          Config.logger.warn("collector.emit-cfmetrics.fail", number_of_metrics: 1, lag_in_seconds: 0)
        end
      end
    end
  end
end
