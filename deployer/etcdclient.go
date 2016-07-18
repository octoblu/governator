package deployer

import (
	"github.com/octoblu/go-simple-etcd-client/etcdclient"
)

// EtcdClient defines the methods needed from the etcdclient
type EtcdClient interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// SimpleEtcdClient implements the EtcdClient
type SimpleEtcdClient struct {
	etcdclient.EtcdClient
}

// NewEtcdClient creates a new instance of the etcd client
func NewEtcdClient(client etcdclient.EtcdClient) EtcdClient {
	return &SimpleEtcdClient{client}
}
