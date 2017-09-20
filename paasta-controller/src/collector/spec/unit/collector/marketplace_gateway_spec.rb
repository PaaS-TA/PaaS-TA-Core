require "spec_helper"

describe Collector::Handler::MarketplaceGateway do
  let(:handler) { described_class.new(nil, nil) }
  let(:context) { Collector::HandlerContext.new(0, Time.now, fixture(:marketplace_gateway)) }

  it 'should have the correct type' do
    handler.service_type.should == 'marketplace'
  end

  it "should provide the correct component_type" do
    handler.component.should == "gateway"
  end

  it "should be the correct base type" do
    handler.should be_kind_of(Collector::ServiceGatewayHandler)
  end

  describe '#additional_tags' do
    it "provides ip address" do
      additional_tags = handler.additional_tags(context)
      expect(additional_tags[:ip]).to eq('10.10.33.0')
    end
  end
end
