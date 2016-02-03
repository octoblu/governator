package deployer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/octoblu/go-simple-etcd-client/etcdclient"
)

// Deployer watches a redis queue
// and deploys services using Etcd
type Deployer struct {
	etcdClient etcdclient.EtcdClient
	redisConn  redis.Conn
	queueName  string
}

// RequestMetadata is the metadata of the request
type RequestMetadata struct {
	EtcdDir   string `json:"etcdDir"`
	DockerURL string `json:"dockerUrl"`
}

// New constructs a new deployer instance
func New(etcdClient etcdclient.EtcdClient, redisConn redis.Conn, queueName string) *Deployer {
	return &Deployer{etcdClient, redisConn, queueName}
}

// Run watches the redis queue and starts taking action
func (deployer *Deployer) Run() error {
	metadata, err := deployer.getNextValidDeploy()
	if err != nil {
		return err
	}

	if metadata == nil {
		return nil
	}

	dockerURLKey := fmt.Sprintf("%v/docker_url", metadata.EtcdDir)
	_ = deployer.etcdClient.Set(dockerURLKey, metadata.DockerURL)

	restartKey := fmt.Sprintf("%v/restart", metadata.EtcdDir)
	_ = deployer.etcdClient.Set(restartKey, "")

	return nil
}

func (deployer *Deployer) getNextDeploy() (string, error) {
	now := time.Now().Unix()
	deploysResult, err := deployer.redisConn.Do("ZRANGEBYSCORE", deployer.queueName, 0, now)

	if err != nil {
		return "", err
	}

	deploys := deploysResult.([]string)
	if len(deploys) == 0 {
		return "", nil
	}

	return deploys[0], nil
}

func (deployer *Deployer) lockDeploy(deploy string) (bool, error) {
	zremResult, err := deployer.redisConn.Do("ZREM", deployer.queueName, deploy)

	if err != nil {
		return false, err
	}

	return (zremResult != 0), nil
}

func (deployer *Deployer) validateDeploy(deploy string) (bool, error) {
	existsResult, err := deployer.redisConn.Do("HEXISTS", deploy, "cancellation")

	if err != nil {
		return false, err
	}

	return (existsResult == 0), nil
}

func (deployer *Deployer) getMetadata(deploy string) (*RequestMetadata, error) {
	var metadata RequestMetadata

	metadataBytes, err := deployer.redisConn.Do("HGET", deploy, "request:metadata")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(metadataBytes.([]byte), &metadata)

	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (deployer *Deployer) getNextValidDeploy() (*RequestMetadata, error) {
	deploy, err := deployer.getNextDeploy()
	if err != nil {
		return nil, err
	}

	if deploy == "" {
		return nil, nil
	}

	ok, err := deployer.lockDeploy(deploy)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	ok, err = deployer.validateDeploy(deploy)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	return deployer.getMetadata(deploy)
}
