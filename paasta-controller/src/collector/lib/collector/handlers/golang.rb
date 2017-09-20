module Collector
  class Handler
    class Golang < Handler
      def process(context)
        varz_message = context.varz
        component_name = varz_message['name']
        tags = varz_message['tags'] || {}

        send_metric("#{component_name}.numCpus", varz_message['numCPUS'], context, tags)
        send_metric("#{component_name}.numGoRoutines", varz_message['numGoRoutines'], context, tags)

        varz_message["memoryStats"].each_pair do |mem_stat_name, mem_stat_value|
          send_metric("#{component_name}.memoryStats.#{mem_stat_name}", mem_stat_value, context, tags)
        end
      end
    end
  end
end
