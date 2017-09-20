module Collector
  class Handler
    class MongodbNode < ServiceNodeHandler
      def service_type
        "mongodb"
      end
    end
  end
end
