require 'collector/tsdb_connection'

module Collector
  class Historian
    class Tsdb
      attr_reader :connection
      def initialize(host, port)
        @host = host
        @port = port
        @connection = EventMachine.connect(@host, @port, TsdbConnection)
      end

      def send_data(properties)
        tags = (properties[:tags].flat_map do |key, value|
          Array(value).map do |v|
            "#{key}=#{v}"
          end
        end).sort.join(" ")

        command = "put #{properties[:key]} #{properties[:timestamp]} #{properties[:value]} #{tags}\n"
        @connection.send_data(command)
      end
    end
  end
end