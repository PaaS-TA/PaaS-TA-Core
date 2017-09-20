# Encoding: utf-8
# Cloud Foundry Java Buildpack
# Copyright 2013-2016 the original author or authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'fileutils'
require 'java_buildpack/component/versioned_dependency_component'
require 'java_buildpack/container'
require 'java_buildpack/util/java_main_utils'

module JavaBuildpack
  module Container

    # Encapsulates the detect, compile, and release functionality for applications running Spring Boot CLI
    # applications.
    class Jboss < JavaBuildpack::Component::VersionedDependencyComponent

      # (see JavaBuildpack::Component::BaseComponent#compile)
      def compile
        download_tar
        update_configuration
        copy_application
        create_dodeploy
      end

      # (see JavaBuildpack::Component::BaseComponent#release)
      def release
        @droplet.java_opts.add_system_property 'http.port', '$PORT'

        [
          @droplet.java_home.as_env_var,
          @droplet.java_opts.as_env_var,
          "$PWD/#{(@droplet.sandbox + 'bin/standalone.sh').relative_path_from(@droplet.root)}",
          '-b',
          '0.0.0.0'
        ].compact.join(' ')
      end

      protected

      # (see JavaBuildpack::Component::VersionedDependencyComponent#supports?)
      def supports?
        web_inf? && !JavaBuildpack::Util::JavaMainUtils.main_class(@application)
      end

      private

      def copy_application
        FileUtils.mkdir_p root
        @application.root.children.each { |child| FileUtils.cp_r child, root }
      end

      def create_dodeploy
        FileUtils.touch(webapps + 'ROOT.war.dodeploy')
      end

      def root
        webapps + 'ROOT.war'
      end

      def update_configuration
        standalone_config = @droplet.sandbox + 'standalone/configuration/standalone.xml'

        modified = standalone_config.read
                     .gsub(/<virtual-server name="default-host" enable-welcome-root="true">/,
                           '<virtual-server name="default-host" enable-welcome-root="false">')
                     .gsub(/<socket-binding name="http" port="8080"\/>/,
                           '<socket-binding name="http" port="${http.port}"/>')

        standalone_config.open('w') { |f| f.write modified }
      end

      def webapps
        @droplet.sandbox + 'standalone/deployments'
      end

      def web_inf?
        (@application.root + 'WEB-INF').exist?
      end

    end

  end
end
