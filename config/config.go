package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for the entire server
type Config struct {
	ServiceConfiguration ServiceConfiguration `yaml:"service"`
	AuthConfiguration    AuthConfiguration    `yaml:"auth"`
	Host                 string               `yaml:"backend_host"`
	Port                 string               `yaml:"backend_port"`
	Namespace            string               `yaml:"namespace"`
}

// AuthConfiguration contains credentials for authenticating with the broker
type AuthConfiguration struct {
	Password string `yaml:"password"`
	Username string `yaml:"username"`
}

// ServiceConfiguration represents the configuration for the Eirini Kubernetes Volume Broker
type ServiceConfiguration struct {
	ServiceName string `yaml:"service_name"`
	ServiceID   string `yaml:"service_id"`

	Plans []Plan `yaml:"plans"`

	Description         string `yaml:"description"`
	LongDescription     string `yaml:"long_description"`
	ProviderDisplayName string `yaml:"provider_display_name"`
	DocumentationURL    string `yaml:"documentation_url"`
	SupportURL          string `yaml:"support_url"`
	DisplayName         string `yaml:"display_name"`
	IconImage           string `yaml:"icon_image"`
}

// Plan represents a Broker plan for a Kubernetes storage class
type Plan struct {
	ID           string `yaml:"plan_id"`
	Name         string `yaml:"plan_name"`
	Description  string `yaml:"description"`
	StorageClass string `yaml:"kube_storage_class"`
	Free         bool   `yaml:"free"`
}

// ParseConfig parses a config file
func ParseConfig(path string) (Config, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}
