package main

import (
	"srcposgo/lib"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// moved down by two lines
		// and resized
		web, err := local.NewCommand(ctx, "web", &local.CommandArgs{Create: pulumi.String("echo web-v2")})
		if err != nil {
			return err
		}
		if _, err := lib.NewPair(ctx, "edge"); err != nil {
			return err
		}
		if _, err := local.NewCommand(ctx, "extra", &local.CommandArgs{Create: pulumi.String("echo extra")}); err != nil {
			return err
		}
		ctx.Export("web", web.Stdout)
		return nil
	})
}
