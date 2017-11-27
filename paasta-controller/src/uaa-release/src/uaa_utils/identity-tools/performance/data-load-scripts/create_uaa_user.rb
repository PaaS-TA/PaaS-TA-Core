#Usage - . ./create_uaa_users.sh <Start index of user> <End index of user> <zone_subdomain> <client_id>

if ARGV.length < 4
  raise 'Please provide the number of users to be created.'
end

start_user_index, end_user_index, zone_subdomain, client_id = ARGV

(start_user_index..end_user_index).each do |user_index|
  host = "http://#{zone_subdomain}.localhost:8080/uaa"
  username = "zoneuser#{user_index}"

  puts 'Target UAAC to the perf UAA app'
  `uaac target #{host}`
  puts host
  puts client_id
  puts 'Get a token from the admin client to create client, groups and users'
  `uaac token client get #{client_id} -s clientsecret`
  puts 'Update client scope and grant_types'
  puts "Add user #{user_index}"
  `uaac user add #{username} --given_name PerformanceUser#{user_index}FN --family_name PerformanceUser#{user_index}LN --emails #{username}@testcf.com -p password`

  ['acs.attributes.read', 'acs.attributes.write', 'acs.policies.write', 'acs.policies.read'].each do |group|
    `uaac group add #{group}`
    `uaac member add #{group} #{username}`
  end

  `rm ~/.uaac.yml`
end

