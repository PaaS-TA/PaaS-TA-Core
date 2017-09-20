module Collector
  class Handler
    class Contexts < Handler
      def process(context)
        varz_message = context.varz
        component_name = varz_message['name']
        tags = varz_message['tags'] || {}

        varz_message['contexts'].each do |message_context|
          context_name = message_context['name']
          metrics = message_context['metrics'] || []

          metrics.each do |metric|
            metric_name = metric['name']
            metric_value = metric['value']
            metric_tags = tags.merge(metric['tags'] || {})

            send_metric("#{component_name}.#{context_name}.#{metric_name}", metric_value, context, metric_tags)
          end
        end
      end
    end
  end
end
