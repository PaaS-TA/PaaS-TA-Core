require 'time'
require 'aws-sdk'

module Collector
  class Historian
    class CloudWatch
      def initialize(access_key_id, access_secret_key)
        Aws.config.update({
            region: 'us-east-1',
            credentials: Aws::Credentials.new(access_key_id, access_secret_key),
           })
      end

      def send_data (data)
        time = data.fetch(:timestamp, Time.now.to_i)
        dimensions = data[:tags].flat_map do |key, value|
          Array(value).map do |v|
            {name: key.to_s, value: v.to_s}
          end
        end

        metric = {
            namespace: "CF/Collector",
            metric_data: [
                {
                    metric_name: data[:key].to_s,
                    value: data[:value].to_s,
                    timestamp: Time.at(time).utc.iso8601,
                    dimensions: dimensions
                }]
        }

        EventMachine.defer do
          Config.logger.info("collector.sending.cloudwatch")
          cloud_watch = Aws::CloudWatch::Client.new
          unless cloud_watch.respond_to?(:put_metric_data)
            Config.logger.warn("collector.cloud-watch-historian.weird-state")
            return
          end
          cloud_watch.put_metric_data metric
        end
      end
    end
  end
end