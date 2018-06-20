package app

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"github.com/cnrancher/cube-apiserver/api"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/provider/common"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	ListenAddress  = "listen-addr"
	ConfigLocation = "kube-config"
	Config         = "config"
	ConfigFile     = "config.yml"
)

func APIServerCmd() cli.Command {
	return cli.Command{
		Name: "serve",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  ListenAddress,
				Usage: "Specify apiServer listen address",
			},
			cli.StringFlag{
				Name:  Config,
				Usage: "Specify an alternate cluster YAML file",
				Value: ConfigFile,
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

func resolveClusterFile(clusterFile string) (string, error) {
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", fmt.Errorf("Can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, nil
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
	filePath := c.String(Config)
	clusterFile, err := resolveClusterFile(ConfigFile)
	if err != nil {
		return fmt.Errorf("Failed to resolve default cluster file: %v", err)
	}
	config, err := ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse default cluster file: %v", err)
	}

	if filePath != ConfigFile {
		cusClusterFile, err := resolveClusterFile(filePath)
		if err != nil {
			return fmt.Errorf("Failed to resolve cluster file: %v", err)
		}
		cusConfig, err := ParseConfig(cusClusterFile)
		if err != nil {
			return fmt.Errorf("Failed to parse cluster file: %v", err)
		}

		if cusConfig != nil {
			config = buildConfig(config, cusConfig)
		}
	}

	apiServerListenAddr := c.String(ListenAddress)
	if "" == apiServerListenAddr {
		return fmt.Errorf("RancherCUBE: require %v", ListenAddress)
	}
	if "" == config.KubeConfig {
		return fmt.Errorf("RancherCUBE: require %v", ConfigLocation)
	}

	clientGenerator := backend.NewClientGenerator(config.KubeConfig, &config.InfraImages)

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
	// generate & deploy cusConfig map resources for base info
	clientGenerator.ConfigMapDeploy()
	// generate logo path
	util.CopyDir("assets/images", controller.FrontendPath+"/images")

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

	server := api.NewServer(clientGenerator, config.KubeConfig)
	router := http.Handler(api.NewRouter(server))

	logrus.Infof("RancherCUBE: listening on %s", apiServerListenAddr)

	go http.ListenAndServe(apiServerListenAddr, router)

	go StartPurgeDaemon(clientGenerator, done)

	util.RegisterShutdownChannel(done)
	<-done

	return nil
}

func buildConfig(config *api.CubeConfig, cusConfig *api.CubeConfig) *api.CubeConfig {
	if cusConfig.KubeConfig != "" {
		config.KubeConfig = cusConfig.KubeConfig
	}

	valueGet := reflect.ValueOf(cusConfig.InfraImages)
	valueSet := reflect.ValueOf(&config.InfraImages).Elem()
	for k := 0; k < valueGet.NumField(); k++ {
		if "" != valueGet.Field(k).String() {
			valueSet.Field(k).SetString(valueGet.Field(k).String())
		}
	}
	return config
}
