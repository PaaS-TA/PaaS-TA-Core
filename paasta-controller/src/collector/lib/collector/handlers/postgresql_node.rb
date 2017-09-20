module Collector
  class Handler
    class PostgresqlNode < ServiceNodeHandler
      def service_type
        "postgresql"
      end
    end
  end
end
