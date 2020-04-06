package iam

import (
	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

const (
	defaultEksNodeGroupRoleName = "eks-node-group-role"
	eksNodeGroupPolicy          = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      }
    },
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "es.amazonaws.com"
      }
    }
  ]
}`

	defaultClusterAutoscalerRoleName = "eks-cluster-autoscaler"
	clusterAutoscalerPolicy          = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "autoscaling:DescribeAutoScalingGroups",
                "autoscaling:DescribeAutoScalingInstances",
                "autoscaling:DescribeLaunchConfigurations",
                "autoscaling:DescribeTags",
                "autoscaling:SetDesiredCapacity",
                "autoscaling:TerminateInstanceInAutoScalingGroup"
            ],
            "Resource": "*"
        }
    ]
}`
)

type Iam struct {
	ctx *pulumi.Context
}

func NewIam(ctx *pulumi.Context) *Iam {
	return &Iam{
		ctx: ctx,
	}
}

func (i *Iam) CreateRoles() error {
	err := i.createEksNodeGroupRole()
	if err != nil {
		return err
	}
	err = i.createClusterAutoscalerPolicy()
	if err != nil {
		return err
	}
	return nil
}

func (i *Iam) createEksNodeGroupRole() error {
	roleName, present := i.ctx.GetConfig("eks-node-group-role-name")
	if !present {
		roleName = defaultEksNodeGroupRoleName
	}

	role, err := iam.NewRole(i.ctx, roleName, &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(eksNodeGroupPolicy),
	})
	if err != nil {
		return err
	}

	i.ctx.Export("eks-node-group-role-arn", role.Arn)
	i.ctx.Export("eks-node-group-role-name", role.Name)

	return nil
}

func (i *Iam) createClusterAutoscalerPolicy() error {
	roleName, present := i.ctx.GetConfig("eks-cluster-autoscaler-role-name")
	if !present {
		roleName = defaultClusterAutoscalerRoleName
	}

	role, err := iam.NewRole(i.ctx, roleName, &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(""),
	})
	if err != nil {
		return err
	}

	policy, err := iam.NewPolicy(i.ctx, roleName+"-policy", &iam.PolicyArgs{
		Policy: pulumi.String(clusterAutoscalerPolicy),
	})
	if err != nil {
		return err
	}

	_, err = iam.NewPolicyAttachment(i.ctx, "attach-ca-policy", &iam.PolicyAttachmentArgs{
		Roles:     pulumi.Array{role.Arn},
		PolicyArn: policy.Arn,
	})
	if err != nil {
		return err
	}

	i.ctx.Export("eks-cluster-autoscaler-role-arn", role.Arn)
	i.ctx.Export("eks-cluster-autoscaler-role-name", role.Name)

	return nil
}
