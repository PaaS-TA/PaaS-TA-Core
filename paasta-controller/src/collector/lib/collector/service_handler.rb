require_relative "handler"

module Collector
  class ServiceHandler < Handler
    def additional_tags(context)
      { :service_type => service_type, :component => component }
    end

    def service_type    # "mysql", "postgresql", "mongodb" ...
      "unknown"
    end

    def component       # "node", "gateway"
      "unknown"
    end
  end
end
