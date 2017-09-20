module Collector
  class Handler
    class HealthManager < Handler
      def process(context)
        varz = context.varz
        total_varz = varz["total"]
        send_metric("total.apps", total_varz["apps"], context)
        send_metric("total.started_apps", total_varz["started_apps"], context)
        send_metric("total.instances", total_varz["instances"], context)
        send_metric("total.started_instances", total_varz["started_instances"], context)
        send_metric("total.memory", total_varz["memory"], context)
        send_metric("total.started_memory", total_varz["started_memory"], context)

        running_varz = varz["running"]
        send_metric("running.crashes", running_varz["crashes"], context)
        send_metric("running.running_apps", running_varz["apps"], context)
        send_metric("running.running_instances", running_varz["running_instances"], context)
        send_metric("running.missing_instances", running_varz["missing_instances"], context)
        send_metric("running.flapping_instances", running_varz["flapping_instances"], context)

        send_metric("hm.time_to_analyze_all_droplets_in_seconds", varz["analysis_loop_duration"], context)
        send_metric("hm.time_to_retrieve_desired_state_in_seconds", varz["bulk_update_loop_duration"], context)

        send_metric("hm.total_heartbeat_messages_received", varz["heartbeat_msgs_received"], context)
        send_metric("hm.total_droplet_exited_messages_received", varz["droplet_exited_msgs_received"], context)
        send_metric("hm.total_droplet_update_messages_received", varz["droplet_updated_msgs_received"], context)
        send_metric("hm.total_status_messages_received", varz["healthmanager_status_msgs_received"], context)
        send_metric("hm.total_health_request_messages_received", varz["healthmanager_health_request_msgs_received"], context)
        send_metric("hm.total_droplet_request_messages_received", varz["healthmanager_droplet_request_msgs_received"], context)
        send_metric("hm.total_health_start_messages_sent", varz["health_start_messages_sent"], context)
        send_metric("hm.total_health_stop_messages_sent", varz["health_stop_messages_sent"], context)
      end
    end
  end
end
