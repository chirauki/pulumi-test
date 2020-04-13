import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as k8s from "@pulumi/kubernetes";

import {
    EnvConfig,
    EnvCfg,
    eksClusterAssumeRolePolicy,
    eksGroupAssumeRolePolicy,
} from "./envconfig"
import { FunctionEventInvokeConfig } from "@pulumi/aws/lambda";

// Config
let config = new pulumi.Config();
let parsedCfg = config.requireObject<EnvConfig>("config");
let cfg = new EnvCfg(parsedCfg);
const awsConfig = new pulumi.Config("aws")


// VPC
var vpcTags: { [k: string]: string } = {};
vpcTags = {
    "Name": cfg.vpc.name,
};
cfg.addSharedTags(vpcTags)

const vpc = new aws.ec2.Vpc(`${pulumi.getStack()}-vpc`, {
    cidrBlock: cfg.vpc.cidr,
    tags: vpcTags,
});

// SUBNETS

// IG
const ig = new aws.ec2.InternetGateway(`${pulumi.getStack()}-vpc-ig`, { vpcId: vpc.id }, { parent: vpc });
const publicRt = new aws.ec2.RouteTable(`${pulumi.getStack()}-vpc-ig-rt`, {
    vpcId: vpc.id,
    routes: [
        {
            cidrBlock: "0.0.0.0/0",
            gatewayId: ig.id,
        }
    ]
}, { parent: ig });
export const vpcId = vpc.id

// PublicSubnets
let publicSubnets: { id: any; }[] = [];
cfg.vpc.subnets.public.forEach((value, index) => {
    let subnetName = `public-${index}`
    var subnetTags: { [k: string]: string } = {};
    subnetTags = {
        "Name": subnetName,
    };
    cfg.addSharedTags(subnetTags)

    const subnet = new aws.ec2.Subnet(`${pulumi.getStack()}-vpc-${subnetName}`, {
        vpcId: vpc.id,
        cidrBlock: value.cidr,
        availabilityZone: value.az,
        tags: subnetTags,
    }, { parent: ig });
    publicSubnets.push(subnet);

    const assoc = new aws.ec2.RouteTableAssociation(`${pulumi.getStack()}-public-${index}-ig`, {
        routeTableId: publicRt.id,
        subnetId: subnet.id,
    }, { parent: publicRt });
});


// PrivateSubnets
let privateSubnets: aws.ec2.Subnet[] = [];
var privateSubnetAz: { [k: string]: any } = {};

cfg.vpc.subnets.private.forEach((value, index) => {
    // NAT GW
    const eip = new aws.ec2.Eip(`${pulumi.getStack()}-vpc-eip-ng-${index}`, { vpc: true }, { parent: vpc })
    const ng = new aws.ec2.NatGateway(`${pulumi.getStack()}-vpc-ng-${index}`, {
        allocationId: eip.id,
        subnetId: publicSubnets[index].id,
    }, { parent: eip });

    // RT
    const rt = new aws.ec2.RouteTable(`${pulumi.getStack()}-private-${index}-rt`, {
        vpcId: vpc.id,
        routes: [{
            cidrBlock: "0.0.0.0/0",
            natGatewayId: ng.id,
        }]
    }, { parent: ng })

    // private Subnet
    let subnetName = `private-${index}`
    var subnetTags: { [k: string]: string } = {};
    subnetTags = {
        "Name": subnetName,
    };
    cfg.addSharedTags(subnetTags)

    const subnet = new aws.ec2.Subnet(`${pulumi.getStack()}-vpc-${subnetName}`, {
        vpcId: vpc.id,
        cidrBlock: value.cidr,
        availabilityZone: value.az,
        tags: subnetTags,
    }, { parent: ng });
    privateSubnets.push(subnet);
    privateSubnetAz[value.az] = subnet.id

    // RT assoc
    const assoc = new aws.ec2.RouteTableAssociation(`${pulumi.getStack()}-private-${index}-rt`, {
        routeTableId: rt.id,
        subnetId: subnet.id,
    }, { parent: rt });
});

// EKS
let clusterNames = cfg.getEksClusterNames();
let clusterSubnets: pulumi.Output<string>[] = [];
privateSubnets.forEach(v => { clusterSubnets.push(v.id); });
let clusters: aws.eks.Cluster[] = [];
let kubeconfigs: pulumi.Output<string>[] = [];

