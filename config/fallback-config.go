package config

import "github.com/rocket-pool/node-manager-core/config/ids"

// Fallback configuration
type FallbackConfig struct {
	// Flag for enabling fallback clients
	UseFallbackClients Parameter[bool]

	// The URLs of the Execution Client HTTP endpoint
	EcHttpUrls Parameter[string]

	// The URLs of the Beacon Node HTTP endpoint
	BnHttpUrls Parameter[string]

	// The URLs of the Prysm gRPC endpoint (only needed if using Prysm VCs)
	PrysmRpcUrls Parameter[string]
}

// Generates a new FallbackConfig configuration
func NewFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		UseFallbackClients: Parameter[bool]{
			ParameterCommon: &ParameterCommon{
				ID:                 ids.FallbackUseFallbackClientsID,
				Name:               "Use Fallback Clients",
				Description:        "Enable this if you would like to specify one or more fallback Execution and Beacon Node pairs, which will temporarily be used by your node and Validator Client(s) if your primary Execution / Beacon Node pair ever go offline (e.g. if you switch, prune, or resync your clients).",
				AffectsContainers:  []ContainerID{ContainerID_Daemon, ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[Network]bool{
				Network_All: false,
			},
		},

		EcHttpUrls: Parameter[string]{
			ParameterCommon: &ParameterCommon{
				ID:                 ids.FallbackEcHttpUrlID,
				Name:               "Execution Client URLs",
				Description:        "A comma-separated list of URLs for the HTTP API endpoints for each fallback Execution client.\n\nNOTE: If you are running any on the same machine as your node, addresses like `localhost` and `127.0.0.1` will not work due to Docker limitations. Enter your machine's LAN IP address instead.",
				AffectsContainers:  []ContainerID{ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[Network]string{
				Network_All: "",
			},
		},

		BnHttpUrls: Parameter[string]{
			ParameterCommon: &ParameterCommon{
				ID:                 ids.FallbackBnHttpUrlID,
				Name:               "Beacon Node URLs",
				Description:        "A comma-separated list of URLs for the HTTP Beacon API endpoints for each fallback Beacon Node.\n\nNOTE: If you are running any on the same machine as your node, addresses like `localhost` and `127.0.0.1` will not work due to Docker limitations. Enter your machine's LAN IP address instead.",
				AffectsContainers:  []ContainerID{ContainerID_Daemon, ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[Network]string{
				Network_All: "",
			},
		},

		PrysmRpcUrls: Parameter[string]{
			ParameterCommon: &ParameterCommon{
				ID:                 ids.PrysmRpcUrlID,
				Name:               "RPC URL (Prysm Only)",
				Description:        "**Only used if you have a Prysm Validator Client.**\n\nA comma-separated list of URLs for the Prysm gRPC API endpoints for each fallback Beacon Node. Prysm's Validator Client will need this in order to connect to them.\nNOTE: If you are running any on the same machine as your node, addresses like `localhost` and `127.0.0.1` will not work due to Docker limitations. Enter your machine's LAN IP address instead.",
				AffectsContainers:  []ContainerID{ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[Network]string{
				Network_All: "",
			},
		},
	}
}

// The title for the config
func (cfg *FallbackConfig) GetTitle() string {
	return "Fallback Clients"
}

// Get the Parameters for this config
func (cfg *FallbackConfig) GetParameters() []IParameter {
	return []IParameter{
		&cfg.UseFallbackClients,
		&cfg.EcHttpUrls,
		&cfg.BnHttpUrls,
		&cfg.PrysmRpcUrls,
	}
}

// Get the sections underneath this one
func (cfg *FallbackConfig) GetSubconfigs() map[string]IConfigSection {
	return map[string]IConfigSection{}
}
