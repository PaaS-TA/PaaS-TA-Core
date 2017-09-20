module Collector
  class Handler
    class RedisNode < ServiceNodeHandler
      def service_type
        "redis"
      end
    end
  end
end
