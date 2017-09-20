module Collector
  class Handler
    class VblobProvisioner < ServiceGatewayHandler
      def service_type
        "vblob"
      end
    end
  end
end
