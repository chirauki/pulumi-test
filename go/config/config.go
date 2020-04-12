package config

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

// Subnets
type Subnets struct {
	Private []Private `yaml:"private"`
	Public  []Public  `yaml:"public"`
}

// Private subnets
type Private struct {
	Az   string `yaml:"az"`
	Cidr string `yaml:"cidr"`
}

// Public subnets
type Public struct {
	Az   string `yaml:"az"`
	Cidr string `yaml:"cidr"`
}

// Eks clusters config
type Eks struct {
	Count               int          `yaml:"count"`
	NamePrefix          string       `yaml:"namePrefix"`
	RolePrefix          string       `yaml:"rolePrefix"`
	NodeGroupRolePrefix string       `yaml:"nodeGroupRolePrefix"`
	NodeGroups          []NodeGroups `yaml:"nodeGroups"`
}

// Eks NodeGroups
type NodeGroups struct {
	InstanceTypes []string `yaml:"instanceTypes"`
	Az            string   `yaml:"az"`
	Size          Size     `yaml:"size"`
	Name          string   `yaml:"name"`
}

// Eks NodeGroup Sizing
type Size struct {
	Desired int `yaml:"desired"`
	Min     int `yaml:"min"`
	Max     int `yaml:"max"`
}

// Vpc config
type Vpc struct {
	Subnets Subnets `yaml:"subnets"`
	Name    string  `yaml:"name"`
	Cidr    string  `yaml:"cidr"`
}

// Env Config
type EnvironmentConfig struct {
	Ctx *pulumi.Context
	Vpc Vpc `yaml:"vpc"`
	Eks Eks `yaml:"eks"`
}

func (e *EnvironmentConfig) AddEksSharedTags(clusterNameTags map[string]pulumi.Input) {
	for _, name := range e.GetEksClusterNames() {
		clusterNameTags[fmt.Sprintf("kubernetes.io/cluster/%s", name)] = pulumi.String("shared")
	}
}

func (e *EnvironmentConfig) GetEksClusterNames() []string {
	var clusterNames []string
	for i := 0; i < e.Eks.Count; i++ {
		clusterNames = append(clusterNames, fmt.Sprintf("%s-%d", e.Eks.NamePrefix, i))
	}
	return clusterNames
}

func (e *EnvironmentConfig) GetConfig(ns, key string) string {
	conf := config.New(e.Ctx, ns)
	return conf.Require(key)
}

func LoadConfig(ctx *pulumi.Context) *EnvironmentConfig {
	var envConfig EnvironmentConfig
	conf := config.New(ctx, "")
	conf.RequireObject("config", &envConfig)
	envConfig.Ctx = ctx
	return &envConfig
}
