require File.expand_path("../spec_helper", File.dirname(__FILE__))

describe Collector::Components do
  describe :get_job_tags do
    it "should mark the core components" do
      ["CloudController", "DEA", "HealthManager", "Router"].each do |job|
        Collector::Components.get_job_tags(job).should == {:role=>"core"}
      end
    end

    it "should mark the service node components" do
      ["MongoaaS-Node", "RMQaaS-Node"].each do |job|
        Collector::Components.get_job_tags(job).should == {:role => "service"}
      end
    end

    it "should mark the service provisioner components" do
      ["MongoaaS-Provisioner", "RMQaaS-Provisioner"].each do |job|
        Collector::Components.get_job_tags(job).should == {:role => "service"}
      end
    end
  end

end
