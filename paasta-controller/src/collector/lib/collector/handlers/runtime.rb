module Collector
  class Handler
    class Runtime < Handler
      def process(context)
        Contexts.new(@historian, job).process(context)
      end
    end
  end
end