clusterNames.forEach((value, index) => {
    // Iam Role
    const clusterRole = new aws.iam.Role(`${pulumi.getStack()}-eks-${value}-iam-role`, {
        assumeRolePolicy: eksClusterAssumeRolePolicy,
    });

    let clusterPolicies = ["AmazonEKSClusterPolicy", "AmazonEKSServicePolicy"];
    clusterPolicies.forEach(v => {
        const clusterRolePolicyAttachement = new aws.iam.RolePolicyAttachment(`${pulumi.getStack()}-eks-${value}-policy-${v}`, {
            role: clusterRole.name,
            policyArn: `arn:aws:iam::aws:policy/${v}`,
        }, { parent: clusterRole });
    }, { parent: clusterRole });

    // Cluster
    const cluster = new aws.eks.Cluster(`${pulumi.getStack()}-eks-${value}`, {
        name: value,
        roleArn: clusterRole.arn,
        vpcConfig: {
            subnetIds: clusterSubnets,
        },
    });

    // Cluster kubeconfig
    let kCfg = pulumi.all([cluster.name, cluster.endpoint, cluster.certificateAuthority.data]).apply(([name, ep, ca]) => `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${ca}
    server: ${ep}
  name: ${name}
contexts:
- context:
    cluster: ${name}
    user: ${name}
  name: ${name}
current-context: ${name}
kind: Config
preferences: {}
users:
- name: ${name}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      args:
      - --region
      - ${awsConfig.get("region")}
      - eks
      - get-token
      - --cluster-name
      - ${name}
      command: aws
`);
    kubeconfigs.push(kCfg);

    // Node Groups
    // IAM role and req policies
    const groupRole = new aws.iam.Role(`${pulumi.getStack()}-eks-${value}-group-iam-role`, {
        assumeRolePolicy: eksGroupAssumeRolePolicy,
    });

    let groupPolicies = ["AmazonEKSWorkerNodePolicy", "AmazonEKS_CNI_Policy", "AmazonEC2ContainerRegistryReadOnly"];
    groupPolicies.forEach(v => {
        const clusterRolePolicyAttachement = new aws.iam.RolePolicyAttachment(`${pulumi.getStack()}-eks-${value}-group-policy-${v}`, {
            role: groupRole.name,
            policyArn: `arn:aws:iam::aws:policy/${v}`,
        }, { parent: groupRole });
    }, { parent: groupRole });

    // Iam policy for cluster - autoscaler
    let caPolicy = new aws.iam.RolePolicy(`${pulumi.getStack()}-eks-${value}-ca`, {
        role: groupRole.name,
        policy: `{
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
        }`,
    }, { parent: cluster });

    // groups
    cfg.eks.nodeGroups.forEach(group => {
        const nodeGroup = new aws.eks.NodeGroup(`${pulumi.getStack()}-eks-${value}-group-${group.name}`, {
            clusterName: cluster.name,
            nodeRoleArn: groupRole.arn,
            subnetIds: [privateSubnetAz[group.az]],
            instanceTypes: group.instanceTypes.join(","),
            scalingConfig: {
                desiredSize: group.size.desired,
                minSize: group.size.min,
                maxSize: group.size.max,
            },
        }, { parent: cluster });
    });

    // Install things in cluster
    const k8sprovider = new k8s.Provider(`k8s-provider-${value}`, {
        cluster: cluster.name,
        kubeconfig: kCfg,
    }, { parent: cluster });

    // Cluster Autoscaler
    const ca = new k8s.helm.v3.Chart(`${value.toLowerCase()}-ca`, {
        repo: "stable",
        chart: "cluster-autoscaler",
        namespace: "kube-system",
        values: {
            autoDiscovery: {
                enabled: true,
                clusterName: value,
            },
            rbac: {
                create: true,
            },
            awsRegion: awsConfig.get("region")
        }
    }, { provider: k8sprovider, parent: cluster });

    // Metrics server
    const ms = new k8s.yaml.ConfigFile(`${value}-metrics-server`, {
        file: "https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.3.6/components.yaml",
        resourcePrefix: value,
    }, { provider: k8sprovider, parent: cluster });
});

export const kubecfgs = kubeconfigs;
