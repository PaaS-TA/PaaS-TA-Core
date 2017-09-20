#!/usr/bin/env ruby

require 'nats/client'
require 'json'
# require 'rspec'

TOTAL_MESSAGES = 10

config = JSON.parse(File.read(ARGV[0]))
nats_hosts = config.fetch('Hosts')
nats_user = config.fetch('User')
nats_password = config.fetch('Password')
nats_port = config.fetch('Port')

nats_uris = nats_hosts.map do |host|
  "nats://#{nats_user}:#{nats_password}@#{host}:#{nats_port}"
end

num_received = 0
timed_out = false

NATS.start(servers: nats_uris) do
  sid = NATS.subscribe("test") do |message|
    num_received += 1
    NATS.stop if num_received == TOTAL_MESSAGES
  end

  NATS.timeout(sid, 10) do
    timed_out = true
    NATS.stop
  end

  puts "Publishing hello world to test #{TOTAL_MESSAGES} times"
  TOTAL_MESSAGES.times { NATS.publish("test", "hello world") }
end

if timed_out
  raise "Only received #{num_received}. Expected #{TOTAL_MESSAGES}"
end

