module Collector
  class Handler
    class HM9000 < Handler
      def process(context)
        Golang.new(@historian, job).process(context)
        Contexts.new(@historian, job).process(context)
      end
    end
  end
end
