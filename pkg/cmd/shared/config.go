package shared

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

// InitConfig initializes Viper configuration for any CLI
func InitConfig(appName string, globalOpts *options.GlobalOptions) {
	if globalOpts.ConfigFile != "" {
		viper.SetConfigFile(globalOpts.ConfigFile)
	} else {
		// Search for config in home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}

		// Search config in home directory with name ".{appname}" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(fmt.Sprintf(".%s", appName))
		viper.SetConfigType("yaml")
	}

	// Environment variable prefix (uppercase app name)
	viper.SetEnvPrefix(appName)
	viper.AutomaticEnv()

	// Read config file if found
	if err := viper.ReadInConfig(); err == nil && globalOpts.Verbose {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}
