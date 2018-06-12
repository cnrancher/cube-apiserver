package app

import (
	"fmt"
	"net/http"

	"github.com/cnrancher/cube-apiserver/api"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/provider/common"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"path/filepath"
	"os"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

const (
	ListenAddress  = "listen-addr"
	ConfigLocation = "kube-config"
	Config         = "config"
)

var clusterFilePath string

func APIServerCmd() cli.Command {
	return cli.Command{
		Name: "serve",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  ListenAddress,
				Usage: "Specify apiServer listen address",
			},
			//cli.StringFlag{
			//	Name:  ConfigLocation,
			//	Usage: "Specify apiServer kubernetes config location",
			//},
			cli.StringFlag{
				Name:  Config,
				Usage: "Specify an alternate cluster YAML file",
				Value: "config.yml",
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

func resolveClusterFile(ctx *cli.Context) (string, string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", "", fmt.Errorf("Can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, clusterFile, nil
}

func ParseConfig(clusterFile string) (*api.CubeConfig, error) {
	logrus.Debugf("Parsing cluster file [%v]", clusterFile)
	var rkeConfig api.CubeConfig
	if err := yaml.Unmarshal([]byte(clusterFile), &rkeConfig); err != nil {
		return nil, err
	}
	return &rkeConfig, nil
}

func startAPIServer(c *cli.Context) error {

	clusterFile, filePath, err := resolveClusterFile(c)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}
	if rkeConfig == nil {
		return fmt.Errorf("RancherCUBE: require %v", Config)
	}

	apiServerListenAddr := c.String(ListenAddress)
	apiServerKubeConfigLocation := /*c.String(ConfigLocation)*/ rkeConfig.KubeConfig
	if "" == apiServerListenAddr {
		return fmt.Errorf("RancherCUBE: require %v", ListenAddress)
	}
	if "" == apiServerKubeConfigLocation {
		return fmt.Errorf("RancherCUBE: require %v", ConfigLocation)
	}

	clientGenerator := backend.NewClientGenerator(rkeConfig.KubeConfig, &rkeConfig.InfraImages)

	// generate & deploy customer resources
	clientGenerator.UserCRDDeploy()
	clientGenerator.InfrastructureCRDDeploy()
	clientGenerator.LonghornEngineCRDDeploy()
	clientGenerator.LonghornReplicaCRDDeploy()
	clientGenerator.LonghornSettingCRDDeploy()
	clientGenerator.LonghornVolumeCRDDeploy()
	clientGenerator.CredentialCRDDeploy()
	clientGenerator.ArptableCRDDeploy()
	clientGenerator.VirtualMachineCRDDeploy()
	// generate & deploy config map resources for base info
	clientGenerator.ConfigMapDeploy()

	done := make(chan struct{})

	infraController := controller.NewInfraController(&clientGenerator.Clientset, &clientGenerator.Infraclientset, clientGenerator.InformerFactory, clientGenerator.CubeInformerFactory)
	userController := controller.NewUserController(&clientGenerator.Clientset, &clientGenerator.Infraclientset, clientGenerator.InformerFactory, clientGenerator.CubeInformerFactory)

	go func() {
		clientGenerator.InformerFactory.Start(done)
		clientGenerator.CubeInformerFactory.Start(done)
		common.Configure(clientGenerator)
	}()

	go func() {
		if err := infraController.Run(2, done); err != nil {
			logrus.Fatalf("RancherCUBE: error running infrastructure controller: %s", err.Error())
		}
	}()

	go func() {
		if err := userController.Run(2, done); err != nil {
			logrus.Fatalf("RancherCUBE: error running user controller: %s", err.Error())
		}
	}()

	server := api.NewServer(clientGenerator, apiServerKubeConfigLocation)
	router := http.Handler(api.NewRouter(server))

	logrus.Infof("RancherCUBE: listening on %s", apiServerListenAddr)

	go http.ListenAndServe(apiServerListenAddr, router)

	go StartPurgeDaemon(clientGenerator, done)

	util.RegisterShutdownChannel(done)
	<-done

	return nil
}
