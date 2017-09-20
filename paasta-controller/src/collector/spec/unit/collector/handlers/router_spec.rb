require 'spec_helper'

describe Collector::Handler::Router do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::Router.new(historian, "job") }
  let(:handler_context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "#additional_tags" do
    let(:varz) { { "host" => "0.0.0.11:4567" } }

    it "tags metrics with the host" do
      handler.additional_tags(handler_context).should == {
        ip: "0.0.0.11",
      }
    end
  end

  describe "#process" do
    let(:varz) do
      {
        "host" => "1.2.3.4:5678",
        "latency" => {
          "50" => 0.0250225,
          "75" => 0.03684,
          "90" => 0.09650980000000002,
          "95" => 0.3351683999999978,
          "99" => 1.314998740000001,
          "samples" => 1,
          "value" => 5.0e-07
        },
        "rate" => [
          0.22460493344950977,
          0.49548432897218125,
          0.9014480037952
        ],
        "requests" => 68213,
        "bad_requests" => 42,
        "bad_gateways" => 45387,
        "requests_per_sec" => 0.22460493344950977,
        "responses_2xx" => 65021,
        "responses_3xx" => 971,
        "responses_4xx" => 2182,
        "responses_5xx" => 1,
        "responses_xxx" => 38,
        "ms_since_last_registry_update" => 15,
        "start" => "2013-05-28 22:01:19 +0000",
        "tags" => {
          "component" => {
          }
        },
        "urls" => 123456789
      }
    end

    context "for default components" do
      let(:component) do
        {
          "latency" => {
            "50" => 0.025036,
            "75" => 0.034314,
            "90" => 0.0791451,
            "95" => 0.1607238499999999,
            "99" => 1.1623077700000013,
            "samples" => 1,
            "value" => 5
          },
          "rate" => [
            0.22490272672626982,
            0.4771015543892108,
            0.8284101734116986
          ],
          "requests" => 3200,
          "responses_2xx" => 100,
          "responses_3xx" => 200,
          "responses_4xx" => 400,
          "responses_5xx" => 800,
          "responses_xxx" => 1600
        }
      end

      before do
        varz['tags']['component']['component-1'] = component
      end

      it "sends the default metrics" do
        tags = {component: "component-1"}

        handler.process(handler_context)

        historian.should have_sent_data("router.total_requests", 68213)
        historian.should have_sent_data("router.total_routes", 123456789)
        historian.should have_sent_data("router.requests_per_sec", (0.22460493344950977).to_i)
        historian.should have_sent_data("router.ms_since_last_registry_update", 15)

        historian.should have_sent_data("router.rejected_requests", 42)
        historian.should have_sent_data("router.bad_gateways", 45387)

        historian.should have_sent_data("router.requests", 3200, tags)
        historian.should have_sent_data("router.latency.1m", 5, tags)

        historian.should have_sent_data("router.responses", 100, tags.merge(status: "2xx"))
        historian.should have_sent_data("router.responses", 200, tags.merge(status: "3xx"))
        historian.should have_sent_data("router.responses", 400, tags.merge(status: "4xx"))
        historian.should have_sent_data("router.responses", 800, tags.merge(status: "5xx"))
        historian.should have_sent_data("router.responses", 1600, tags.merge(status: "xxx"))
      end
    end

    context "for dea-related components (i.e., apps)" do
      let(:dea_1) do
        {
            "latency" => {
                "50" => 0.025036,
                "75" => 0.034314,
                "90" => 0.0791451,
                "95" => 0.1607238499999999,
                "99" => 1.1623077700000013,
                "samples" => 1,
                "value" => 5
            },
            "rate" => [
                0.22490272672626982,
                0.4771015543892108,
                0.8284101734116986
            ],
            "requests" => 2400,
            "responses_2xx" => 200,
            "responses_3xx" => 300,
            "responses_4xx" => 400,
            "responses_5xx" => 400,
            "responses_xxx" => 1000
        }
      end

      let(:dea_2) do
        {
          "latency" => {
            "50" => 0.025036,
            "75" => 0.034314,
            "90" => 0.0791451,
            "95" => 0.1607238499999999,
            "99" => 1.1623077700000013,
            "samples" => 1,
            "value" => 5
          },
          "rate" => [
            0.22490272672626982,
            0.4771015543892108,
            0.8284101734116986
          ],
          "requests" => 1500,
          "responses_2xx" => 200,
          "responses_3xx" => 300,
          "responses_4xx" => 400,
          "responses_5xx" => 400,
          "responses_xxx" => 100
        }
      end

      it "sends metrics tagged with component:dea and dea_index:x" do
        varz['tags']['component']['dea-1'] = dea_1

        tags = {:component => "app", :dea_index => "1"}

        handler.process(handler_context)

        historian.should have_sent_data("router.requests", 2400, tags)
        historian.should have_sent_data("router.latency.1m", 5, tags)
      end

      it "sends a metric of all dea requests" do
        varz['tags']['component']['dea-1'] = dea_1
        varz['tags']['component']['dea-2'] = dea_2

        handler.process(handler_context)

        historian.should have_sent_data("router.routed_app_requests", 3900)
      end
    end
  end
end
