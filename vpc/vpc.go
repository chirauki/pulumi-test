package vpc

import (
	"github.com/pulumi/pulumi-aws/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

type Vpc struct {
	ctx *pulumi.Context
	vpc *ec2.Vpc
}

func NewVpc(ctx *pulumi.Context) *Vpc {
	return &Vpc{
		ctx: ctx,
	}
}

func (v *Vpc) CreateVpc() error {
	conf := config.New(v.ctx, "")
	cidr := conf.Require("vpcCidr")
	name := conf.Require("vpcName")

	vpc, err := ec2.NewVpc(v.ctx, v.ctx.Stack()+"-vpc", &ec2.VpcArgs{
		EnableDnsSupport:   pulumi.BoolPtr(true),
		EnableDnsHostnames: pulumi.BoolPtr(true),
		CidrBlock:          pulumi.String(cidr),
		Tags: pulumi.Map{
			"Name":           pulumi.String(name),
			"pulumi-stack":   pulumi.String(v.ctx.Stack()),
			"pulumi-project": pulumi.String(v.ctx.Project()),
		},
	})
	if err != nil {
		return err
	}

	v.vpc = vpc
	v.ctx.Export("vpc-id", vpc.ID())

	return nil
}
