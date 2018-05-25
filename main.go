package main

import (
	"fmt"
	"os"

	"github.com/cnrancher/cube-apiserver/app"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func cmdNotFound(c *cli.Context, command string) {
	panic(fmt.Errorf("RancherCUBE: unrecognized command: %s", command))
}

func onUsageError(c *cli.Context, err error, isSubcommand bool) error {
	panic(fmt.Errorf("RancherCUBE: usage error, please check your command"))
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})

	a := cli.NewApp()
	a.Name = "RancherCUBE"
	a.Version = VERSION
	a.Usage = "RancherCUBE"
	a.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	a.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "enable debug logging level",
			EnvVar: "RANCHER_DEBUG",
		},
	}

	a.Commands = []cli.Command{
		app.APIServerCmd(),
	}
	a.CommandNotFound = cmdNotFound
	a.OnUsageError = onUsageError

	if err := a.Run(os.Args); err != nil {
		logrus.Fatalf("RancherCUBE: critical error: %v", err)
	}
}
