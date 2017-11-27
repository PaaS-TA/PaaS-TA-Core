package cf_tls_upgrade_test

import "github.com/pivotal-cf-experimental/destiny/ops"

func addEtcdTLSInstanceGroup(manifest, varsStore string) (string, error) {
	etcdClientCA, err := ops.FindOp(varsStore, "/etcd_client/ca")
	if err != nil {
		return "", err
	}

	etcdClientCertificate, err := ops.FindOp(varsStore, "/etcd_client/certificate")
	if err != nil {
		return "", err
	}

	etcdClientPrivateKey, err := ops.FindOp(varsStore, "/etcd_client/private_key")
	if err != nil {
		return "", err
	}

	etcdServerCA, err := ops.FindOp(varsStore, "/etcd_server/ca")
	if err != nil {
		return "", err
	}

	etcdServerCertificate, err := ops.FindOp(varsStore, "/etcd_server/certificate")
	if err != nil {
		return "", err
	}

	etcdServerPrivateKey, err := ops.FindOp(varsStore, "/etcd_server/private_key")
	if err != nil {
		return "", err
	}

	etcdPeerCA, err := ops.FindOp(varsStore, "/etcd_peer/ca")
	if err != nil {
		return "", err
	}

	etcdPeerCertificate, err := ops.FindOp(varsStore, "/etcd_peer/certificate")
	if err != nil {
		return "", err
	}

	etcdPeerPrivateKey, err := ops.FindOp(varsStore, "/etcd_peer/private_key")
	if err != nil {
		return "", err
	}

	etcd, err := ops.FindOp(manifest, "/instance_groups/name=etcd")
	if err != nil {
		return "", err
	}

	return ops.ApplyOps(manifest, []ops.Op{
		// --- Add an etcd-tls group, keep the etcd group ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/name",
			Value: "etcd-non-tls",
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/-",
			Value: etcd,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/name",
			Value: "etcd-tls",
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-non-tls/name",
			Value: "etcd",
		},

		// --- Add consul agent job ---
		{
			Type: "replace",
			Path: "/instance_groups/name=etcd-tls/jobs/-",
			Value: map[string]interface{}{
				"name":    "consul_agent",
				"release": "consul",
				"consumes": map[string]interface{}{
					"consul": map[string]string{
						"from": "consul_server",
					},
				},
				"properties": map[string]interface{}{
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"services": map[string]interface{}{
								"etcd": map[string]string{
									"name": "cf-etcd",
								},
							},
						},
					},
				},
			},
		},

		// --- Add cluster properties ---
		{
			Type: "replace",
			Path: "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/cluster?/-",
			Value: map[string]interface{}{
				"name":      "etcd",
				"instances": 3,
			},
		},

		// --- Remove static ips ---
		{
			Type: "remove",
			Path: "/instance_groups/name=etcd-tls/networks/name=default/static_ips",
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/machines",
			Value: []string{"cf-etcd.service.cf.internal"},
		},

		// --- Enable ssl requirements and add certs/keys ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/peer_require_ssl",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/require_ssl",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/ca_cert?",
			Value: etcdClientCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/server_cert?",
			Value: etcdServerCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/server_key?",
			Value: etcdServerPrivateKey.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/peer_ca_cert?",
			Value: etcdPeerCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/peer_cert?",
			Value: etcdPeerCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd/properties/etcd/peer_key?",
			Value: etcdPeerPrivateKey.(string),
		},

		// --- Remove static ip of etcd instance for the etcd_metrics_server ---
		{
			Type: "remove",
			Path: "/instance_groups/name=etcd-tls/jobs/name=etcd_metrics_server/properties/etcd_metrics_server/etcd/machine",
		},

		// --- Enable tls communication for etcd_metrics_server ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd_metrics_server/properties/etcd_metrics_server/etcd/require_ssl",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd_metrics_server/properties/etcd_metrics_server/etcd/ca_cert?",
			Value: etcdServerCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd_metrics_server/properties/etcd_metrics_server/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd-tls/jobs/name=etcd_metrics_server/properties/etcd_metrics_server/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},
	})
}

