package app

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/cnrancher/cube-apiserver/api"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/urfave/cli"
)

const (
	ListenAddress  = "listen-addr"
	ConfigLocation = "kube-config"
)

func APIServerCmd() cli.Command {
	flag.Parse()
	return cli.Command{
		Name: "serve",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  ListenAddress,
				Usage: "Specify apiServer listen address",
			},
			cli.StringFlag{
				Name:  ConfigLocation,
				Usage: "Specify apiServer kubernetes config location",
			},
		},
		Action: func(c *cli.Context) error {
			if err := startAPIServer(c); err != nil {
				logrus.Errorf("RancherCUBE: error starting apiServer: %v", err)
				return err
			}
			return nil
		},
	}
}

func startAPIServer(c *cli.Context) error {
	apiServerListenAddr := c.String(ListenAddress)
	apiServerKubeConfigLocation := c.String(ConfigLocation)
	if "" == apiServerListenAddr {
		return fmt.Errorf("RancherCUBE: require %v", ListenAddress)
	}
	if "" == apiServerKubeConfigLocation {
		return fmt.Errorf("RancherCUBE: require %v", ConfigLocation)
	}

	clientGenerator := backend.NewClientGenerator(apiServerKubeConfigLocation)

	// generate & deploy customer resources
	clientGenerator.UserCRDDeploy()
	clientGenerator.InfrastructureCRDDeploy()

	done := make(chan struct{})

	infraController := controller.NewInfraController(&clientGenerator.Clientset, &clientGenerator.Infraclientset, clientGenerator.InformerFactory, clientGenerator.CubeInformerFactory)
	userController := controller.NewUserController(&clientGenerator.Clientset, &clientGenerator.Infraclientset, clientGenerator.InformerFactory, clientGenerator.CubeInformerFactory)

	go clientGenerator.InformerFactory.Start(done)
	go clientGenerator.CubeInformerFactory.Start(done)

	go func() {
		if err := infraController.Run(2, done); err != nil {
			glog.Fatalf("Error running infrastructure controller: %s", err.Error())
		}
	}()

	go func() {
		if err := userController.Run(2, done); err != nil {
			glog.Fatalf("Error running user controller: %s", err.Error())
		}
	}()

	server := api.NewServer(clientGenerator)
	router := http.Handler(api.NewRouter(server))

	logrus.Infof("RancherCUBE: listening on %s", apiServerListenAddr)

	go http.ListenAndServe(apiServerListenAddr, router)

	util.RegisterShutdownChannel(done)
	<-done

	return nil
}
