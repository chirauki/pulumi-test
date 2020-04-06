package vpc

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

func (v *Vpc) listAZs() ([]string, error) {
	var azs []string
	availabilityZones, err := aws.GetAvailabilityZones(v.ctx, &aws.GetAvailabilityZonesArgs{})
	if err != nil {
		return azs, err
	}
	return availabilityZones.Names, nil
}

func (v *Vpc) CreateSubnets() error {
	conf := config.New(v.ctx, "")
	publicSubnetCidrs := conf.Require("publicSubnetCidrs")
	privateSubnetCidrs := conf.Require("privateSubnetCidrs")
	azs, err := v.listAZs()
	if err != nil {
		return err
	}

	// Create public subnets
	var publicSubnets []*ec2.Subnet
	for i, cidr := range strings.Split(publicSubnetCidrs, ",") {
		subnet, err := v.createSubnet(cidr, azs[i])
		if err != nil {
			return err
		}
		publicSubnets = append(publicSubnets, subnet)
	}
	// Create IG and attach to public subnets
	err = v.createInternetGateway(publicSubnets)
	if err != nil {
		return err
	}

	// Create private subnets
	var privateSubnets []*ec2.Subnet
	for i, cidr := range strings.Split(privateSubnetCidrs, ",") {
		subnet, err := v.createSubnet(cidr, azs[i])
		if err != nil {
			return err
		}
		privateSubnets = append(privateSubnets, subnet)
	}
	// Create Nat GW for these private subnets
	err = v.createNatGateways(publicSubnets, privateSubnets)
	if err != nil {
		return err
	}

	return nil
}

func (v *Vpc) createSubnet(cidr, az string) (*ec2.Subnet, error) {
	var subnet *ec2.Subnet
	subnet, err := ec2.NewSubnet(v.ctx, v.ctx.Stack()+"-subnet-"+cidr+"-"+az, &ec2.SubnetArgs{
		AssignIpv6AddressOnCreation: pulumi.BoolPtr(false),
		CidrBlock:                   pulumi.String(cidr),
		VpcId:                       v.vpc.ID(),
		AvailabilityZone:            pulumi.String(az),
	})
	if err != nil {
		return subnet, err
	}

	return subnet, nil
}

func (v *Vpc) createInternetGateway(subnets []*ec2.Subnet) error {
	gw, err := ec2.NewInternetGateway(v.ctx, "VPCIG", &ec2.InternetGatewayArgs{
		VpcId: v.vpc.ID(),
	})
	if err != nil {
		return err
	}

	rt, err := ec2.NewRouteTable(v.ctx, "PbNetRT", &ec2.RouteTableArgs{
		VpcId: v.vpc.ID(),
	})
	if err != nil {
		return err
	}

	ec2.NewRoute(v.ctx, "PubNetDefRoute", &ec2.RouteArgs{
		RouteTableId:         rt.ID(),
		DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
		GatewayId:            gw.ID(),
	})

	for i, subnet := range subnets {
		_, err = ec2.NewRouteTableAssociation(v.ctx, fmt.Sprintf("%d-PubRTAssoc", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     subnet.ID(),
			RouteTableId: rt.ID(),
		})
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
		eip, err := ec2.NewEip(v.ctx, fmt.Sprintf("natGw%dIP", i), &ec2.EipArgs{
			Vpc: pulumi.BoolPtr(true),
		})
		if err != nil {
			return err
		}

		gw, err := ec2.NewNatGateway(v.ctx, fmt.Sprintf("natGw%d", i), &ec2.NatGatewayArgs{
			SubnetId:     publicSubnet.ID(),
			AllocationId: eip.ID(),
		})
		if err != nil {
			return err
		}

		rt, err := ec2.NewRouteTable(v.ctx, fmt.Sprintf("natGw%dRT", i), &ec2.RouteTableArgs{
			VpcId: v.vpc.ID(),
		})
		if err != nil {
			return err
		}

		ec2.NewRoute(v.ctx, fmt.Sprintf("natGw%dDefRoute", i), &ec2.RouteArgs{
			RouteTableId:         rt.ID(),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			GatewayId:            gw.ID(),
		})

		_, err = ec2.NewRouteTableAssociation(v.ctx, fmt.Sprintf("%d-RTAssoc", i), &ec2.RouteTableAssociationArgs{
			SubnetId:     subnet.ID(),
			RouteTableId: rt.ID(),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