func convertNonTLSEtcdToProxy(manifest, varsStore string) (string, error) {
	etcdClientCA, err := ops.FindOp(varsStore, "/etcd_client/ca")
	if err != nil {
		return "", err
	}

	etcdClientCertificate, err := ops.FindOp(varsStore, "/etcd_client/certificate")
	if err != nil {
		return "", err
	}

	etcdClientPrivateKey, err := ops.FindOp(varsStore, "/etcd_client/private_key")
	if err != nil {
		return "", err
	}

	etcdServerCA, err := ops.FindOp(varsStore, "/etcd_server/ca")
	if err != nil {
		return "", err
	}

	return ops.ApplyOps(manifest, []ops.Op{
		// --- Rename etcd job template with etcd_proxy ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/jobs/name=etcd/name",
			Value: "etcd_proxy",
		},

		// --- Scale etcd_proxy down to 1 instance ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/instances",
			Value: 1,
		},

		// --- Add static ip for the etcd_proxy machine ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/networks/name=default/static_ips?",
			Value: []string{"10.0.31.231"},
		},

		// --- Reduce etcd_proxy to 1 az ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/azs",
			Value: []string{"z1"},
		},

		// --- Add consul agent without advertising consul service ---
		{
			Type: "replace",
			Path: "/instance_groups/name=etcd/jobs/-",
			Value: map[string]interface{}{
				"name":    "consul_agent",
				"release": "consul",
				"consumes": map[string]interface{}{
					"consul": map[string]string{
						"from": "consul_server",
					},
				},
			},
		},

		// Set the etcd_proxy properties ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/jobs/name=etcd_proxy/properties/etcd_proxy?/etcd/dns_suffix",
			Value: "cf-etcd.service.cf.internal",
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/jobs/name=etcd_proxy/properties/etcd_proxy/etcd/ca_cert?",
			Value: etcdClientCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/jobs/name=etcd_proxy/properties/etcd_proxy/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=etcd/jobs/name=etcd_proxy/properties/etcd_proxy/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},
		{
			Type: "remove",
			Path: "/instance_groups/name=etcd/jobs/name=etcd_proxy/properties/etcd",
		},

		// --- Remove the etcd_metrics_server from the etcd_proxy ---
		{
			Type: "remove",
			Path: "/instance_groups/name=etcd/jobs/name=etcd_metrics_server",
		},

		// --- Updating diego bbs job
		{
			Type:  "replace",
			Path:  "/instance_groups/name=diego-bbs/jobs/name=bbs/properties/diego/bbs/etcd/require_ssl?",
			Value: false,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=diego-bbs/jobs/name=bbs/properties/diego/bbs/etcd/machines",
			Value: nil,
		},

		// --- Enable etcd tls communication for doppler ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=doppler/properties/doppler/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=doppler/properties/doppler/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=doppler/properties/loggregator/etcd/require_ssl?",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=doppler/properties/loggregator/etcd/ca_cert?",
			Value: etcdServerCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=doppler/properties/loggregator/etcd/machines?",
			Value: []string{"cf-etcd.service.cf.internal"},
		},

		// --- Enable etcd tls communication for syslog_drain_binder ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=syslog_drain_binder/properties/loggregator/etcd/require_ssl?",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=syslog_drain_binder/properties/loggregator/etcd/ca_cert?",
			Value: etcdServerCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=syslog_drain_binder/properties/loggregator/etcd/machines",
			Value: []string{"cf-etcd.service.cf.internal"},
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=syslog_drain_binder/properties/syslog_drain_binder/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=doppler/jobs/name=syslog_drain_binder/properties/syslog_drain_binder/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},

		// --- Enable etcd tls communication for loggregator_trafficcontroller ---
		{
			Type:  "replace",
			Path:  "/instance_groups/name=log-api/jobs/name=loggregator_trafficcontroller/properties/traffic_controller/etcd/client_cert?",
			Value: etcdClientCertificate.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=log-api/jobs/name=loggregator_trafficcontroller/properties/traffic_controller/etcd/client_key?",
			Value: etcdClientPrivateKey.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=log-api/jobs/name=loggregator_trafficcontroller/properties/loggregator/etcd/require_ssl?",
			Value: true,
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=log-api/jobs/name=loggregator_trafficcontroller/properties/loggregator/etcd/ca_cert?",
			Value: etcdServerCA.(string),
		},
		{
			Type:  "replace",
			Path:  "/instance_groups/name=log-api/jobs/name=loggregator_trafficcontroller/properties/loggregator/etcd/machines?",
			Value: []string{"cf-etcd.service.cf.internal"},
		},
	})
}
