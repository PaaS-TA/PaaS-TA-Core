package helpers_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"

	"code.cloudfoundry.org/dockerapplifecycle"
	"code.cloudfoundry.org/dockerapplifecycle/docker/nat"
	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"github.com/docker/distribution/registry/client/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

type testCreds struct {
	user     string
	password string
}

func (t testCreds) Basic(url *url.URL) (string, string) {
	return t.user, t.password
}

var _ = Describe("Builder helpers", func() {
	var response = `{
   "schemaVersion": 1,
   "name": "cloudfoundry/diego-docker-app",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:b1238eb35a82386918217263b74d44d17ed450a1ed3db50b430d523c3a2342b7"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:39702b58482c695535d7d86333f222733ecea49d80132707d35d73b02878e608"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:1db09adb5ddd7f1a07b6d585a7db747a51c7bd17418d47e91f901bdf420abd66"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"7ea1ccc3c3ebe0cd68f07be3dca8ab3a74c8652de6892b87fde66805f9e7b496\",\"parent\":\"f32290ad356da6e1256f228e53b21af11b9ec1b9e81a35920361d11304170891\",\"created\":\"2015-08-21T00:19:28.139525649Z\",\"container\":\"8fd2f1227d1ce59825ff630ea82b26a7244144130cb8ca1b0bc93eca9ce743ed\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"./dockerapp\\\"]\"],\"Image\":\"f32290ad356da6e1256f228e53b21af11b9ec1b9e81a35920361d11304170891\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"./dockerapp\"],\"Image\":\"f32290ad356da6e1256f228e53b21af11b9ec1b9e81a35920361d11304170891\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"f32290ad356da6e1256f228e53b21af11b9ec1b9e81a35920361d11304170891\",\"parent\":\"8a9960d2adab8b1219da489720e214abe42a7aac59c1eaa870773db37a768fa1\",\"created\":\"2015-08-21T00:19:27.969274026Z\",\"container\":\"097bcffd1c4b366a02759f546e376a0b1b9918d291dae26a9f9c9ccf59a0c24c\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"adduser -D vcap\"],\"Image\":\"8a9960d2adab8b1219da489720e214abe42a7aac59c1eaa870773db37a768fa1\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"8a9960d2adab8b1219da489720e214abe42a7aac59c1eaa870773db37a768fa1\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":2673}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"8a9960d2adab8b1219da489720e214abe42a7aac59c1eaa870773db37a768fa1\",\"parent\":\"df587f288b3b25f5632876be520d5e3e4b9464eb669ba27483b361b0eb061966\",\"created\":\"2015-08-21T00:18:11.330238508Z\",\"container\":\"13ea240036a56abb25d5f4b1c89fe876ee5c67e29067dacf9d53d4b400fa9a7b\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) WORKDIR /myapp\"],\"Image\":\"df587f288b3b25f5632876be520d5e3e4b9464eb669ba27483b361b0eb061966\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"df587f288b3b25f5632876be520d5e3e4b9464eb669ba27483b361b0eb061966\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"df587f288b3b25f5632876be520d5e3e4b9464eb669ba27483b361b0eb061966\",\"parent\":\"db04f28a5c7020233e53fd499c13ee0825332dbe1dea460343e0d321b7b24e1e\",\"created\":\"2015-08-21T00:18:11.137585237Z\",\"container\":\"9353b2fd746a53057aef105ecc461b92a975720a2a55c8a339754122f3ba3e62\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:2e7c934245de1a54e14719c166d7e9da1ea7ffbf4a7d4ab582aadd846d8a8410 in /myapp/dockerapp\"],\"Image\":\"db04f28a5c7020233e53fd499c13ee0825332dbe1dea460343e0d321b7b24e1e\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"db04f28a5c7020233e53fd499c13ee0825332dbe1dea460343e0d321b7b24e1e\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":6205408}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"db04f28a5c7020233e53fd499c13ee0825332dbe1dea460343e0d321b7b24e1e\",\"parent\":\"f5637bee1ed6d24624af87171b43b610317ce279982cd8ba4c6d9b20ba13f883\",\"created\":\"2015-07-27T21:25:00.353200497Z\",\"container\":\"22f3e2437ac329aa76eea96799cb3c8c2e31c46b9a47e529f544d055f7d280b9\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) EXPOSE 8080/tcp\"],\"Image\":\"f5637bee1ed6d24624af87171b43b610317ce279982cd8ba4c6d9b20ba13f883\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"f5637bee1ed6d24624af87171b43b610317ce279982cd8ba4c6d9b20ba13f883\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"f5637bee1ed6d24624af87171b43b610317ce279982cd8ba4c6d9b20ba13f883\",\"parent\":\"11376daad7c309fcb61b8e0f60fc225d3fb4c137fd7d5bd9ffb96ffc02984148\",\"created\":\"2015-07-27T21:25:00.153173623Z\",\"container\":\"dfdad968c9c53bee5e1cbb7cc2be990eac60764fcd83f558629ee4840b42e044\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV SOME_VAR=some_docker_value\"],\"Image\":\"11376daad7c309fcb61b8e0f60fc225d3fb4c137fd7d5bd9ffb96ffc02984148\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"11376daad7c309fcb61b8e0f60fc225d3fb4c137fd7d5bd9ffb96ffc02984148\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"11376daad7c309fcb61b8e0f60fc225d3fb4c137fd7d5bd9ffb96ffc02984148\",\"parent\":\"c116d10b2719cf284232e648a3dedfa00f486aa36b33f103e3451d38b50750f8\",\"created\":\"2015-07-27T21:24:59.986302656Z\",\"container\":\"6f87a4c58f08d13f289bef33d15ce5136fd4f01ac56c7bf038b589d849216f58\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV HOME=/home/some_docker_user\"],\"Image\":\"c116d10b2719cf284232e648a3dedfa00f486aa36b33f103e3451d38b50750f8\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"c116d10b2719cf284232e648a3dedfa00f486aa36b33f103e3451d38b50750f8\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"c116d10b2719cf284232e648a3dedfa00f486aa36b33f103e3451d38b50750f8\",\"parent\":\"4333cbf9a5602e88bd4ac6a97cc64e980dbdd9e85cb812026748629d55b5c986\",\"created\":\"2015-07-27T21:24:59.782883391Z\",\"container\":\"2f1367603f39af46c93081b78be2e2f6060ace6f1f8d9bf4a0600ed82007621c\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV BAD_SHELL=$1\"],\"Image\":\"4333cbf9a5602e88bd4ac6a97cc64e980dbdd9e85cb812026748629d55b5c986\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"4333cbf9a5602e88bd4ac6a97cc64e980dbdd9e85cb812026748629d55b5c986\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"4333cbf9a5602e88bd4ac6a97cc64e980dbdd9e85cb812026748629d55b5c986\",\"parent\":\"0932715998f2ffc36135e1f051fa748eee842551295ef7fc218ea81fffd8c675\",\"created\":\"2015-07-27T21:24:59.572959896Z\",\"container\":\"2b29a2899dd5e823fef74762fcaa4408e781b396d9a1afde10d9a5fa5d055a8d\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV BAD_QUOTE='\"],\"Image\":\"0932715998f2ffc36135e1f051fa748eee842551295ef7fc218ea81fffd8c675\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"0932715998f2ffc36135e1f051fa748eee842551295ef7fc218ea81fffd8c675\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"0932715998f2ffc36135e1f051fa748eee842551295ef7fc218ea81fffd8c675\",\"parent\":\"f299f81f08116b725aaa299e3ca89a81107a47bd34be43d3f1f62bb0a9909734\",\"created\":\"2015-07-27T21:24:59.363711955Z\",\"container\":\"f9c0c574a41e833dba420b89cc23c2dfc8842af765480c02c3c58ced516278bb\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV VCAP_APPLICATION={}\"],\"Image\":\"f299f81f08116b725aaa299e3ca89a81107a47bd34be43d3f1f62bb0a9909734\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"VCAP_APPLICATION={}\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"f299f81f08116b725aaa299e3ca89a81107a47bd34be43d3f1f62bb0a9909734\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"f299f81f08116b725aaa299e3ca89a81107a47bd34be43d3f1f62bb0a9909734\",\"parent\":\"8c2e06607696bd4afb3d03b687e361cc43cf8ec1a4a725bc96e39f05ba97dd55\",\"created\":\"2015-07-27T17:42:14.06035004Z\",\"container\":\"6a8de25300280150268c572188e254550388e70d35a62eb8c18cd41db15e726a\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) MAINTAINER https://github.com/cloudfoundry/diego-dockerfiles\"],\"Image\":\"8c2e06607696bd4afb3d03b687e361cc43cf8ec1a4a725bc96e39f05ba97dd55\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.7.1\",\"author\":\"https://github.com/cloudfoundry/diego-dockerfiles\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\"],\"Image\":\"8c2e06607696bd4afb3d03b687e361cc43cf8ec1a4a725bc96e39f05ba97dd55\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"8c2e06607696bd4afb3d03b687e361cc43cf8ec1a4a725bc96e39f05ba97dd55\",\"parent\":\"6ce2e90b0bc7224de3db1f0d646fe8e2c4dd37f1793928287f6074bc451a57ea\",\"created\":\"2015-04-17T22:01:13.062208605Z\",\"container\":\"811003e0012ef6e6db039bcef852098d45cf9f84e995efb93a176a11e9aca6b9\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"/bin/sh\\\"]\"],\"Image\":\"6ce2e90b0bc7224de3db1f0d646fe8e2c4dd37f1793928287f6074bc451a57ea\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.6.0\",\"author\":\"Jérôme Petazzoni \\u003cjerome@docker.com\\u003e\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\"],\"Image\":\"6ce2e90b0bc7224de3db1f0d646fe8e2c4dd37f1793928287f6074bc451a57ea\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"6ce2e90b0bc7224de3db1f0d646fe8e2c4dd37f1793928287f6074bc451a57ea\",\"parent\":\"cf2616975b4a3cba083ca99bc3f0bf25f5f528c3c52be1596b30f60b0b1c37ff\",\"created\":\"2015-04-17T22:01:12.62756842Z\",\"container\":\"19bbb9ebab4da181db898c79bbdd8d2a8010dc2e4a7dc8b1a24d4b5eff99c5b4\",\"container_config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:8cf517d90fe79547c474641cc1e6425850e04abbd8856718f7e4a184ea878538 in /\"],\"Image\":\"cf2616975b4a3cba083ca99bc3f0bf25f5f528c3c52be1596b30f60b0b1c37ff\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.6.0\",\"author\":\"Jérôme Petazzoni \\u003cjerome@docker.com\\u003e\",\"config\":{\"Hostname\":\"19bbb9ebab4d\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"cf2616975b4a3cba083ca99bc3f0bf25f5f528c3c52be1596b30f60b0b1c37ff\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":2433303}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"cf2616975b4a3cba083ca99bc3f0bf25f5f528c3c52be1596b30f60b0b1c37ff\",\"created\":\"2015-04-17T22:01:05.451579326Z\",\"container\":\"39e791194498a7ee0c10e290ef51dd1ecde1f0fd83db977ddae38910b02b6fb9\",\"container_config\":{\"Hostname\":\"39e791194498\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) MAINTAINER Jérôme Petazzoni \\u003cjerome@docker.com\\u003e\"],\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.6.0\",\"author\":\"Jérôme Petazzoni \\u003cjerome@docker.com\\u003e\",\"config\":{\"Hostname\":\"39e791194498\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "PAH5:7YBZ:CDN2:6FNN:7CEE:6B2X:QJLA:TEW4:FJDO:M7PE:JHDA:WVUC",
               "kty": "EC",
               "x": "HKEbXwisv2y0hpNthl6XvgtROTr_JukIi5qFQJ5bE8I",
               "y": "JsVOxrCIY4Osa2PJb4SnPtw9KsDM62ZsSmdQ0YBldt0"
            },
            "alg": "ES256"
         },
         "signature": "D_sOhChLwN780Qzy_wCENSb1lZtDbItOvOP6_hGni9jrAs7wCsXiLaOT-XQ5Z_h6cCW_uJ5o_wmXbFNocaYqXg",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjI3MDY3LCJmb3JtYXRUYWlsIjoiQ24wIiwidGltZSI6IjIwMTUtMTAtMTlUMjI6MDc6MDBaIn0"
      }
   ]
 }`

	var (
		server *ghttp.Server
	)

	setupPingableRegistry := func() {
		server.AllowUnhandledRequests = true
		server.AppendHandlers(
			ghttp.VerifyRequest("GET", "/v2/"),
		)
	}

	setupSlowRegistry := func(tag ...string) {
		tagString := "latest"
		if len(tag) > 0 {
			tagString = tag[0]
		}

		server.AppendHandlers(
			ghttp.RespondWith(200, "Push failed due to a network error. Please try again. If the problem persists, it may be due to a slow connection."),
			ghttp.RespondWith(200, "Push failed due to a network error. Please try again. If the problem persists, it may be due to a slow connection."),
			ghttp.RespondWith(200, "Push failed due to a network error. Please try again. If the problem persists, it may be due to a slow connection."),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+tagString),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("X-Docker-Token", "token-1,token-2")
					w.Write([]byte(response))
				}),
			),
		)
	}

	setupRegistry := func(tag ...string) {
		tagString := "latest"
		if len(tag) > 0 {
			tagString = tag[0]
		}

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+tagString),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("X-Docker-Token", "token-1,token-2")
					w.Write([]byte(response))
				}),
			),
		)
	}

	resultJSON := func(filename string) []byte {
		resultInfo, err := ioutil.ReadFile(filename)
		Expect(err).NotTo(HaveOccurred())

		return resultInfo
	}

	Describe("ParseDockerRef", func() {
		Context("when the repo image is from dockerhub", func() {
			It("prepends 'library/' to the repo Name if there is no '/' character", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("redis")
				Expect(repositoryURL).To(Equal("registry-1.docker.io"))
				Expect(repoName).To(Equal("library/redis"))
			})

			It("does not prepends 'library/' to the repo Name if there is a '/' ", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("b/c")
				Expect(repositoryURL).To(Equal("registry-1.docker.io"))
				Expect(repoName).To(Equal("b/c"))
			})
		})

		Context("When the registryURL is not dockerhub", func() {
			It("does not add a '/' character to a single repo name", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("foobar:5123/baz")
				Expect(repositoryURL).To(Equal("foobar:5123"))
				Expect(repoName).To(Equal("baz"))
			})
		})

		Context("Parsing tags", func() {
			It("should parse tags based off the last colon", func() {
				_, _, tag := helpers.ParseDockerRef("baz/bot:test")
				Expect(tag).To(Equal("test"))
			})

			It("should default the tag to latest", func() {
				_, _, tag := helpers.ParseDockerRef("redis")
				Expect(tag).To(Equal("latest"))
			})
		})
	})

	Describe("FetchMetadata", func() {
		var registryURL string
		var repoName string
		var tag string
		var insecureRegistries []string

		BeforeEach(func() {
			server = ghttp.NewUnstartedServer()

			setupPingableRegistry()

			registryURL = server.Addr()

			repoName = ""
			tag = "latest"

			insecureRegistries = []string{}
		})

		JustBeforeEach(func() {
			if server.HTTPTestServer.TLS != nil {
				server.HTTPTestServer.StartTLS()
			} else {
				server.Start()
			}
			var err error
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with an invalid host", func() {
			BeforeEach(func() {
				registryURL = "qewr:5431"
				repoName = "some_user/some_repo"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with an unknown repository", func() {
			BeforeEach(func() {
				server.AllowUnhandledRequests = true
				repoName = "some_user/not_some_repo"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with an unknown tag", func() {
			BeforeEach(func() {
				server.AllowUnhandledRequests = true
				repoName = "some_user/some_repo"
				tag = "not_some_tag"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with a valid repository reference", func() {
			BeforeEach(func() {
				setupRegistry()

				repoName = "some_user/some_repo"
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the top-most image layer metadata", func() {
				img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(img).NotTo(BeNil())
				Expect(img.Config).NotTo(BeNil())
				Expect(img.Config.Cmd).NotTo(BeNil())
				Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
			})
		})

		Context("when the network connection is slow", func() {
			BeforeEach(func() {
				repoName = "some_user/some_repo"
				tag = "some-tag"

				setupSlowRegistry(tag)
			})

			It("should retry 3 times", func() {
				stderr := gbytes.NewBuffer()
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, stderr)
				Expect(err).NotTo(HaveOccurred())

				Expect(stderr).To(gbytes.Say("retry attempt: 1"))
				Expect(stderr).To(gbytes.Say("retry attempt: 2"))
				Expect(stderr).To(gbytes.Say("retry attempt: 3"))
			})
		})

		Context("with a valid repository:tag reference", func() {
			BeforeEach(func() {
				repoName = "some_user/some_repo"
				tag = "some-tag"

				setupRegistry(tag)
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the top-most image layer metadata", func() {
				img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(img).NotTo(BeNil())
				Expect(img.Config).NotTo(BeNil())
				Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
			})
		})

		Context("when the image exposes custom ports", func() {
			BeforeEach(func() {
				setupRegistry()

				repoName = "some_user/some_repo"
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the exposed ports", func() {
				img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(img.Config).NotTo(BeNil())

				Expect(img.Config.ExposedPorts).To(HaveKeyWithValue(nat.NewPort("tcp", "8080"), struct{}{}))
			})
		})

		Context("with an insecure registry", func() {
			BeforeEach(func() {
				setupRegistry()
				server.HTTPTestServer.TLS = &tls.Config{InsecureSkipVerify: false}
				insecureRegistries = append(insecureRegistries, server.Addr())

				repoName = "some_user/some_repo"
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the top-most image layer metadata", func() {
				img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, nil, os.Stderr)
				Expect(img).NotTo(BeNil())
				Expect(img.Config).NotTo(BeNil())
				Expect(img.Config.Cmd).NotTo(BeNil())
				Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
			})
		})

		Context("with a private registry with token authorization", func() {
			var credStore auth.CredentialStore
			BeforeEach(func() {
				server = ghttp.NewUnstartedServer()
				registryURL = server.Addr()
				credStore = testCreds{"username", "password"}

				authenticateHeader := http.Header{}
				authenticateHeader.Add("WWW-Authenticate", fmt.Sprintf(`Bearer realm="http://%s/token"`, registryURL))
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(401, "", authenticateHeader),
					),
				)
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/token"),
						ghttp.VerifyBasicAuth("username", "password"),
						ghttp.RespondWith(200, `{"token":"tokenstring"}`),
					),
				)
			})

			Context("with a valid repository:tag reference", func() {
				BeforeEach(func() {
					repoName = "some_user/some_repo"
					tag = "some-tag"

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV("Authorization", "Bearer tokenstring"),
							ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+tag),
							http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
								w.Header().Set("X-Docker-Token", "token-1,token-2")
								w.Write([]byte(response))
							}),
						),
					)
				})

				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(img).NotTo(BeNil())
					Expect(img.Config).NotTo(BeNil())
					Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
				})
			})

			Context("with a valid repository:tag reference", func() {
				BeforeEach(func() {
					repoName = "some_user/some_repo"
					tag = "some-tag"

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV("Authorization", "Bearer tokenstring"),
							ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+tag),
							http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
								w.Header().Set("X-Docker-Token", "token-1,token-2")
								w.Write([]byte(response))
							}),
						),
					)
				})

				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(img).NotTo(BeNil())
					Expect(img.Config).NotTo(BeNil())
					Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
				})
			})

		})

		Context("with a private registry with basic authorization", func() {
			var credStore auth.CredentialStore
			BeforeEach(func() {
				server = ghttp.NewUnstartedServer()
				registryURL = server.Addr()
				credStore = testCreds{"username", "password"}

				authenticateHeader := http.Header{}
				authenticateHeader.Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="http://%s/token"`, registryURL))
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(401, "", authenticateHeader),
					),
				)
			})

			Context("with a valid repository:tag reference", func() {
				BeforeEach(func() {
					repoName = "some_user/some_repo"
					tag = "some-tag"

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyBasicAuth("username", "password"),
							ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+tag),
							http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
								w.Header().Set("X-Docker-Token", "token-1,token-2")
								w.Write([]byte(response))
							}),
						),
					)
				})

				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					img, _ := helpers.FetchMetadata(registryURL, repoName, tag, insecureRegistries, credStore, os.Stderr)
					Expect(img).NotTo(BeNil())
					Expect(img.Config).NotTo(BeNil())
					Expect(img.Config.Cmd).To(Equal([]string{"./dockerapp"}))
				})
			})
		})
	})

	Context("SaveMetadata", func() {
		var metadata protocol.DockerImageMetadata
		var outputDir string

		BeforeEach(func() {
			metadata = protocol.DockerImageMetadata{
				ExecutionMetadata: protocol.ExecutionMetadata{
					Cmd:        []string{"fake-arg1", "fake-arg2"},
					Entrypoint: []string{"fake-cmd", "fake-arg0"},
					Workdir:    "/fake-workdir",
				},
				DockerImage: "cloudfoundry/diego-docker-app",
			}
		})

		Context("to an unwritable path on disk", func() {
			It("should error", func() {
				err := helpers.SaveMetadata("////tmp/", &metadata)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("with a writable path on disk", func() {

			BeforeEach(func() {
				var err error
				outputDir, err = ioutil.TempDir(os.TempDir(), "metadata")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(outputDir)
			})

			It("should output a json file", func() {
				filename := path.Join(outputDir, "result.json")
				err := helpers.SaveMetadata(filename, &metadata)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.Stat(filename)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("the json", func() {
				verifyMetadata := func(expectedEntryPoint []string, expectedStartCmd string) {
					err := helpers.SaveMetadata(path.Join(outputDir, "result.json"), &metadata)
					Expect(err).NotTo(HaveOccurred())
					result := resultJSON(path.Join(outputDir, "result.json"))

					var stagingResult dockerapplifecycle.StagingResult
					err = json.Unmarshal(result, &stagingResult)
					Expect(err).NotTo(HaveOccurred())

					Expect(stagingResult.ExecutionMetadata).NotTo(BeEmpty())
					Expect(stagingResult.ProcessTypes).NotTo(BeEmpty())

					var executionMetadata protocol.ExecutionMetadata
					err = json.Unmarshal([]byte(stagingResult.ExecutionMetadata), &executionMetadata)
					Expect(err).NotTo(HaveOccurred())

					Expect(executionMetadata.Cmd).To(Equal(metadata.ExecutionMetadata.Cmd))
					Expect(executionMetadata.Entrypoint).To(Equal(expectedEntryPoint))
					Expect(executionMetadata.Workdir).To(Equal(metadata.ExecutionMetadata.Workdir))

					Expect(stagingResult.ProcessTypes).To(HaveLen(1))
					Expect(stagingResult.ProcessTypes).To(HaveKeyWithValue("web", expectedStartCmd))

					Expect(stagingResult.LifecycleMetadata.DockerImage).To(Equal(metadata.DockerImage))
				}

				It("should contain the metadata", func() {
					verifyMetadata(metadata.ExecutionMetadata.Entrypoint, "fake-cmd fake-arg0 fake-arg1 fake-arg2")
				})

				Context("when the EntryPoint is empty", func() {
					BeforeEach(func() {
						metadata.ExecutionMetadata.Entrypoint = []string{}
					})

					It("contains all but the EntryPoint", func() {
						verifyMetadata(nil, "fake-arg1 fake-arg2")
					})
				})

				Context("when the EntryPoint is nil", func() {
					BeforeEach(func() {
						metadata.ExecutionMetadata.Entrypoint = nil
					})

					It("contains all but the EntryPoint", func() {
						verifyMetadata(metadata.ExecutionMetadata.Entrypoint, "fake-arg1 fake-arg2")
					})
				})
			})
		})
	})
})
