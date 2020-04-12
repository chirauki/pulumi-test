package main

import (
	"github/chirauki/pulumi-test/config"
	"github/chirauki/pulumi-test/eks"
	"github/chirauki/pulumi-test/vpc"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.LoadConfig(ctx)

		// VPC and subnets
		v := vpc.NewVpc(cfg)
		if err := v.CreateVpc(); err != nil {
			return err
		}
		if err := v.CreateSubnets(); err != nil {
			return err
		}

		// EKS cluster
		e := eks.NewEks(cfg, v)
		if err := e.CreateClusters(); err != nil {
			return err
		}
		return nil
	})
}
