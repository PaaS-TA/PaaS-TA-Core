require 'spec_helper'

describe Collector::Handler::Etcd do
  let(:historian) { FakeHistorian.new }
  let(:timestamp) { 123456789 }
  let(:handler) { Collector::Handler::Etcd.new(historian, "job") }
  let(:context) { Collector::HandlerContext.new(1, timestamp, varz) }

  describe "handler mappings" do
    it "contains etcd" do
      Collector::Handler.handler_map[Collector::Components::ETCD_COMPONENT].should == handler.class
    end

    it "contains etcd" do
      Collector::Handler.handler_map[Collector::Components::ETCD_DIEGO_COMPONENT].should == handler.class
    end
  end

  describe "etcd" do
    let(:varz) { fixture(:etcd) }

    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("etcd.leader.SomeValueA", 40)
      historian.should have_sent_data("etcd.leader.SomeValueB", 41, foo: "bar")
      historian.should have_sent_data("etcd.server.SomeValueA", 40)
      historian.should have_sent_data("etcd.server.SomeValueB", 41, foo: "bar")
      historian.should have_sent_data("etcd.store.SomeValueA", 40)
      historian.should have_sent_data("etcd.store.SomeValueB", 41, foo: "bar")
    end
  end

  describe "etcd-diego" do
    let(:varz) { fixture(:etcd_diego) }

    it "sends the metrics" do
      handler.process(context)
      historian.should have_sent_data("etcd-diego.leader.SomeValueA", 40)
      historian.should have_sent_data("etcd-diego.leader.SomeValueB", 41, foo: "bar")
      historian.should have_sent_data("etcd-diego.server.SomeValueA", 40)
      historian.should have_sent_data("etcd-diego.server.SomeValueB", 41, foo: "bar")
      historian.should have_sent_data("etcd-diego.store.SomeValueA", 40)
      historian.should have_sent_data("etcd-diego.store.SomeValueB", 41, foo: "bar")
    end
  end
end
