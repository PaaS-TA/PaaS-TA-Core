module Collector
  class Handler
    class Neo4jProvisioner < ServiceGatewayHandler
      def service_type
        "neo4j"
      end
    end
  end
end
