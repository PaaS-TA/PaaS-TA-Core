require_relative "service_handler"

module Collector
  class ServiceGatewayHandler < ServiceHandler
    def process(context)
      process_plan_score_metric(context)
      process_online_nodes(context)
      process_response_codes(context)
    end

    # Sum up all nodes' available_capacity value for each service, report
    # low_water & high_water value at the same time.
    #
    def process_plan_score_metric(context)
      varz = context.varz

      return unless varz.include?("plans")
      if varz["plans"]
        varz["plans"].each do |plan|
          tags = {
            :plan => plan["plan"],
          }
          allow_over_provisioning = plan.delete("allow_over_provisioning") ? 1 : 0
          send_metric("services.plans.allow_over_provisioning", allow_over_provisioning, context, tags)
          plan.each do |metric_name, value|
            send_metric("services.plans.#{metric_name}", value, context, tags)
          end
        end
      end
    end

    def process_response_codes(context)
      varz = context.varz

      return unless varz.has_key?("responses_metrics")
      varz.fetch("responses_metrics").each do |response_range, counter|
        response_code = response_range.split("_")[1]
        send_metric("services.http_status", counter, context, {status: response_code})
      end
    end

    # Get online nodes varz for each service gateway, report the total
    # number of online nodes
    #
    def process_online_nodes(context)
      varz = context.varz

      return unless varz.include?("nodes")
      send_metric("services.online_nodes", varz["nodes"].length, context)
    end

    def component
      "gateway"
    end
  end
end
