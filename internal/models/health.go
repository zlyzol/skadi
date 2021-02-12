package models

// HealthStatus contains health status and metrics of crucial units of Skadi.
type HealthStatus struct {
	Database      bool  `json:"database"`
//	ScannerHeight int64 `json:"scannerHeight"`
}
