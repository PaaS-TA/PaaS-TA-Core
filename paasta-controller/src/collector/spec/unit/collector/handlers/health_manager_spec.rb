require 'spec_helper'

describe "Collector::Handler::HealthManager" do
  let(:varz) do
    {
      "running" => {
        "flapping_instances" => 11,
        "missing_instances" => 13,
        "running_instances" => 88,
        "crashes" => 77,
        "apps" => 98
      },
      "heartbeat_msgs_received" => 1716,
      "droplet_exited_msgs_received" => 128,
      "droplet_updated_msgs_received" => 68,
      "healthmanager_status_msgs_received" => 451,
      "healthmanager_health_request_msgs_received" => 35,
      "healthmanager_droplet_request_msgs_received" => 18,
      "health_start_messages_sent" => 32,
      "health_stop_messages_sent" => 64,
      "analysis_loop_duration" => 60.5,
      "bulk_update_loop_duration" => 1.25,
      "total" => {
        "started_memory" => 32000,
        "memory" => 176000,
        "started_instances" => 112,
        "instances" => 1193,
        "started_apps" => 2,
        "apps" => 150
      }
    }
  end
  let(:handler) { Collector::Handler::HealthManager.new(nil, nil) }

  it "sends metrics for every entry" do
    context = Collector::HandlerContext.new(nil, nil, varz)
    handler.should_receive(:send_metric).with("total.apps", 150, context)
    handler.should_receive(:send_metric).with("total.started_apps", 2, context)
    handler.should_receive(:send_metric).with("total.instances", 1193, context)
    handler.should_receive(:send_metric).with("total.started_instances", 112, context)
    handler.should_receive(:send_metric).with("total.memory", 176000, context)
    handler.should_receive(:send_metric).with("total.started_memory", 32000, context)
    handler.should_receive(:send_metric).with("running.crashes", 77, context)
    handler.should_receive(:send_metric).with("running.running_apps", 98, context)
    handler.should_receive(:send_metric).with("running.running_instances", 88, context)
    handler.should_receive(:send_metric).with("running.missing_instances", 13, context)
    handler.should_receive(:send_metric).with("running.flapping_instances", 11, context)

    handler.should_receive(:send_metric).with("hm.time_to_analyze_all_droplets_in_seconds", 60.5, context)
    handler.should_receive(:send_metric).with("hm.time_to_retrieve_desired_state_in_seconds", 1.25, context)

    handler.should_receive(:send_metric).with("hm.total_heartbeat_messages_received", 1716, context)
    handler.should_receive(:send_metric).with("hm.total_droplet_exited_messages_received", 128, context)
    handler.should_receive(:send_metric).with("hm.total_droplet_update_messages_received", 68, context)
    handler.should_receive(:send_metric).with("hm.total_status_messages_received", 451, context)
    handler.should_receive(:send_metric).with("hm.total_health_request_messages_received", 35, context)
    handler.should_receive(:send_metric).with("hm.total_droplet_request_messages_received", 18, context)
    handler.should_receive(:send_metric).with("hm.total_health_start_messages_sent", 32, context)
    handler.should_receive(:send_metric).with("hm.total_health_stop_messages_sent", 64, context)

    handler.process(context)
  end
end
