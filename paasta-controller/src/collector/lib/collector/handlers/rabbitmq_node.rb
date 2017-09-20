module Collector
  class Handler
    class RabbitmqNode < ServiceNodeHandler
      def service_type
        "rabbitmq"
      end
    end
  end
end
