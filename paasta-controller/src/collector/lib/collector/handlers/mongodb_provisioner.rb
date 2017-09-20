module Collector
  class Handler
    class MongodbProvisioner < ServiceGatewayHandler
      def service_type
        "mongodb"
      end
    end
  end
end
