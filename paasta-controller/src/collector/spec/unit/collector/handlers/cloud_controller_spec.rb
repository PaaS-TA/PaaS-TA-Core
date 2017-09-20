require "spec_helper"

describe Collector::Handler::CloudController do
  let(:now) { Time.now }
  let(:data) { {} }
  let(:historian) do
    Object.new.tap do |historian|
      historian.stub(:send_data) do |send_data|
        data[send_data[:key]] = send_data
      end
    end
  end
  let(:context) { Collector::HandlerContext.new(0, now, fixture(:cloud_controller)) }
  let(:handler) { Collector::Handler::CloudController.new(historian, "CloudController") }

  it "should register itself as a handler" do
    Collector::Handler.handler_map["CloudController"].should == handler.class
  end

  describe "#additional_tags" do
    it "tags metrics with the host" do
      handler.additional_tags(context).should == {
        ip: "10.10.16.13",
      }
    end
  end

  describe "process" do
    subject do
      handler.process(context)
      data
    end

    let(:tags) do
      {
        role: "core",
        job: "CloudController",
        index: 0,
        ip: "10.10.16.13",
        name: "CloudController/0",
        deployment: "untitled_dev"
      }
    end

    its(["cc.requests.outstanding"]) { should eq(
      key: "cc.requests.outstanding",
      timestamp: now,
      value: 22,
      tags: tags
    )}

    its(["cc.requests.completed"]) { should eq(
      key: "cc.requests.completed",
      timestamp: now,
      value: 9828,
      tags: tags
    )}

    its(["cc.http_status.1XX"]) { should eq(
      key: "cc.http_status.1XX",
      timestamp: now,
      value: 3,
      tags: tags
    )}

    its(["cc.http_status.2XX"]) { should eq(
      key: "cc.http_status.2XX",
      timestamp: now,
      value: 9105 + 203,
      tags: tags
    )}


    its(["cc.http_status.3XX"]) { should eq(
      key: "cc.http_status.3XX",
      timestamp: now,
      value: 12 + 21,
      tags: tags
    )}

    its(["cc.http_status.4XX"]) { should eq(
      key: "cc.http_status.4XX",
      timestamp: now,
      value: 622 + 99 + 2,
      tags: tags
    )}

    its(["cc.http_status.5XX"]) { should eq(
      key: "cc.http_status.5XX",
      timestamp: now,
      value: 22,
      tags: tags
    )}

    its(["cc.uptime"]) { should eq(
      key: "cc.uptime",
      timestamp: now,
      value: 2 + (60 * 2) + (60 * 60 * 2) + (60 * 60 * 24 * 2),
      tags: tags
    )}

    its(["cc.thread_info.thread_count"]) { should eq(
      key: "cc.thread_info.thread_count",
      timestamp: now,
      value: 21,
      tags: tags
    )}

    its(["cc.thread_info.event_machine.connection_count"]) { should eq(
      key: "cc.thread_info.event_machine.connection_count",
      timestamp: now,
      value: 30,
      tags: tags
    )}

    its(["cc.thread_info.event_machine.threadqueue.size"]) { should eq(
      key: "cc.thread_info.event_machine.threadqueue.size",
      timestamp: now,
      value: 40,
      tags: tags
    )}

    its(["cc.thread_info.event_machine.threadqueue.num_waiting"]) { should eq(
     key: "cc.thread_info.event_machine.threadqueue.num_waiting",
     timestamp: now,
     value: 10,
     tags: tags
    )}

    its(["cc.thread_info.event_machine.resultqueue.size"]) { should eq(
      key: "cc.thread_info.event_machine.resultqueue.size",
      timestamp: now,
      value: 4,
      tags: tags
    )}

    its(["cc.thread_info.event_machine.resultqueue.num_waiting"]) { should eq(
      key: "cc.thread_info.event_machine.resultqueue.num_waiting",
      timestamp: now,
      value: 2,
      tags: tags

    )}
  end

  describe "users" do
    let(:varz) { context.varz }

    it "sends the user count" do
      varz["cc_user_count"] = 4
      handler.process(context)
      expect(data["total_users"][:value]).to eq(4)
    end
  end

  describe "Cloud Controller jobs queue length" do
    let(:varz) { context.varz }
    let(:cc_job_queue_length) { {"cc-local" => 2, "cc-generic" => 1, "total" => 3} }

    it "sends the job queue length" do
      varz["cc_job_queue_length"] = cc_job_queue_length
      handler.process(context)
      expect(data["cc.job_queue_length.cc-local"][:value]).to eq(2)
      expect(data["cc.job_queue_length.cc-generic"][:value]).to eq(1)
      expect(data["cc.job_queue_length.total"][:value]).to eq(3)
    end
  end

  describe "Cloud Controller failed job count" do
    let(:varz) { context.varz }
    let(:cc_failed_job_count) { {"cc-local" => 2, "cc-generic" => 1, "total" => 3} }

    it "sends the failed job count" do
      varz["cc_failed_job_count"] = cc_failed_job_count
      handler.process(context)
      expect(data["cc.failed_job_count.cc-local"][:value]).to eq(2)
      expect(data["cc.failed_job_count.cc-generic"][:value]).to eq(1)
      expect(data["cc.failed_job_count.total"][:value]).to eq(3)
    end
  end
end
