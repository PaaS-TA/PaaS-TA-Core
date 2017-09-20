module Collector
  class Handler
    class MysqlProvisioner < ServiceGatewayHandler
      def service_type
        "mysql"
      end
    end
  end
end
