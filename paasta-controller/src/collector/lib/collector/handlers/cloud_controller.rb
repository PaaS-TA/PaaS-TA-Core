require 'matrix'

module Collector
  class Handler
    class CloudController < Handler
      DHMS_IN_SECS = [24 * 60 * 60, 60 * 60, 60, 1].freeze

      def additional_tags(context)
        { ip: context.varz["host"].split(":").first }
      end

      def process(context)
        varz = context.varz

        varz["vcap_sinatra"]["requests"].each do |key, value|
          send_metric("cc.requests.#{key}", value, context)
        end

        aggregate_http_status(varz).each do |key, value|
          send_metric("cc.http_status.#{key}", value, context)
        end

        send_metric("cc.uptime", uptime_in_seconds(varz), context)

        send_metric("total_users", varz["cc_user_count"], context)

        varz["cc_job_queue_length"] ||= {}
        varz["cc_job_queue_length"].each do |key, value|
          send_metric("cc.job_queue_length.#{key}", value, context)
        end

        varz["cc_failed_job_count"] ||= {}
        varz["cc_failed_job_count"].each do |key, value|
          send_metric("cc.failed_job_count.#{key}", value, context)
        end

        thread_info_metrics(varz['thread_info'], 'thread_info').each do |key, value|
          send_metric("cc.#{key}", value, context)
        end
      end

      private

      def uptime_in_seconds(varz)
        uptime_in_human = varz["uptime"].gsub("[dhms]", "").split(":").map(&:to_i)
        (Matrix.row_vector(DHMS_IN_SECS) * Matrix.column_vector(uptime_in_human)).element(0, 0)
      end

      def aggregate_http_status(varz)
        varz["vcap_sinatra"]["http_status"].group_by { |key, _| key[0] }.map do |key, value|
          value = value.inject(0) do |sum, (_, number_of_requests)|
            sum += number_of_requests
          end
          ["#{key}XX", value]
        end
      end

      def thread_info_metrics(hash, parent_key, result={})
        hash.each do |key, value|
          metric_name = "#{parent_key}.#{key}"
          if value.is_a? Hash
            thread_info_metrics(value, metric_name, result)
          else
            result[metric_name] = value
          end
        end

        result
      end
    end
  end
end
