package history

import "time"

// ResourcePoint is one snapshot's monthly cost for a single resource URN.
type ResourcePoint struct {
	Timestamp time.Time
	Version   int
	Monthly   float64
	Currency  string
}

// ResourcePoints returns the monthly series for resourceID.
// resourceID matches CostResource.URN. Snapshots that omit the resource are
// left out. Several resources with the same URN in one snapshot are summed.
// The series is ordered by snapshot time, then version.
func ResourcePoints(snapshots []CostSnapshot, resourceID string) []ResourcePoint {
	if resourceID == "" {
		return nil
	}
	var out []ResourcePoint
	for _, snapshot := range orderedSnapshots(snapshots) {
		var amount float64
		found := false
		for _, resource := range snapshot.Resources {
			if resource.URN != resourceID {
				continue
			}
			amount += resource.MonthlyCost
			found = true
		}
		if !found {
			continue
		}
		out = append(out, ResourcePoint{
			Timestamp: snapshot.Timestamp,
			Version:   snapshot.Version,
			Monthly:   amount,
			Currency:  snapshot.Currency,
		})
	}
	return out
}
