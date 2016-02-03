package main

import (
	"fmt"
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/fatih/color"
)

func main() {
	app := cli.NewApp()
	app.Name = "governator"
	app.Version = version()
	app.Action = run
	app.Flags = []cli.Flag{
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
	redisURI := context.String("redis-uri")
	redisQueue := context.String("redis-queue")

	if redisURI == "" || redisQueue == "" {
		cli.ShowAppHelp(context)

		if redisURI == "" {
			color.Red("  Missing required flag --redis-uri or GOVERNATOR_REDIS_URI")
		}
		if redisQueue == "" {
			color.Red("  Missing required flag --redis-queue or GOVERNATOR_REDIS_QUEUE")
		}
		os.Exit(1)
	}
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
