module Collector
  class Handler
    class SerializationDataServer < ServiceHandler
      def process
        if varz["nfs_free_space"]
          send_metric("services.nfs_free_space", varz["nfs_free_space"])
        end
      end

      def service_type
        "serialization_data_server"
      end
    end
  end
end
