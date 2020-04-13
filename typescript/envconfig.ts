import { rootPulumiStackTypeName } from "@pulumi/pulumi/runtime"

type Size = {
    desired: number;
    max: number;
    min: number;
}

type NodeGroup = {
    az: string;
    instanceTypes: string[];
    name: string;
    size: Size;
}

type Eks = {
    count: number;
    namePrefix: string;
    rolePrefix: string;
    nodeGroupRolePrefix: string;
    nodeGroups: NodeGroup[];
}

type Private = {
    az: string;
    cidr: string;
}

type Public = {
    az: string;
    cidr: string;
}

type Subnets = {
    private: Private[];
    public: Public[];
}

type Vpc = {
    cidr: string;
    name: string;
    subnets: Subnets;
}

export type EnvConfig = {
    eks: Eks;
    vpc: Vpc;
}

export class EnvCfg {
    eks: Eks;
    vpc: Vpc;

    constructor(c: EnvConfig) {
        this.eks = c.eks;
        this.vpc = c.vpc;
    }

    getEksClusterNames(): string[] {
        let names: string[] = [];
        for (var _i = 0; _i < this.eks.count; _i++) {
            names.push(`${this.eks.namePrefix}-${_i}`);
        }
        return names;
    }

    addSharedTags(tags: { [k: string]: string }): any {
        this.getEksClusterNames().forEach(
            function (value, array) {
                tags[`kubernetes.io/cluster/${value}`] = "shared";
            }
        );
    }
}

export const eksClusterAssumeRolePolicy = `{
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

export const eksGroupAssumeRolePolicy = `{
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
