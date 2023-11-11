package types

import "github.com/bacalhau-project/bacalhau/pkg/model"

type ComputeConfig struct {
	Capacity       CapacityConfig           `yaml:"Capacity"`
	ExecutionStore JobStoreConfig           `yaml:"ExecutionStore"`
	JobTimeouts    JobTimeoutConfig         `yaml:"JobTimeouts"`
	JobSelection   model.JobSelectionPolicy `yaml:"JobSelection"`
	Queue          QueueConfig              `yaml:"Queue"`
	Logging        LoggingConfig            `yaml:"Logging"`
	Executors      ExecutorPluginConfig     `yaml:"Executors"`
	Storages       StoragePluginConfig      `yaml:"Storages"`
	Publishers     PublisherPluginConfig    `yaml:"Publishers"`
}

type CapacityConfig struct {
	IgnorePhysicalResourceLimits bool `yaml:"IgnorePhysicalResourceLimits"`
	// Total amount of resource the system can be using at one time in aggregate for all jobs.
	TotalResourceLimits model.ResourceUsageConfig `yaml:"TotalResourceLimits"`
	// Per job amount of resource the system can be using at one time.
	JobResourceLimits        model.ResourceUsageConfig `yaml:"JobResourceLimits"`
	DefaultJobResourceLimits model.ResourceUsageConfig `yaml:"DefaultJobResourceLimits"`
	QueueResourceLimits      model.ResourceUsageConfig `yaml:"QueueResourceLimits"`
}

type JobTimeoutConfig struct {
	// JobExecutionTimeoutClientIDBypassList is the list of clients that are allowed to bypass the job execution timeout
	// check.
	JobExecutionTimeoutClientIDBypassList []string `yaml:"JobExecutionTimeoutClientIDBypassList"`
	// JobNegotiationTimeout default timeout value to hold a bid for a job
	JobNegotiationTimeout Duration `yaml:"JobNegotiationTimeout"`
	// MinJobExecutionTimeout default value for the minimum execution timeout this compute node supports. Jobs with
	// lower timeout requirements will not be bid on.
	MinJobExecutionTimeout Duration `yaml:"MinJobExecutionTimeout"`
	// MaxJobExecutionTimeout default value for the maximum execution timeout this compute node supports. Jobs with
	// higher timeout requirements will not be bid on.
	MaxJobExecutionTimeout Duration `yaml:"MaxJobExecutionTimeout"`
	// DefaultJobExecutionTimeout default value for the execution timeout this compute node will assign to jobs with
	// no timeout requirement defined.
	DefaultJobExecutionTimeout Duration `yaml:"DefaultJobExecutionTimeout"`
}

type QueueConfig struct {
}

type LoggingConfig struct {
	// logging running executions
	LogRunningExecutionsInterval Duration `yaml:"LogRunningExecutionsInterval"`
}

type ExecutorPluginConfig struct {
	Plugins []PluginConfig `yaml:"Plugins"`
}

type PublisherPluginConfig struct {
	Plugins []PluginConfig `yaml:"Plugins"`
}

type StoragePluginConfig struct {
	Plugins []PluginConfig `yaml:"Plugins"`
}

type PluginConfig struct {
	Name             string `yaml:"Name"`
	Path             string `yaml:"Path"`
	Command          string `yaml:"Command"`
	ProtocolVersion  uint   `yaml:"ProtocolVersion"`
	MagicCookieKey   string `yaml:"MagicCookieKey"`
	MagicCookieValue string `yaml:"MagicCookieValue"`
}
