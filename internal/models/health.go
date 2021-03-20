package models

// HealthStatus contains health status and metrics of crucial units of Skadi.
type HealthStatus struct {
	Bot			string  `json:"bot"`
	Database	string  `json:"database"`
//	ScannerHeight int64 `json:"scannerHeight"`
}
