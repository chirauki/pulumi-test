package eks

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

const (
	eksNodeGroupAssumeRolePolicy = `{
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
)

func (e *Eks) CreateNodeGroups(clusterName string, cluster *eks.Cluster) error {
	// Create Iam Role for node group
	role, err := e.createNodeGroupIamRole(clusterName)
	if err != nil {
		return err
	}

	// Create all node groups
	for _, group := range e.config.Eks.NodeGroups {
		subnetID := e.vpc.PrivateSubnetsMap[group.Az]
		e.config.Ctx.Log.Info(fmt.Sprintf("Got Subnet ID: %v", subnetID), nil)
		g, err := eks.NewNodeGroup(e.config.Ctx, fmt.Sprintf("%s-node-group-%s", clusterName, group.Name), &eks.NodeGroupArgs{
			ClusterName:   cluster.Name,
			SubnetIds:     pulumi.StringArray(pulumi.StringArray{subnetID}),
			InstanceTypes: pulumi.StringPtr(strings.Join(group.InstanceTypes, ",")),
			NodeRoleArn:   role.Arn,
			ScalingConfig: eks.NodeGroupScalingConfigArgs{
				DesiredSize: pulumi.Int(group.Size.Desired),
				MinSize:     pulumi.Int(group.Size.Min),
				MaxSize:     pulumi.Int(group.Size.Max),
			},
		}, pulumi.Parent(cluster))
		if err != nil {
			return err
		}
		e.config.Ctx.Export(clusterName+"-node-group-"+group.Name, g.ID())
	}

	return nil
}

func (e *Eks) createNodeGroupIamRole(clusterName string) (*iam.Role, error) {
	var role *iam.Role

	role, err := iam.NewRole(e.config.Ctx, e.config.Eks.NodeGroupRolePrefix+"-"+clusterName, &iam.RoleArgs{
		Name:             pulumi.String(e.config.Eks.NodeGroupRolePrefix + "-" + clusterName),
		AssumeRolePolicy: pulumi.String(eksNodeGroupAssumeRolePolicy),
	})
	if err != nil {
		return role, err
	}

	var roleNames []pulumi.Input
	roleNames = append(roleNames, role.Name.ToStringOutput())

	policies := []string{"AmazonEKSWorkerNodePolicy", "AmazonEKS_CNI_Policy", "AmazonEC2ContainerRegistryReadOnly"}
	for _, policy := range policies {
		_, err = iam.NewPolicyAttachment(e.config.Ctx, clusterName+"-eks-node-group-iam-role-"+policy, &iam.PolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/" + policy),
			Roles:     pulumi.Array(roleNames),
		}, pulumi.Parent(role))
		if err != nil {
			return role, err
		}
	}
	return role, nil
}

func (e *Eks) buildSubnetMap(subnets []*ec2.Subnet) map[pulumi.StringOutput]pulumi.IDOutput {
	subnetMap := make(map[pulumi.StringOutput]pulumi.IDOutput)
	for _, subnet := range subnets {
		subnetMap[subnet.AvailabilityZone] = subnet.ID()
	}
	return subnetMap
}

func (e *Eks) privateSubnetInAz(az string) (*ec2.LookupSubnetResult, error) {
	result, err := ec2.LookupSubnet(e.config.Ctx, &ec2.LookupSubnetArgs{
		AvailabilityZone: &az,
		Filters: []ec2.GetSubnetFilter{
			ec2.GetSubnetFilter{
				Name:   "tag:kubernetes.io/role/internal-lb",
				Values: []string{"1"},
			},
		},
	})
	if err != nil {
		return result, err
	}
	return result, nil
}
