module Collector
  class Handler
    class Dea < Handler
      def additional_tags(context)
        { stack: context.varz["stacks"], ip: context.varz["host"].split(":").first }
      end

      def process(context)
        send_metric("can_stage", context.varz["can_stage"], context)
        send_metric("reservable_stagers", context.varz["reservable_stagers"], context)
        send_metric("available_memory_ratio", context.varz["available_memory_ratio"], context)
        send_metric("available_disk_ratio", context.varz["available_disk_ratio"], context)

        state_counts(context).each do |state, count|
          send_metric("dea_registry_#{state.downcase}", count, context)
        end

        metrics = registry_usage(context)
        send_metric("dea_registry_mem_reserved", metrics[:mem], context)
        send_metric("dea_registry_disk_reserved", metrics[:disk], context)

        send_metric("total_warden_response_time_in_ms", context.varz["total_warden_response_time_in_ms"], context)
        send_metric("warden_request_count", context.varz["warden_request_count"], context)
        send_metric("warden_error_response_count", context.varz["warden_error_response_count"], context)
      end

      private

      DEA_STATES = %W[
        BORN STARTING RUNNING STOPPING STOPPED CRASHED RESUMING DELETED EVACUATING
      ].freeze

      def state_counts(context)
        metrics = DEA_STATES.each.with_object({}) { |s, h| h[s] = 0 }

        context.varz["instance_registry"].each do |_, instances|
          instances.each do |_, instance|
            metrics[instance["state"]] += 1
          end
        end

        metrics
      end

      RESERVING_STATES = %W[BORN STARTING RUNNING RESUMING EVACUATING].freeze

      def registry_usage(context)
        reserved_mem = reserved_disk = 0

        context.varz["instance_registry"].each do |_, instances|
          instances.each do |_, instance|
            if RESERVING_STATES.include?(instance["state"])
              reserved_mem += instance["limits"]["mem"]
              reserved_disk += instance["limits"]["disk"]
            end
          end
        end

        {mem: reserved_mem, disk: reserved_disk}
      end
    end
  end
end
