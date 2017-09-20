module Collector
  class Handler
    class RedisProvisioner < ServiceGatewayHandler
      def service_type
        "redis"
      end
    end
  end
end
