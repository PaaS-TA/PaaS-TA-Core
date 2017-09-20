module Collector
  class Handler
    class PostgresqlProvisioner < ServiceGatewayHandler
      def service_type
        "postgresql"
      end
    end
  end
end
