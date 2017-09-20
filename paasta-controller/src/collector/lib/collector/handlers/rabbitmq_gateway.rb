module Collector
  class Handler
    class RabbitmqProvisioner < ServiceGatewayHandler
      def service_type
        "rabbitmq"
      end
    end
  end
end
