package vpc

import (
	"github/chirauki/pulumi-test/config"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

type Vpc struct {
	config            *config.EnvironmentConfig
	Vpc               *ec2.Vpc
	PublicSubnets     []*ec2.Subnet
	PrivateSubnets    []*ec2.Subnet
	PrivateSubnetsMap map[string]pulumi.IDOutput
}

func NewVpc(cfg *config.EnvironmentConfig) *Vpc {
	return &Vpc{
		config: cfg,
	}
}

func (v *Vpc) CreateVpc() error {
	cidr := v.config.Vpc.Cidr
	name := v.config.Vpc.Name

	vpcTags := pulumi.Map{
		"Name":           pulumi.String(name),
		"pulumi-stack":   pulumi.String(v.config.Ctx.Stack()),
		"pulumi-project": pulumi.String(v.config.Ctx.Project()),
	}
	v.config.AddEksSharedTags(vpcTags)

	vpc, err := ec2.NewVpc(v.config.Ctx, v.config.Ctx.Stack()+"-vpc-"+name, &ec2.VpcArgs{
		EnableDnsSupport:   pulumi.BoolPtr(true),
		EnableDnsHostnames: pulumi.BoolPtr(true),
		CidrBlock:          pulumi.String(cidr),
		Tags:               vpcTags,
	})
	if err != nil {
		return err
	}

	v.Vpc = vpc
	v.config.Ctx.Export("vpc-id", vpc.ID())

	return nil
}
