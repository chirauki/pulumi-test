package vpc

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-aws/sdk/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

func (v *Vpc) listAZs() ([]string, error) {
	var azs []string
	availabilityZones, err := aws.GetAvailabilityZones(v.config.Ctx, &aws.GetAvailabilityZonesArgs{})
	if err != nil {
		return azs, err
	}
	return availabilityZones.Names, nil
}

func (v *Vpc) CreateSubnets() error {
	publicSubnetConfig := v.config.Vpc.Subnets.Public
	privateSubnetConfig := v.config.Vpc.Subnets.Private

	// Create public subnets
	var publicSubnets []*ec2.Subnet
	for i, subnetConfig := range publicSubnetConfig {
		subnetName := fmt.Sprintf("public%d", i)
		subnetTags := map[string]pulumi.Input{
			"Name":                   pulumi.String(subnetName),
			"kubernetes.io/role/elb": pulumi.String(strconv.Itoa(1)),
		}
		v.config.AddEksSharedTags(subnetTags)

		subnet, err := v.createSubnet(subnetConfig.Cidr, subnetConfig.Az, subnetName, subnetTags)
		if err != nil {
			return err
		}
		publicSubnets = append(publicSubnets, subnet)
	}
	// Create IG and attach to public subnets
	err := v.createInternetGateway(publicSubnets)
	if err != nil {
		return err
	}

	// Create private subnets
	var privateSubnets []*ec2.Subnet
	privateSubnetsMap := make(map[string]pulumi.IDOutput)
	for i, subnetConfig := range privateSubnetConfig {
		subnetName := fmt.Sprintf("private%d", i)
		subnetTags := map[string]pulumi.Input{
			"Name":                           pulumi.String(subnetName),
			"kubernetes.io/role/internal-lb": pulumi.String(strconv.Itoa(1)),
		}
		v.config.AddEksSharedTags(subnetTags)

		subnet, err := v.createSubnet(subnetConfig.Cidr, subnetConfig.Az, subnetName, subnetTags)
		if err != nil {
			return err
		}
		privateSubnets = append(privateSubnets, subnet)
		privateSubnetsMap[subnetConfig.Az] = subnet.ID()
	}
	// Create Nat GW for these private subnets
	err = v.createNatGateways(publicSubnets, privateSubnets)
	if err != nil {
		return err
	}

	v.PublicSubnets = publicSubnets
	v.PrivateSubnets = privateSubnets
	v.PrivateSubnetsMap = privateSubnetsMap

	return nil
}

func (v *Vpc) createSubnet(cidr, az, name string, tags map[string]pulumi.Input) (*ec2.Subnet, error) {
	var subnet *ec2.Subnet

	subnet, err := ec2.NewSubnet(v.config.Ctx, v.config.Ctx.Stack()+"-subnet-"+name, &ec2.SubnetArgs{
		AssignIpv6AddressOnCreation: pulumi.BoolPtr(false),
		CidrBlock:                   pulumi.String(cidr),
		VpcId:                       v.Vpc.ID(),
		AvailabilityZone:            pulumi.String(az),
		Tags:                        pulumi.Map(tags),
	}, pulumi.Parent(v.Vpc))
	if err != nil {
		return subnet, err
	}

	return subnet, nil
}

func (v *Vpc) createInternetGateway(subnets []*ec2.Subnet) error {
	gw, err := ec2.NewInternetGateway(v.config.Ctx, "vpc-"+v.config.Vpc.Name+"-igw", &ec2.InternetGatewayArgs{
		VpcId: v.Vpc.ID(),
	}, pulumi.Parent(v.Vpc))
	if err != nil {
		return err
	}

	rt, err := ec2.NewRouteTable(v.config.Ctx, "vpc-"+v.config.Vpc.Name+"-publicRT", &ec2.RouteTableArgs{
		VpcId: v.Vpc.ID(),
	}, pulumi.Parent(v.Vpc))
	if err != nil {
		return err
	}

	ec2.NewRoute(v.config.Ctx, "vpc-"+v.config.Vpc.Name+"-publicRT-defRoute", &ec2.RouteArgs{
		RouteTableId:         rt.ID(),
		DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
		GatewayId:            gw.ID(),
	}, pulumi.Parent(rt))

	for i, subnet := range subnets {
		_, err = ec2.NewRouteTableAssociation(v.config.Ctx, fmt.Sprintf("vpc-"+v.config.Vpc.Name+"-publicRT-defRoute-%d", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     subnet.ID(),
			RouteTableId: rt.ID(),
		}, pulumi.Parent(rt))
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Vpc) createNatGateways(publicSubnets, privateSubnets []*ec2.Subnet) error {
	for i, subnet := range privateSubnets {
		publicSubnet := publicSubnets[i]

		// Elastic IP for NatGW
		eip, err := ec2.NewEip(v.config.Ctx, fmt.Sprintf("natGw%dIP", i), &ec2.EipArgs{
			Vpc: pulumi.BoolPtr(true),
		}, pulumi.Parent(v.Vpc))
		if err != nil {
			return err
		}

		gw, err := ec2.NewNatGateway(v.config.Ctx, fmt.Sprintf("natGw%d", i), &ec2.NatGatewayArgs{
			SubnetId:     publicSubnet.ID(),
			AllocationId: eip.ID(),
		}, pulumi.Parent(eip))
		if err != nil {
			return err
		}

		rt, err := ec2.NewRouteTable(v.config.Ctx, fmt.Sprintf("natGw%dRT", i), &ec2.RouteTableArgs{
			VpcId: v.Vpc.ID(),
		}, pulumi.Parent(gw))
		if err != nil {
			return err
		}

		ec2.NewRoute(v.config.Ctx, fmt.Sprintf("natGw%dDefRoute", i), &ec2.RouteArgs{
			RouteTableId:         rt.ID(),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			NatGatewayId:         gw.ID(),
		}, pulumi.Parent(rt))

		_, err = ec2.NewRouteTableAssociation(v.config.Ctx, fmt.Sprintf("%d-RTAssoc", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     subnet.ID(),
			RouteTableId: rt.ID(),
		}, pulumi.Parent(rt))
		if err != nil {
			return err
		}
	}

	return nil
}
