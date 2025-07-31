package config

import "github.com/rocket-pool/node-manager-core/log"

// NMC servers typically provide some kind of persistent configuration; it must implement this interface.
type IConfig interface {
	IConfigSection

	// The path to use for the API log file
	GetApiLogFilePath() string

	// The path to use for the tasks log file
	GetTasksLogFilePath() string

	// The path to use for the node address file
	GetNodeAddressFilePath() string

	// The path to use for the wallet keystore file
	GetWalletFilePath() string

	// The path to use for the wallet keystore's password file
	GetPasswordFilePath() string

	// The URLs for the Execution clients to use - the primary, and a collection of fallbacks if applicable
	GetExecutionClientUrls() (string, []string)

	// The URLs for the Beacon nodes to use - the primary, and a collection of fallbacks if applicable
	GetBeaconNodeUrls() (string, []string)

	// The configuration for the daemon loggers
	GetLoggerOptions() log.LoggerOptions
}
