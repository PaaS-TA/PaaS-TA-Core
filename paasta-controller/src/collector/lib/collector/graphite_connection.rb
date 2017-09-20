require_relative "tcp_connection"

module Collector
  # Graphite connection for sending metrics
  class GraphiteConnection < Collector::TCPConnection
    def initialize
      super("collector.graphite")
    end
  end
end
