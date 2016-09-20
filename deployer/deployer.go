package deployer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	De "github.com/tj/go-debug"
)

var debug = De.Debug("governator:deployer")

// Deployer watches a redis queue
// and deploys services using Etcd
type Deployer struct {
	etcdClient     EtcdClient
	redisConn      redis.Conn
	queueName      string
	deployStateUri string
	cluster        string
}

// RequestMetadata is the metadata of the request
type RequestMetadata struct {
	EtcdDir   string `json:"etcdDir"`
	DockerURL string `json:"dockerUrl"`
}

// New constructs a new deployer instance
func New(etcdClient EtcdClient, redisConn redis.Conn, queueName, deployStateUri, cluster string) *Deployer {
	return &Deployer{
		etcdClient:     etcdClient,
		redisConn:      redisConn,
		queueName:      queueName,
		deployStateUri: deployStateUri,
		cluster:        cluster,
	}
}

// Run watches the redis queue and starts taking action
func (deployer *Deployer) Run() error {
	deploy, err := deployer.getNextValidDeploy()
	if err != nil {
		return err
	}

	if deploy == nil {
		return nil
	}

	return deployer.deploy(deploy)
}

func (deployer *Deployer) getReleaseVersion(dockerURL string) string {
	parts := strings.Split(dockerURL, ":")
	return parts[len(parts)-1]
}

func (deployer *Deployer) getKey(key string) string {
	return fmt.Sprintf("%s:%s", deployer.queueName, key)
}

func (deployer *Deployer) deploy(metadata *RequestMetadata) error {
	var err error
	dockerURLKey := fmt.Sprintf("%v/docker_url", metadata.EtcdDir)
	err = deployer.etcdClient.Set(dockerURLKey, metadata.DockerURL)
	if err != nil {
		return err
	}

	releaseKey := fmt.Sprintf("%v/env/SENTRY_RELEASE", metadata.EtcdDir)
	err = deployer.etcdClient.Set(releaseKey, deployer.getReleaseVersion(metadata.DockerURL))
	if err != nil {
		return err
	}

	restartValue := fmt.Sprintf("%v", time.Now())
	restartKey := fmt.Sprintf("%v/restart", metadata.EtcdDir)
	err = deployer.etcdClient.Set(restartKey, restartValue)
	if err != nil {
		return err
	}

	err = deployer.notifyDeployState(metadata.DockerURL)
	if err != nil {
		return err
	}

	return nil
}

func (deployer *Deployer) getNextDeploy() (string, error) {
	now := time.Now().Unix()
	deploysResult, err := deployer.redisConn.Do("ZRANGEBYSCORE", deployer.getKey("governator:deploys"), 0, now)

	if err != nil {
		return "", err
	}

	deploys := deploysResult.([]interface{})
	if len(deploys) == 0 {
		return "", nil
	}

	return string(deploys[0].([]byte)), nil
}

func (deployer *Deployer) lockDeploy(deploy string) (bool, error) {
	debug("lockDeploy: %v", deploy)
	zremResult, err := deployer.redisConn.Do("ZREM", deployer.getKey("governator:deploys"), deploy)

	if err != nil {
		return false, err
	}

	result := zremResult.(int64)

	return (result != 0), nil
}

func (deployer *Deployer) validateDeploy(deploy string) (bool, error) {
	debug("validateDeploy: %v", deploy)
	existsResult, err := deployer.redisConn.Do("HEXISTS", deployer.getKey(deploy), "cancellation")

	if err != nil {
		return false, err
	}

	exists := existsResult.(int64)
	return (exists == 0), nil
}

func (deployer *Deployer) getMetadata(deploy string) (*RequestMetadata, error) {
	debug("getMetadata: %v", deploy)
	var metadata RequestMetadata

	metadataBytes, err := deployer.redisConn.Do("HGET", deployer.getKey(deploy), "request:metadata")
	if err != nil {
		return nil, err
	}

	if metadataBytes == nil {
		return nil, fmt.Errorf("Deploy metadata not found for '%v'", deploy)
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
		debug("Failed to obtain lock for: %v", deploy)
		return nil, nil
	}

	ok, err = deployer.validateDeploy(deploy)
	if err != nil {
		return nil, err
	}

	if !ok {
		debug("Deploy was cancelled: %v", deploy)
		return nil, nil
	}

	return deployer.getMetadata(deploy)
}

func (deployer *Deployer) notifyDeployState(dockerURL string) error {
	var owner, repo, tag string

	dockerURLParts := strings.Split(dockerURL, ":")

	if len(dockerURLParts) != 2 {
		return errors.New("invalid docker url")
	}

	if dockerURLParts[1] != "" {
		tag = dockerURLParts[1]
	}

	projectParts := strings.Split(dockerURLParts[0], "/")

	if len(projectParts) == 2 {
		owner = projectParts[0]
		repo = projectParts[1]
	} else if len(projectParts) == 3 {
		owner = projectParts[1]
		repo = projectParts[2]
	} else {
		return errors.New("invalid base docker url")
	}

	uri := fmt.Sprintf("deployments/%s/%s/%s/cluster/%s/passed", owner, repo, tag, deployer.cluster)
	fullUrl := fmt.Sprintf("%s/%s", deployer.deployStateUri, uri)

	debug("making request to %s", fullUrl)
	client := &http.Client{}
	request, err := http.NewRequest("PUT", fullUrl, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	debug("Response StatusCode %v", response.StatusCode)

	response.Body.Close()
	if response.StatusCode > 399 {
		return errors.New("invalid response from deploy-state-service")
	}
	return nil
}
