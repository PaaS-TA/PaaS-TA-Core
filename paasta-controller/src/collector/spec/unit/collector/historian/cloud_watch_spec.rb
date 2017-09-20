require File.expand_path("../../../spec_helper", File.dirname(__FILE__))

describe Collector::Historian::CloudWatch do
  describe "initialization" do
    it "configures AWS credentials" do
      Aws::Credentials.should_receive(:new).with("ACCESS", "SECRET")
      Aws.config.should_receive(:update).with({region: 'us-east-1',
                                       credentials: anything(),
                                       })
      described_class.new("ACCESS", "SECRET")
    end
  end

  describe "sending data to CloudWatch" do
    let(:cloud_watch) { double('CloudWatch') }

    before do
      Aws::CloudWatch::Client.should_receive(:new).and_return(cloud_watch)
      ::Collector::Config.stub(:deployment_name).and_return("dev114cw")
    end

    it "converts the properties hash into a cloud watch command" do
      cloud_watch_historian = described_class.new("ACCESS", "SECRET")

      cloud_watch.should_receive(:put_metric_data).with({
                                                            namespace: "CF/Collector",
                                                            metric_data: [
                                                                {
                                                                    metric_name: "some_metric.some_key",
                                                                    value: "2",
                                                                    timestamp: "2013-03-07T19:13:28Z",
                                                                    dimensions: [
                                                                        {name: "job", value: "Test"},
                                                                        {name: "index", value: "1"},
                                                                        {name: "component", value: "unknown"},
                                                                        {name: "service_type", value: "unknown"},
                                                                        {name: "tag", value: "value"},
                                                                        {name: "foo", value: "bar"},
                                                                        {name: "foo", value: "baz"},
                                                                    ]
                                                                }]
                                                        })

      cloud_watch_historian.send_data({
                                   key: "some_metric.some_key",
                                   timestamp: 1362683608,
                                   value: 2,
                                   tags: {
                                       job: "Test",
                                       index: 1,
                                       component: "unknown",
                                       service_type: "unknown",
                                       tag: "value",
                                       foo: %w(bar baz)
                                   }
                               })
    end

    it "converts different structure of tags" do
      cloud_watch_historian = described_class.new("ACCESS", "SECRET")

      cloud_watch.should_receive(:put_metric_data).with({
                                                            namespace: "CF/Collector",
                                                            metric_data: [
                                                                {
                                                                    metric_name: "some_key",
                                                                    value: "2",
                                                                    timestamp: "2013-03-07T19:13:28Z",
                                                                    dimensions: [
                                                                        {name: "job", value: "Test"},
                                                                        {name: "index", value: "1"},
                                                                        {name: "plan", value: "free"},
                                                                        {name: "service_type", value: "unknown"},
                                                                        {name: "tag", value: "value"},
                                                                        {name: "foo", value: "bar"},
                                                                        {name: "foo", value: "baz"},
                                                                    ]
                                                                }]
                                                        })

      cloud_watch_historian.send_data({
                                          key: "some_key",
                                          timestamp: 1362683608,
                                          value: 2,
                                          tags: {
                                              job: "Test",
                                              index: 1,
                                              plan: "free",
                                              service_type: "unknown",
                                              tag: "value",
                                              foo: %w(bar baz)
                                          }
                                      })
    end
  end
end