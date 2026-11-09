package engine

import (
	"encoding/json"
	"maps"

	"github.com/rshade/finfocus/internal/proto"
)

// MarshalJSON omits the provider region core adds to Properties and
// OldProperties. It is a pricing input, not a property the resource declares,
// so user-facing JSON does not show it.
func (r ResourceDescriptor) MarshalJSON() ([]byte, error) {
	type plain ResourceDescriptor
	out := plain(r)
	out.Properties = withoutProviderRegion(r.Properties)
	out.OldProperties = withoutProviderRegion(r.OldProperties)
	return json.Marshal(out)
}

// withoutProviderRegion returns props without proto.ProviderRegionProperty.
// It returns props itself when the key is absent and never modifies props.
func withoutProviderRegion(props map[string]any) map[string]any {
	if _, ok := props[proto.ProviderRegionProperty]; !ok {
		return props
	}
	out := maps.Clone(props)
	delete(out, proto.ProviderRegionProperty)
	return out
}

// withoutProviderRegionRow returns row with the provider region removed from
// both property maps, for JSON and NDJSON output. The row's maps stay as they were.
func withoutProviderRegionRow(row OverviewRow) OverviewRow {
	row.Properties = withoutProviderRegion(row.Properties)
	row.ProjectedProperties = withoutProviderRegion(row.ProjectedProperties)
	return row
}
