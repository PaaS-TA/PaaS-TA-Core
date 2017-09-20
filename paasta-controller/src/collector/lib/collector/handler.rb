require "vcap/common"

module Collector

  class HandlerContext
    attr_reader :index
    attr_reader :now
    attr_reader :varz

    def initialize(index, now, varz)
      @index = index
      @now = now
      @varz = varz
    end

    def ==(other)
      other.index == index && other.now == now && other.varz == varz
    end
  end

  # Varz metric handler
  #
  # It's used for processing varz from jobs and publishing them to the metric collector (Historian)
  # server
  class Handler
    @instance_map = {}

    class << self
      # @return [Hash<String, Handler>] hash of jobs to {Handler}s
      attr_accessor :handler_map
      attr_accessor :instance_map

      def handler_map
        Components::HANDLERS
      end

      # Retrieves a {Handler} for the job type with the provided context. Will
      # default to the generic one if the job does not have a handler
      # registered.
      #
      # @param [Collector::Historian] historian the historian to use for
      #   writing metrics
      # @param [String] job the job name
      # @param [Fixnum] index the job index
      # @param [Fixnum] now the timestamp of when the metrics were collected
      # @param [Hash] varz the values from the remote server /varz
      # @return [Handler] the handler for this job from the handler map or the
      #   default one
      def handler(historian, job)
        handler_class = Handler.handler_map.fetch(job, Handler)
        handler_instance = @instance_map[handler_class]
        unless handler_instance
          handler_instance = handler_class.new(historian, job)
          @instance_map[handler_class] = handler_instance
        end
        handler_instance
      end
    end

    # @return [String] job name
    attr_reader :job

    # Creates a new varz handler
    #
    # @param [Collector::Historian] historian
    # @param [String] job the job for this varz
    # @param [Fixnum] index the index for this varz
    # @param [Fixnum] now the timestamp when it was collected
    def initialize(historian, job)
      @historian = historian
      @job = job
    end

    # Processes varz in the context of the collection. Subclasses
    # should override this.
    #
    # @param [Hash] varz the varzs collected for this job
    def process(context)
    end

    # Subclasses can override this to add additional tags to the metrics
    # submitted.
    #
    # @param [Hash] varz the varzs collected for this job
    # @return [Hash] the key/value tags that will be added to the submission
    def additional_tags(context)
      {}
    end

    MEM_AND_CPU_STATS = %w(mem_bytes mem_used_bytes mem_free_bytes cpu_load_avg).freeze

    RECORDED_LOG_LEVELS = %w(fatal error warn).freeze

    # Called by the collector to process the varz. Processes common
    # metric data and then calls process() to add subclass behavior.
    def do_process(context)
      varz = context.varz

      MEM_AND_CPU_STATS.each { |stat| send_metric(stat, varz[stat], context) if varz[stat] }

      send_metric("uptime_in_seconds", VCAP.uptime_string_to_seconds(varz["uptime"]), context) if varz["uptime"]

      # Log counts in varz look like: { log_counts: { "error": 2, "warn": 1 }}
      varz.fetch("log_counts", {}).each do |level, count|
        next unless RECORDED_LOG_LEVELS.include?(level)
        send_metric("log_count", count, context, {"level" => level})
      end

      process(context)
    end

    # Sends the metric to the metric collector (historian)
    #
    # @param [String] name the metric name
    # @param [String, Fixnum] value the metric value
    def send_metric(name, value, context, tags_provided = {})
      if value.nil?
        Config.logger.warn("Received no value for #{name}")
        return
      end

      tags_provided = symbolize_keys(tags_provided)
      tags = base_tags(context).merge(tags_provided)
      job = tags[:job]
      index = tags[:index]
      tags[:name] = "#{job}/#{index}" unless tags[:name]

      @historian.send_data({key: name,
                               timestamp: context.now,
                               value: value,
                               tags: tags})
    end

    # Sends latency metrics to the metric collector (historian)
    #
    # @param [String] name the metric name
    # @param [Hash] value the latency metric value
    def send_latency_metric(name, value, context, tags = {})
      return unless value

      samples = value[:samples] || value["samples"]
      if samples > 0
        value = value[:value] || value["value"]
        send_metric(name, value / samples, context, tags)
      end
    end

    private

    def symbolize_keys(hash)
      hash.inject({}){|memo,(k,v)| memo[k.to_sym] = v; memo}
    end

    def base_tags(context)
      base_tags = additional_tags(context)
      base_tags.merge!(Components.get_job_tags(@job))
      base_tags.merge!(job: @job, index: context.index, deployment: Config.deployment_name)
    end
  end
end
