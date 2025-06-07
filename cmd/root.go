package cmd

import (
	"fmt"
	"os"
	"path/filepath" // Added for path manipulation
	"strings"
	"toolkit/config"
	"toolkit/database"
	"toolkit/logger"

	"github.com/spf13/cobra"
)

var (
	cfgFile          string
	dbPath           string // Bound to --dbpath flag
	appLogPathFlag   string
	proxyLogPathFlag string
	logLevelFlag     string
)

// Helper function to expand tilde in this package too
func expandTildeCmd(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}


var rootCmd = &cobra.Command{
	Use:   "toolkit",
	Short: "A brief description of your bug bounty tool",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to simplify bug bounty hunting tasks.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize config first, passing flag values for logging config
		if err := config.Init(cfgFile, appLogPathFlag, proxyLogPathFlag, logLevelFlag); err != nil {
			return fmt.Errorf("failed to initialize config in PersistentPreRunE: %w", err)
		}

		// --- Start DB Path Determination and Expansion ---
		finalDBPath := dbPath // Get value from flag first
		configDBPath := config.AppConfig.Database.Path // Get value potentially loaded from config file

		if finalDBPath != "" {
			// Flag was provided, expand it
			expandedPath, err := expandTildeCmd(finalDBPath)
			if err != nil {
				logger.Error("Error expanding tilde in --dbpath flag '%s': %v. Using original.", finalDBPath, err)
				// Use original path from flag if expansion fails
			} else {
				finalDBPath = expandedPath
				logger.Info("PersistentPreRunE: Using expanded database path from --dbpath flag: '%s'", finalDBPath)
			}
		} else {
			// Flag was not provided, use path from config (which should already have default expanded)
			finalDBPath = configDBPath
			logger.Info("PersistentPreRunE: --dbpath flag was empty, using config path: '%s'", finalDBPath)
			// No need to expand here again if GetDefaultConfigPaths does it, unless config file itself contains '~'
			// For robustness, expand here too in case config file had a tilde
			expandedPath, err := expandTildeCmd(finalDBPath)
			if err != nil {
				logger.Error("Error expanding tilde in config DB path '%s': %v. Using original.", finalDBPath, err)
				// Use original path if expansion fails
			} else {
				finalDBPath = expandedPath
			}
		}

		// Final fallback if everything else resulted in empty
		if finalDBPath == "" {
			logger.Error("PersistentPreRunE: Database path is empty after checking flag and config! Falling back to 'bountytool.db' in CWD.")
			finalDBPath = "bountytool.db"
		}
		// --- End DB Path Determination and Expansion ---


		logger.Info("PersistentPreRunE: Attempting to InitDB with final path: '%s'", finalDBPath)
		if err := database.InitDB(finalDBPath); err != nil {
			return fmt.Errorf("failed to initialize database at %s: %w", finalDBPath, err)
		}

		isSuppressedCmd := false
		if cmd.Name() == "completion" ||
			cmd.Name() == cobra.ShellCompRequestCmd ||
			cmd.Name() == cobra.ShellCompNoDescRequestCmd ||
			cmd.Name() == "start" {
			isSuppressedCmd = true
		}

		if !isSuppressedCmd {
			logger.Info("Database initialized at: %s (from rootCmd PersistentPreRunE)", finalDBPath)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/toolkit/config.yaml or ./config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbPath, "dbpath", "", "path to SQLite database file (overrides config/default)")
	rootCmd.PersistentFlags().StringVar(&appLogPathFlag, "app-log", "", "path for the application log file (overrides config/default)")
	rootCmd.PersistentFlags().StringVar(&proxyLogPathFlag, "proxy-log", "", "path for the proxy log file (overrides config/default)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "", "log level: DEBUG, INFO, ERROR (overrides config/default)")
}