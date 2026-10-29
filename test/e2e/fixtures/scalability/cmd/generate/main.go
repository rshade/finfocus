// Command generate writes a deterministic Pulumi preview JSON plan for the
// scalability E2E fixture. The plan has 17 fixed AWS resources, the stack and
// provider steps a real preview carries, and a configurable number of EC2
// instances, two of every five of which get an EBS volume.
//
// Steps follow the shape `pulumi preview --json` emits: type and inputs live
// under newState, and unresolved outputs are the Pulumi unknown sentinel.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	stack           = "dev"
	project         = "scalability"
	unknown         = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	ami             = "ami-0c02fb55956c7d316"
	region          = "us-east-1"
	subnetCount     = 3
	volumeCycle     = 5
	volumesPerCycle = 2
	volumeSizeGiB   = 20
	dbStorageGiB    = 20
	lambdaMemoryMB  = 128
	lambdaTimeoutS  = 3

	opCreate            = "create"
	keyTags             = "tags"
	keyVPCID            = "vpcId"
	keyZone             = "availabilityZone"
	keyRouteTableID     = "routeTableId"
	keySubnetID         = "subnetId"
	keyRole             = "role"
	keySecurityGroupIDs = "vpcSecurityGroupIds"
)

// Plan mirrors the fields of ingest.PulumiPlan that a preview populates.
type Plan struct {
	Steps         []Step         `json:"steps"`
	ChangeSummary map[string]int `json:"changeSummary"`
}

// Step mirrors ingest.PulumiStep for a create operation.
type Step struct {
	Op       string `json:"op"`
	URN      string `json:"urn"`
	Provider string `json:"provider,omitempty"`
	NewState State  `json:"newState"`
}

// State mirrors ingest.PulumiState.
type State struct {
	Type                 string              `json:"type"`
	URN                  string              `json:"urn"`
	Custom               bool                `json:"custom"`
	Parent               string              `json:"parent,omitempty"`
	Provider             string              `json:"provider,omitempty"`
	Inputs               map[string]any      `json:"inputs"`
	Outputs              map[string]any      `json:"outputs"`
	PropertyDependencies map[string][]string `json:"propertyDependencies,omitempty"`
}

type builder struct {
	steps    []Step
	stackURN string
	provider string
}

func urn(typ, name string) string {
	return fmt.Sprintf("urn:pulumi:%s::%s::%s::%s", stack, project, typ, name)
}

func zone(i int) string {
	return fmt.Sprintf("%s%c", region, 'a'+rune(i%subnetCount))
}

func tags(name string) map[string]any {
	return map[string]any{"Name": name, "Environment": "test"}
}

func (b *builder) add(typ, name string, inputs map[string]any, deps map[string][]string) string {
	u := urn(typ, name)
	b.steps = append(b.steps, Step{
		Op:       opCreate,
		URN:      u,
		Provider: b.provider,
		NewState: State{
			Type:                 typ,
			URN:                  u,
			Custom:               true,
			Parent:               b.stackURN,
			Provider:             b.provider,
			Inputs:               inputs,
			Outputs:              map[string]any{},
			PropertyDependencies: deps,
		},
	})
	return u
}

