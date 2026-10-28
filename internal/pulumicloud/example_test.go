package pulumicloud_test

import (
	"context"
	"fmt"
	"os"

	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

func ExampleNewClient() {
	session, fail := esc.DiscoverSession(os.Getenv, "", "dev")
	if fail != nil {
		return
	}
	client := pulumicloud.NewClient(session, nil)
	stack := pulumicloud.StackRef{Org: session.Org, Project: "demo", Stack: "dev"}

	updates, err := client.ListUpdates(context.Background(), stack, "")
	if err != nil {
		return
	}
	for _, update := range updates.Updates {
		identities, err := client.ExportIdentities(context.Background(), stack, update.Version)
		if err != nil {
			return
		}
		fmt.Println(update.Version, len(identities))
	}
}
