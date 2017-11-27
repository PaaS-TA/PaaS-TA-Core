#!/usr/bin/env ruby

require 'yaml'

cf_manifest = YAML.load_file ARGV[0]

cf_manifest['resource_pools'] << {
  "name" => "ha_proxy_z1",
  "cloud_properties" => {"name" => "random", "ports" => [{"host" => 80},{"host" => 443},{"host" => 2222},{"host" => 4443}]},
  "stemcell" => cf_manifest["resource_pools"][0]["stemcell"],
  "network" => cf_manifest["resource_pools"][0]["network"],
  "env" => cf_manifest["resource_pools"][0]["env"],
}
cf_manifest['jobs'].find {|job| job['name'] == 'ha_proxy_z1' }['resource_pool'] = "ha_proxy_z1"

File.open(ARGV[1], 'w+') {|f| f.write YAML.dump(cf_manifest) }
