package scheduler

import (
	workerstatus "code.finan.one/finan-one-be/fo-utils/worker/status"
)

var (
	AllowedMaxRetries int = 100
	DefaultRetries    int = 10
)

const ScheduleFailed = 1

const (
	OneTimeMode = "once"
	EndlessMode = "endless"
)

// ISchedulerTask ...
type ISchedulerTask interface {
	// GetName returns the task name
	GetName() string
	// GetNameWithSuffix returns the task name with suffix
	GetNameWithSuffix() string
	// Before the func call when task exec
	Before()
	// Handle the func process task logic
	Handle() workerstatus.ProcessStatus
	// After the func call when task exec done
	After()
}

// ScheduleHandlerConfig ...
type ScheduleHandlerConfig struct {
	Handler ISchedulerTask `json:"handler" mapstructure:"handler"`
	Retries int            `json:"retries" mapstructure:"retries"`
	Spec    string         `json:"spec" mapstructure:"spec"`
	Type    string         `json:"type" mapstructure:"type"` // can be: once ,	endless
}
