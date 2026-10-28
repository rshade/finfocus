package pulumicloud_test

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/pulumicloud"
)

func ExampleDecodeIdentities() {
	export := `{"deployment":{"resources":[{
		"urn":"urn:pulumi:dev::demo::aws:ec2/instance:Instance::web",
		"type":"aws:ec2/instance:Instance","custom":true,"id":"i-0abc",
		"inputs":{"password":"never retained"}}]}}`

	identities, err := pulumicloud.DecodeIdentities(strings.NewReader(export))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, identity := range identities {
		fmt.Println(identity.CloudID, identity.Type, identity.Provider)
	}
	// Output: i-0abc aws:ec2/instance:Instance aws
}
