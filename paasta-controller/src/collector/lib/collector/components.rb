module Collector
  module Components
    CLOUD_CONTROLLER_COMPONENT = "CloudController".freeze
    DEA_COMPONENT = "DEA".freeze
    HEALTH_MANAGER_COMPONENT = "HealthManager".freeze
    HM9000_COMPONENT = "HM9000".freeze
    ROUTER_COMPONENT = "Router".freeze
    DOPPLER_SERVER_COMPONENT = "DopplerServer".freeze
    LOGGREGATOR_TRAFFICCONTROLLER_COMPONENT = "LoggregatorTrafficcontroller".freeze
    LOGGREGATOR_DEA_AGENT_COMPONENT = "LoggregatorDeaAgent".freeze
    METRON_AGENT_COMPONENT = "MetronAgent".freeze

    MARKETPLACE_GATEWAY = 'MarketplaceGateway'

    ETCD_COMPONENT = "etcd".freeze
    ETCD_DIEGO_COMPONENT = "etcd-diego".freeze

    # diego runtime state
    RUNTIME_COMPONENT = "runtime".freeze

    # services components
    MYSQL_PROVISIONER = "MyaaS-Provisioner".freeze
    MYSQL_NODE = "MyaaS-Node".freeze

    PGSQL_PROVISIONER = "AuaaS-Provisioner".freeze
    PGSQL_NODE = "AuaaS-Node".freeze

    MONGODB_PROVISIONER = "MongoaaS-Provisioner".freeze
    MONGODB_NODE = "MongoaaS-Node".freeze

    NEO4J_PROVISIONER = "Neo4jaaS-Provisioner".freeze
    NEO4J_NODE = "Neo4jaaS-Node".freeze

    RABBITMQ_PROVISIONER = "RMQaaS-Provisioner".freeze
    RABBITMQ_NODE = "RMQaaS-Node".freeze

    REDIS_PROVISIONER = "RaaS-Provisioner".freeze
    REDIS_NODE = "RaaS-Node".freeze

    VBLOB_PROVISIONER = "VBlobaaS-Provisioner".freeze
    VBLOB_NODE = "VBlobaaS-Node".freeze

    SERIALIZATION_DATA_SERVER = "SerializationDataServer".freeze

    BACKUP_MANAGER = "BackupManager".freeze

    CORE_COMPONENTS = Set.new([CLOUD_CONTROLLER_COMPONENT, DEA_COMPONENT,
      HEALTH_MANAGER_COMPONENT, HM9000_COMPONENT, ROUTER_COMPONENT,
      DOPPLER_SERVER_COMPONENT, LOGGREGATOR_TRAFFICCONTROLLER_COMPONENT, LOGGREGATOR_DEA_AGENT_COMPONENT]).freeze
    SERVICE_COMPONENTS = Set.new([MYSQL_PROVISIONER, MYSQL_NODE,
      PGSQL_PROVISIONER, PGSQL_NODE,
      MONGODB_PROVISIONER, MONGODB_NODE,
      NEO4J_PROVISIONER, NEO4J_NODE,
      RABBITMQ_PROVISIONER, RABBITMQ_NODE,
      REDIS_PROVISIONER, REDIS_NODE,
      VBLOB_PROVISIONER, VBLOB_NODE]).freeze
    SERVICE_AUXILIARY_COMPONENTS = Set.new([SERIALIZATION_DATA_SERVER,
      BACKUP_MANAGER]).freeze

    HANDLERS = {
      CLOUD_CONTROLLER_COMPONENT => Collector::Handler::CloudController,
      DEA_COMPONENT => Collector::Handler::Dea,
      HEALTH_MANAGER_COMPONENT => Collector::Handler::HealthManager,
      HM9000_COMPONENT => Collector::Handler::HM9000,
      ROUTER_COMPONENT => Collector::Handler::Router,
      DOPPLER_SERVER_COMPONENT => Collector::Handler::DopplerServer,
      LOGGREGATOR_TRAFFICCONTROLLER_COMPONENT => Collector::Handler::LoggregatorRouter,
      LOGGREGATOR_DEA_AGENT_COMPONENT => Collector::Handler::LoggregatorDeaAgent,
      METRON_AGENT_COMPONENT => Collector::Handler::MetronAgent,
      MARKETPLACE_GATEWAY => Collector::Handler::MarketplaceGateway,
      ETCD_COMPONENT => Collector::Handler::Etcd,
      ETCD_DIEGO_COMPONENT => Collector::Handler::Etcd,
      RUNTIME_COMPONENT => Collector::Handler::Runtime,
      MYSQL_PROVISIONER => Collector::Handler::MysqlProvisioner,
      MYSQL_NODE => Collector::Handler::MysqlNode,
      PGSQL_PROVISIONER => Collector::Handler::PostgresqlProvisioner,
      PGSQL_NODE => Collector::Handler::PostgresqlNode,
      MONGODB_PROVISIONER => Collector::Handler::MongodbProvisioner,
      MONGODB_NODE => Collector::Handler::MongodbNode,
      NEO4J_PROVISIONER => Collector::Handler::Neo4jProvisioner,
      NEO4J_NODE => Collector::Handler::Neo4jNode,
      RABBITMQ_PROVISIONER => Collector::Handler::RabbitmqProvisioner,
      RABBITMQ_NODE => Collector::Handler::RabbitmqNode,
      REDIS_PROVISIONER => Collector::Handler::RedisProvisioner,
      REDIS_NODE => Collector::Handler::RedisNode,
      VBLOB_PROVISIONER => Collector::Handler::VblobProvisioner,
      VBLOB_NODE => Collector::Handler::VblobNode,
      SERIALIZATION_DATA_SERVER => Collector::Handler::SerializationDataServer,
      BACKUP_MANAGER => Collector::Handler::BackupManager
    }.freeze

    # Generates the common tags used for generating common
    # (memory, health, etc.) metrics.
    #
    # @param [String] type the job type
    # @return [Hash<Symbol, String>] tags for this job type
    def self.get_job_tags(type)
      tags = {}
      if CORE_COMPONENTS.include?(type)
        tags[:role] = "core"
      elsif SERVICE_COMPONENTS.include?(type)
        tags[:role] = "service"
      elsif SERVICE_AUXILIARY_COMPONENTS.include?(type)
        tags[:role] = "service"
      end
      tags
    end
  end
end
