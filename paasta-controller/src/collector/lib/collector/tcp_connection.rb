module Collector
  # TCP connection for sending metrics
  class TCPConnection < EventMachine::Connection
    def initialize(logger_name)
      @logger_name = logger_name
    end

    def connection_completed
      Config.logger.info("#{@logger_name}-connected")
      @port, @ip = Socket.unpack_sockaddr_in(get_peername)
    end

    def unbind
      if @port && @ip
        Config.logger.warn("#{@logger_name}-connection-lost")
        EM.add_timer(1.0) do
          begin
            reconnect(@ip, @port)
          rescue EventMachine::ConnectionError => e
            Config.logger.warn("#{@logger_name}-connection-error", error: e, backtrace: e.backtrace)
            unbind
          end
        end
      else
        Config.logger.fatal("#{@logger_name}.could-not-connect")
        exit!
      end
    end

    def receive_data(_)
    end
  end
end
