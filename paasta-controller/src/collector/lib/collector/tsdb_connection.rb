require_relative "tcp_connection"

module Collector
  # TSDB connection for sending metrics
  class TsdbConnection < Collector::TCPConnection
    def initialize
      super("collector.tsdb")
    end
  end
end
