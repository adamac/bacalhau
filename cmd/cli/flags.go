package cli

import (
	"github.com/bacalhau-project/bacalhau/cmd/util/flags"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/logger"
)

var APIFlags = []flags.FlagDefinition{
	{
		FlagName:     "api-host",
		DefaultValue: types.Default.Node.API.Host,
		ConfigPath:   types.NodeAPIHost,
		Description: `The host for the client and server to communicate on (via REST).
Ignored if BACALHAU_API_HOST environment variable is set.`,
	},
	{
		FlagName:     "api-port",
		DefaultValue: types.Default.Node.API.Port,
		ConfigPath:   types.NodeAPIPort,
		Description: `The port for the client and server to communicate on (via REST).
Ignored if BACALHAU_API_PORT environment variable is set.`,
	},
}

var LogFlags = []flags.FlagDefinition{
	{
		FlagName:     "log-mode",
		DefaultValue: logger.LogMode(logger.LogModeDefault),
		ConfigPath:   types.NodeLoggingMode,
		Description:  `Log format: 'default','station','json','combined','event'`,
	},
}