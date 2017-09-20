module Collector
  class Handler
    class MysqlNode < ServiceNodeHandler
      def service_type
        "mysql"
      end
    end
  end
end
