package handlers

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/testconsumer/etcd"

	goetcd "github.com/coreos/go-etcd/etcd"
)

const KEY_NOT_FOUND = "Key not found"

type KVHandler struct {
	etcdURLs   []string
	caCert     string
	clientCert string
	clientKey  string
}

func NewKVHandler(etcdURLs []string, caCert, clientCert, clientKey string) KVHandler {
	return KVHandler{
		etcdURLs:   etcdURLs,
		caCert:     caCert,
		clientCert: clientCert,
		clientKey:  clientKey,
	}
}

func (k *KVHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	splitPath := strings.Split(req.URL.Path, "/")
	key := splitPath[len(splitPath)-1]

	var goEtcdClient *goetcd.Client

	if k.caCert != "" && k.clientCert != "" && k.clientKey != "" {
		goEtcdClient, err = goetcd.NewTLSClient(k.etcdURLs, k.clientCert, k.clientKey, k.caCert)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		goEtcdClient = goetcd.NewClient(k.etcdURLs)
	}

	goEtcdClient.SetConsistency(goetcd.STRONG_CONSISTENCY)

	client := etcd.NewClient(goEtcdClient)
	defer client.Close()

	switch req.Method {
	case "GET":
		value, err := client.Get(key)
		if err != nil {
			if strings.Contains(err.Error(), KEY_NOT_FOUND) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

		w.Write([]byte(value))

	case "PUT":
		value, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = client.Set(key, string(value))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
