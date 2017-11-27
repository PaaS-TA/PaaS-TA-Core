package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/cf-routing-test-helpers/helpers"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	uaaconfig "code.cloudfoundry.org/uaa-go-client/config"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/config"
	cfworkflow_helpers "github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	"github.com/nu7hatch/gouuid"

	. "github.com/onsi/gomega"
)

type RoutingConfig struct {
	*config.Config
	RoutingApiUrl     string       `json:"-"` //"-" is used for ignoring field
	Addresses         []string     `json:"addresses"`
	OAuth             *OAuthConfig `json:"oauth"`
	IncludeHttpRoutes bool         `json:"include_http_routes"`
	TcpAppDomain      string       `json:"tcp_apps_domain"`
	LBConfigured      bool         `json:"lb_configured"`
	TCPRouterGroup    string       `json:"tcp_router_group"`
}

type OAuthConfig struct {
	TokenEndpoint string `json:"token_endpoint"`
	ClientName    string `json:"client_name"`
	ClientSecret  string `json:"client_secret"`
	Port          int    `json:"port"`
}

func loadDefaultTimeout(conf *RoutingConfig) {
	if conf.DefaultTimeout <= 0 {
		conf.DefaultTimeout = 120
	}

	if conf.CfPushTimeout <= 0 {
		conf.CfPushTimeout = 120
	}
}
func LoadConfig() RoutingConfig {
	loadedConfig := loadConfigJsonFromPath()

	loadedConfig.Config = config.LoadConfig()
	loadDefaultTimeout(&loadedConfig)

	if loadedConfig.OAuth == nil {
		panic("missing configuration oauth")
	}

	if len(loadedConfig.Addresses) == 0 {
		panic("missing configuration 'addresses'")
	}

	if loadedConfig.AppsDomain == "" {
		panic("missing configuration apps_domain")
	}

	if loadedConfig.ApiEndpoint == "" {
		panic("missing configuration api")
	}

	if loadedConfig.TCPRouterGroup == "" {
		panic("missing configuration tcp_router_group")
	}

	loadedConfig.RoutingApiUrl = fmt.Sprintf("https://%s", loadedConfig.ApiEndpoint)

	return loadedConfig
}

func ValidateRouterGroupName(context cfworkflow_helpers.UserContext, tcpRouterGroup string) {
	var routerGroupOutput string
	cfworkflow_helpers.AsUser(context, context.Timeout, func() {
		routerGroupOutput = string(cf.Cf("router-groups").Wait(context.Timeout).Out.Contents())
	})

	Expect(routerGroupOutput).To(MatchRegexp(fmt.Sprintf("%s\\s+tcp", tcpRouterGroup)), fmt.Sprintf("Router group %s of type tcp doesn't exist", tcpRouterGroup))
}

func NewUaaClient(routerApiConfig RoutingConfig, logger lager.Logger) uaaclient.Client {

	tokenURL := fmt.Sprintf("%s:%d", routerApiConfig.OAuth.TokenEndpoint, routerApiConfig.OAuth.Port)

	cfg := &uaaconfig.Config{
		UaaEndpoint:           tokenURL,
		SkipVerification:      routerApiConfig.SkipSSLValidation,
		ClientName:            routerApiConfig.OAuth.ClientName,
		ClientSecret:          routerApiConfig.OAuth.ClientSecret,
		MaxNumberOfRetries:    3,
		RetryInterval:         500 * time.Millisecond,
		ExpirationBufferInSec: 30,
	}

	uaaClient, err := uaaclient.NewClient(logger, cfg, clock.NewClock())
	Expect(err).ToNot(HaveOccurred())

	_, err = uaaClient.FetchToken(true)
	Expect(err).ToNot(HaveOccurred())

	return uaaClient
}

func UpdateOrgQuota(context cfworkflow_helpers.UserContext) {
	os.Setenv("CF_TRACE", "false")
	cfworkflow_helpers.AsUser(context, context.Timeout, func() {
		orgGuid := cf.Cf("org", context.Org, "--guid").Wait(context.Timeout).Out.Contents()
		quotaUrl, err := helpers.GetOrgQuotaDefinitionUrl(string(orgGuid), context.Timeout)
		Expect(err).NotTo(HaveOccurred())

		cf.Cf("curl", quotaUrl, "-X", "PUT", "-d", "'{\"total_reserved_route_ports\":-1}'").Wait(context.Timeout)
	})
}

func loadConfigJsonFromPath() RoutingConfig {
	var config RoutingConfig

	path := configPath()

	configFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		panic(err)
	}

	return config
}

func configPath() string {
	path := os.Getenv("CONFIG")
	if path == "" {
		panic("Must set $CONFIG to point to an integration config .json file.")
	}

	return path
}

func RandomName() string {
	guid, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}

	return guid.String()
}