// generatePlan returns the plan for the given number of EC2 instances.
func generatePlan(instanceCount int) Plan {
	stackType := "pulumi:pulumi:Stack"
	providerType := "pulumi:providers:aws"
	b := &builder{stackURN: urn(stackType, project+"-"+stack)}
	providerURN := urn(providerType, "default")

	b.steps = append(b.steps,
		Step{Op: opCreate, URN: b.stackURN, NewState: State{
			Type: stackType, URN: b.stackURN, Inputs: map[string]any{}, Outputs: map[string]any{},
		}},
		Step{Op: opCreate, URN: providerURN, NewState: State{
			Type: providerType, URN: providerURN, Custom: true, Parent: b.stackURN,
			Inputs: map[string]any{"region": region}, Outputs: map[string]any{},
		}},
	)
	b.provider = providerURN + "::" + unknown

	vpc := b.add("aws:ec2/vpc:Vpc", "test-vpc", map[string]any{
		"cidrBlock": "10.0.0.0/16", "enableDnsHostnames": true, keyTags: tags("test-vpc"),
	}, nil)
	onVPC := map[string][]string{keyVPCID: {vpc}}

	subnets := make([]string, subnetCount)
	for i := range subnetCount {
		name := fmt.Sprintf("subnet-%d", i)
		subnets[i] = b.add("aws:ec2/subnet:Subnet", name, map[string]any{
			keyVPCID: unknown, "cidrBlock": fmt.Sprintf("10.0.%d.0/24", i),
			keyZone: zone(i), keyTags: tags(name),
		}, onVPC)
	}

	sg := b.add("aws:ec2/securityGroup:SecurityGroup", "test-sg", map[string]any{
		keyVPCID: unknown, "description": "scalability fixture", keyTags: tags("test-sg"),
	}, onVPC)
	igw := b.add("aws:ec2/internetGateway:InternetGateway", "test-igw", map[string]any{
		keyVPCID: unknown, keyTags: tags("test-igw"),
	}, onVPC)
	rt := b.add("aws:ec2/routeTable:RouteTable", "test-rt", map[string]any{
		keyVPCID: unknown, keyTags: tags("test-rt"),
	}, onVPC)
	b.add("aws:ec2/route:Route", "test-route", map[string]any{
		keyRouteTableID: unknown, "destinationCidrBlock": "0.0.0.0/0", "gatewayId": unknown,
	}, map[string][]string{keyRouteTableID: {rt}, "gatewayId": {igw}})
	for i, subnet := range subnets {
		b.add("aws:ec2/routeTableAssociation:RouteTableAssociation", fmt.Sprintf("rta-%d", i),
			map[string]any{keyRouteTableID: unknown, keySubnetID: unknown},
			map[string][]string{keyRouteTableID: {rt}, keySubnetID: {subnet}})
	}

	role := b.add("aws:iam/role:Role", "test-fn-role", map[string]any{
		"assumeRolePolicy": `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
			`"Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`,
	}, nil)
	b.add("aws:iam/rolePolicyAttachment:RolePolicyAttachment", "test-fn-basic-exec", map[string]any{
		keyRole:     unknown,
		"policyArn": "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
	}, map[string][]string{keyRole: {role}})
	b.add("aws:lambda/function:Function", "test-fn", map[string]any{
		"runtime": "nodejs20.x", "handler": "index.handler", "memorySize": lambdaMemoryMB,
		"timeout": lambdaTimeoutS, keyRole: unknown, keyTags: tags("test-fn"),
	}, map[string][]string{keyRole: {role}})
	b.add("aws:s3/bucket:Bucket", "test-bucket", map[string]any{
		"forceDestroy": true, keyTags: tags("test-bucket"),
	}, nil)
	dbSubnets := b.add("aws:rds/subnetGroup:SubnetGroup", "test-db-subnets", map[string]any{
		"subnetIds": []any{unknown, unknown, unknown}, keyTags: tags("test-db-subnets"),
	}, map[string][]string{"subnetIds": subnets})
	b.add("aws:rds/instance:Instance", "test-db", map[string]any{
		"engine": "postgres", "engineVersion": "16", "instanceClass": "db.t3.micro",
		"allocatedStorage": dbStorageGiB, "storageType": "gp3", "username": "scalability",
		"manageMasterUserPassword": true, "skipFinalSnapshot": true,
		"dbSubnetGroupName": unknown, keySecurityGroupIDs: []any{unknown},
		keyTags: tags("test-db"),
	}, map[string][]string{"dbSubnetGroupName": {dbSubnets}, keySecurityGroupIDs: {sg}})

	for i := range instanceCount {
		name := fmt.Sprintf("instance-%d", i)
		b.add("aws:ec2/instance:Instance", name, map[string]any{
			"ami": ami, "instanceType": "t3.micro", keyZone: zone(i),
			keySubnetID: unknown, keySecurityGroupIDs: []any{unknown}, keyTags: tags(name),
		}, map[string][]string{keySubnetID: {subnets[i%subnetCount]}, keySecurityGroupIDs: {sg}})

		if i%volumeCycle < volumesPerCycle {
			volume := fmt.Sprintf("volume-%d", i)
			b.add("aws:ebs/volume:Volume", volume, map[string]any{
				keyZone: zone(i), "size": volumeSizeGiB, "type": "gp3", keyTags: tags(volume),
			}, nil)
		}
	}

	return Plan{Steps: b.steps, ChangeSummary: map[string]int{opCreate: len(b.steps)}}
}

// write encodes the plan for instanceCount as indented JSON.
func write(w io.Writer, instanceCount int) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(generatePlan(instanceCount))
}

func run() error {
	count := flag.Int("count", 10, "number of EC2 instances")
	output := flag.String("output", "", "output file (default: stdout)")
	flag.Parse()

	if *count < 0 {
		return fmt.Errorf("-count must not be negative, got %d", *count)
	}
	if *output == "" {
		return write(os.Stdout, *count)
	}

	f, err := os.Create(*output)
	if err != nil {
		return fmt.Errorf("creating %s: %w", *output, err)
	}
	if writeErr := write(f, *count); writeErr != nil {
		_ = f.Close()
		return fmt.Errorf("writing %s: %w", *output, writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("closing %s: %w", *output, closeErr)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}
