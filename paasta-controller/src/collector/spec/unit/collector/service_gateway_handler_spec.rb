require File.expand_path("../../spec_helper", File.dirname(__FILE__))

describe Collector::ServiceGatewayHandler do
  it "has the right component type" do
    handler = Collector::ServiceGatewayHandler.new(nil, nil)
    handler.component.should == "gateway"
  end

  describe "#process" do
    it "should call the other process methods" do
      handler = Collector::ServiceGatewayHandler.new(nil, nil)
      handler.should_receive(:process_plan_score_metric).with(context)
      handler.should_receive(:process_online_nodes).with(context)
      handler.should_receive(:process_response_codes).with(context)
      handler.process(context)
    end
  end

  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::ServiceGatewayHandler.new(historian, "job") }
  let(:varz) { {} }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "process_plan_score_metric" do
    let(:varz) do
      {
        "plans" => [
          {
            "plan" => "free",
            "low_water" => 100,
            "high_water" => 1400,
            "score" => 150,
            "max_capacity" => 500,
            "available_capacity" => 450,
            "used_capacity" => 50
          }
        ]
      }
    end

    it "reports plan information" do
      handler.process_plan_score_metric(context)
      historian.should have_sent_data("services.plans.low_water", 100)
      historian.should have_sent_data("services.plans.high_water", 1400)
      historian.should have_sent_data("services.plans.score", 150)
      historian.should have_sent_data("services.plans.allow_over_provisioning", 0)
      historian.should have_sent_data("services.plans.used_capacity", 50)
      historian.should have_sent_data("services.plans.max_capacity", 500)
      historian.should have_sent_data("services.plans.available_capacity", 450)
    end
  end

  describe "response code metrics" do
    let(:varz) do
      {
        "responses_metrics" => {
          "responses_2xx" => 2,
          "responses_3xx" => 3,
          "responses_4xx" => 4,
          "responses_5xx" => 5,
        }
      }
    end

    it "reports response code metrics to the historian" do
      handler.process_response_codes(context)
      historian.should have_sent_data("services.http_status", 2, {service_type: "unknown", component: "gateway", status: "2xx"})
      historian.should have_sent_data("services.http_status", 3, {service_type: "unknown", component: "gateway", status: "3xx"})
      historian.should have_sent_data("services.http_status", 4, {service_type: "unknown", component: "gateway", status: "4xx"})
      historian.should have_sent_data("services.http_status", 5, {service_type: "unknown", component: "gateway", status: "5xx"})
    end
  end

  describe :process_online_nodes do
    let(:varz) do
      {
        "nodes" => {
          "node_0" => {
            "available_capacity" => 50,
            "plan" => "free"
          },
          "node_1" => {
            "available_capacity" => 50,
            "plan" => "free"
          }
        }
      }
    end

    it "should report online nodes number to TSDB server" do
      handler.process_online_nodes(context)
      historian.should have_sent_data("services.online_nodes", 2, { component: "gateway", index: 1, job: "job", service_type: "unknown" })
    end
  end

end
