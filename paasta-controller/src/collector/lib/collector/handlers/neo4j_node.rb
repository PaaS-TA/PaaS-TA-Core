module Collector
  class Handler
    class Neo4jNode < ServiceNodeHandler
      def service_type
        "neo4j"
      end
    end
  end
end
