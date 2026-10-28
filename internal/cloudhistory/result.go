package cloudhistory

// Result summarizes the observations imported from an update list.
type Result struct {
	Listed     int    `json:"listed"`
	Skipped    int    `json:"skipped"`
	FromCache  int    `json:"fromCache"`
	Fetched    int    `json:"fetched"`
	Recorded   int    `json:"recorded"`
	Unreadable int    `json:"unreadable"`
	Clamped    bool   `json:"clamped"`
	Reason     string `json:"-"`
}

// Incomplete reports whether any selected update could not be persisted.
func (r Result) Incomplete() bool { return r.Unreadable > 0 }
