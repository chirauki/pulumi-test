package eks

import (
	"github/chirauki/pulumi-test/config"
	"github/chirauki/pulumi-test/vpc"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

const (
	eksRoleAssumeRolePolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "eks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

	kubeConfigTemplate = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %v
    server: %v
  name: %v
contexts:
- context:
    cluster: %v
    user: %v
  name: %v
current-context: %v
kind: Config
preferences: {}
users:
- name: %v
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      args:
      - --region
      - %v
      - eks
      - get-token
      - --cluster-name
      - %v
      command: aws
`
)

type Eks struct {
	config *config.EnvironmentConfig
	vpc    *vpc.Vpc
}

func NewEks(cfg *config.EnvironmentConfig, vpc *vpc.Vpc) *Eks {
	return &Eks{
		config: cfg,
		vpc:    vpc,
	}
}

func (e *Eks) CreateClusters() error {
	// Create the required number of subnets
	var subnets []pulumi.StringInput
	for _, subnet := range e.vpc.PrivateSubnets {
		subnets = append(subnets, subnet.ID())
	}

	args := &eks.ClusterVpcConfigArgs{
		SubnetIds: pulumi.StringArray(subnets),
	}

	clusterNames := e.config.GetEksClusterNames()
	for _, name := range clusterNames {
		role, err := e.createClusterIamRole(name)
		if err != nil {
			return nil
		}

		cluster, err := eks.NewCluster(e.config.Ctx, e.config.Ctx.Stack()+"-eks-"+name, &eks.ClusterArgs{
			Name:      pulumi.String(name),
			RoleArn:   role.Arn,
			VpcConfig: args,
		}, pulumi.Parent(e.vpc.Vpc))
		if err != nil {
			return err
		}

		err = e.CreateNodeGroups(name, cluster)
		if err != nil {
			return err
		}

		kubeCfg := pulumi.Sprintf(kubeConfigTemplate, cluster.CertificateAuthority.Data(), cluster.Endpoint,
			cluster.Arn, cluster.Arn, cluster.Arn, cluster.Arn, cluster.Arn, cluster.Arn, pulumi.String(e.config.GetConfig("aws", "region")), cluster.Name)

		e.config.Ctx.Export("eks-"+name, cluster.Name)
		e.config.Ctx.Export("kubeconfig-"+name, kubeCfg)
	}

	return nil
}

func (e *Eks) createClusterIamRole(clusterName string) (*iam.Role, error) {
	var role *iam.Role

	role, err := iam.NewRole(e.config.Ctx, e.config.Eks.RolePrefix+"-"+clusterName, &iam.RoleArgs{
		Name:             pulumi.String(e.config.Eks.RolePrefix + "-" + clusterName),
		AssumeRolePolicy: pulumi.String(eksRoleAssumeRolePolicy),
	})
	if err != nil {
		return role, err
	}

	var roleNames []pulumi.Input
	roleNames = append(roleNames, role.Name.ToStringOutput())

	policies := []string{"AmazonEKSClusterPolicy", "AmazonEKSServicePolicy"}
	for _, policy := range policies {
		_, err = iam.NewPolicyAttachment(e.config.Ctx, clusterName+"-eks-iam-role-"+policy, &iam.PolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/" + policy),
			Roles:     pulumi.Array(roleNames),
		}, pulumi.Parent(role))
		if err != nil {
			return role, err
		}
	}

	kubernetes.N

	return role, nil
}
