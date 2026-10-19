package carbon

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// DatasetRevision identifies the pinned upstream coefficient revision.
const DatasetRevision = "f584c549ee358d5980d36513267d007ecd3ee716"

//go:embed data/aws.json
var awsData []byte

type instance struct {
	VCPUs       float64 `json:"vcpus"`
	MemoryGiB   float64 `json:"memory_gib"`
	MinWatts    float64 `json:"min_watts"`
	MaxWatts    float64 `json:"max_watts"`
	ExtraMemory float64 `json:"extra_memory"`
}

type dataset struct {
	Revision  string              `json:"revision"`
	Instances map[string]instance `json:"instances"`
	Regions   map[string]float64  `json:"regions"`
}

func loadData() (dataset, error) {
	var data dataset
	if err := json.Unmarshal(awsData, &data); err != nil {
		return dataset{}, fmt.Errorf("decode carbon model: %w", err)
	}
	return data, nil
}
