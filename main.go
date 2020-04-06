package main

import (
	"github/chirauki/pulumi-test/vpc"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		vpc := vpc.NewVpc(ctx)
		if err := vpc.CreateVpc(); err != nil {
			return err
		}
		if err := vpc.CreateSubnets(); err != nil {
			return err
		}
		return nil
	})
}
