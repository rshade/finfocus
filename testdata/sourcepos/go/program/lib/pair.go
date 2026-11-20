package lib

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Pair struct {
	pulumi.ResourceState
}

func NewPair(ctx *pulumi.Context, name string) (*Pair, error) {
	p := &Pair{}
	if err := ctx.RegisterComponentResource("shop:index:Pair", name, p); err != nil {
		return nil, err
	}
	for _, side := range []string{"left", "right"} {
		if _, err := local.NewCommand(ctx, name+"-"+side, &local.CommandArgs{Create: pulumi.String("echo " + side)}, pulumi.Parent(p)); err != nil {
			return nil, err
		}
	}
	return p, nil
}
