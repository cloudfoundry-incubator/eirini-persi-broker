package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345

	"code.cloudfoundry.org/eirini-persi-broker/broker"
	"code.cloudfoundry.org/eirini-persi-broker/config"
)

func main() {

	brokerConfigPath := configPath()

	brokerLogger := lager.NewLogger("eirini-persi-broker")
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))

	brokerLogger.Info("Starting Eirini Persi Broker broker")

	brokerLogger.Info("Config File: " + brokerConfigPath)

	config, err := config.ParseConfig(brokerConfigPath)
	if err != nil {
		brokerLogger.Fatal("Loading config file", err, lager.Data{
			"broker-config-path": brokerConfigPath,
		})
	}

	// Try to configure the connection to Kubernetes
	configGetter := NewKubeConfigGetter(brokerLogger)
	kubeConfig, err := configGetter.Get(os.Getenv("KUBECONFIG"))
	if err != nil {
		brokerLogger.Fatal("Couldn't configure Kubernetes client", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGTERM)
	go func() {
		<-sigChannel
		brokerLogger.Info("Starting Eirini Persi Broker shutdown")
		os.Exit(0)
	}()

	serviceBroker := &broker.KubeVolumeBroker{
		KubeClient: clientset,
		Config:     config,
		Context:    context.Background(),
	}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: config.AuthConfiguration.Username,
		Password: config.AuthConfiguration.Password,
	}

	brokerAPI := brokerapi.New(serviceBroker, brokerLogger, brokerCredentials)
	//authWrapper := auth.NewWrapper(brokerCredentials.Username, brokerCredentials.Password)

	http.Handle("/", brokerAPI)

	brokerLogger.Fatal("http-listen", http.ListenAndServe(config.Host+":"+config.Port, nil))
}

func configPath() string {
	brokerConfigYamlPath := os.Getenv("BROKER_CONFIG_PATH")
	if brokerConfigYamlPath == "" {
		panic("BROKER_CONFIG_PATH not set")
	}
	return brokerConfigYamlPath
}
