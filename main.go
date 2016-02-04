package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/fatih/color"
	"github.com/garyburd/redigo/redis"
	"github.com/octoblu/go-simple-etcd-client/etcdclient"
	"github.com/octoblu/governator/deployer"
	De "github.com/tj/go-debug"
)

var debug = De.Debug("governator:main")

func main() {
	app := cli.NewApp()
	app.Name = "governator"
	app.Version = version()
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "etcd-uri, e",
			EnvVar: "GOVERNATOR_ETCD_URI",
			Usage:  "Etcd server to deploy to",
		},
		cli.StringFlag{
			Name:   "redis-uri, r",
			EnvVar: "GOVERNATOR_REDIS_URI",
			Usage:  "Redis server to pull deployments from",
		},
		cli.StringFlag{
			Name:   "redis-queue, q",
			EnvVar: "GOVERNATOR_REDIS_QUEUE",
			Usage:  "Redis queue to pull deployments from",
		},
	}
	app.Run(os.Args)
}

func run(context *cli.Context) {
	etcdURI, redisURI, redisQueue := getOpts(context)

	etcdClient := getEtcdClient(etcdURI)
	redisConn := getRedisConn(redisURI)

	theDeployer := deployer.New(etcdClient, redisConn, redisQueue)

	for {
		debug("theDeployer.Run()")
		err := theDeployer.Run()
		if err != nil {
			log.Panic("Run error", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func getOpts(context *cli.Context) (string, string, string) {
	etcdURI := context.String("etcd-uri")
	redisURI := context.String("redis-uri")
	redisQueue := context.String("redis-queue")

	if etcdURI == "" || redisURI == "" || redisQueue == "" {
		cli.ShowAppHelp(context)

		if etcdURI == "" {
			color.Red("  Missing required flag --etcd-uri or GOVERNATOR_ETCD_URI")
		}
		if redisURI == "" {
			color.Red("  Missing required flag --redis-uri or GOVERNATOR_REDIS_URI")
		}
		if redisQueue == "" {
			color.Red("  Missing required flag --redis-queue or GOVERNATOR_REDIS_QUEUE")
		}
		os.Exit(1)
	}

	return etcdURI, redisURI, redisQueue
}

func getEtcdClient(etcdURI string) etcdclient.EtcdClient {
	etcdClient, err := etcdclient.New(etcdURI)
	if err != nil {
		log.Panicln("Error with etcdclient.New", err.Error())
	}
	return etcdClient
}

func getRedisConn(redisURI string) redis.Conn {
	redisConn, err := redis.DialURL(redisURI)
	if err != nil {
		log.Panicln("Error with redis.DialURL", err.Error())
	}
	return redisConn
}

func version() string {
	versionStr := "1.0.0"
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		errorMessage := fmt.Sprintf("Error with version number: %v", versionStr)
		log.Panicln(errorMessage, err.Error())
	}
	return version.String()
}
