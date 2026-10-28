package engine

// HistoryImportStatus marks cost reports whose Cloud identity import was incomplete.
type HistoryImportStatus struct {
	Incomplete        bool `json:"incomplete"`
	UnreadableUpdates int  `json:"unreadableUpdates"`
}
