package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"toolkit/logger"

	"github.com/spf13/viper"
)

type DefaultPaths struct {
	ConfigDir        string
	StateFilePath    string
	LogPathApp       string
	LogPathProxy     string
	CACertPath       string
	CAKeyPath        string
	DBPath           string
	LogLevel         string
	SynackTargetsURL string
}

type Configuration struct {
	Database struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"database"`
	Server struct {
		Port    string `mapstructure:"port"`
		LogPath string `mapstructure:"log_path"`
	} `mapstructure:"server"`
	Proxy struct {
		Port                  string `mapstructure:"port"`
		CACertPath            string `mapstructure:"ca_cert_path"`
		CAKeyPath             string `mapstructure:"ca_key_path"`
		LogPath               string `mapstructure:"log_path"`
		ModifierSkipTLSVerify bool   `mapstructure:"modifier_skip_tls_verify"`
		ModifierAllowLoopback bool   `mapstructure:"modifier_allow_loopback"`
	} `mapstructure:"proxy"`
	Logging struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
	Synack struct {
		TargetsURL           string `mapstructure:"targets_url"`
		TargetIDField        string `mapstructure:"target_id_field"`
		AnalyticsEnabled     bool   `mapstructure:"analytics_enabled"`
		AnalyticsBaseURL     string `mapstructure:"analytics_base_url"`
		AnalyticsPathPattern string `mapstructure:"analytics_path_pattern"`
		TargetNameField      string `mapstructure:"target_name_field"`
		TargetsArrayPath     string `mapstructure:"targets_array_path"`
		// Fields for individual findings
		FindingsEnabled                  bool   `mapstructure:"findings_enabled"`
		FindingsBaseURL                  string `mapstructure:"findings_base_url"`
		FindingsPathPattern              string `mapstructure:"findings_path_pattern"`                 // Path for a separate findings endpoint (currently unused if findings are in analytics)
		FindingsArrayPathInAnalyticsJson string `mapstructure:"findings_array_path_in_analytics_json"` // GJSON path to findings array within the analytics response
	} `mapstructure:"synack"`
	UI struct {
		ShowSynackSection bool `mapstructure:"showSynackSection" yaml:"showSynackSection"`
	} `mapstructure:"ui"`
}

var AppConfig Configuration

func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}

func GetDefaultConfigPaths() DefaultPaths {
	var paths DefaultPaths
	userConfigDirBase, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not get user config dir: %v. Using current directory.\n", err)
		userConfigDirBase = "."
	}

	userConfigDir, err := expandTilde(userConfigDirBase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in user config dir '%s': %v. Using potentially literal path.\n", userConfigDirBase, err)
		userConfigDir = userConfigDirBase
	}

	paths.ConfigDir = filepath.Join(userConfigDir, "toolkit")
	logDir := filepath.Join(paths.ConfigDir, "logs")

	paths.StateFilePath = filepath.Join(paths.ConfigDir, "state.json")
	paths.LogPathApp = filepath.Join(logDir, "app.log")
	paths.LogPathProxy = filepath.Join(logDir, "proxy.log")
	paths.CACertPath = filepath.Join(paths.ConfigDir, "mytool-ca.crt")
	paths.CAKeyPath = filepath.Join(paths.ConfigDir, "mytool-ca.key")
	paths.DBPath = filepath.Join(paths.ConfigDir, "bountytool.db")
	paths.LogLevel = "DEBUG"
	paths.SynackTargetsURL = "https://platform.synack.com/api/targets/registered_summary"
	return paths
}

func Init(cfgFile string, flagAppLogPath, flagProxyLogPath, flagLogLevel string) error {
	v := viper.New()

	defaults := GetDefaultConfigPaths()
	v.SetDefault("database.path", defaults.DBPath)
	v.SetDefault("server.port", "8778") // UPDATED default server port
	v.SetDefault("server.log_path", defaults.LogPathApp)
	v.SetDefault("proxy.port", "8777") // UPDATED default proxy port
	v.SetDefault("proxy.ca_cert_path", defaults.CACertPath)
	v.SetDefault("proxy.ca_key_path", defaults.CAKeyPath)
	v.SetDefault("proxy.log_path", defaults.LogPathProxy)
	v.SetDefault("proxy.modifier_skip_tls_verify", false) // Default to secure: verify TLS
	v.SetDefault("proxy.modifier_allow_loopback", false)  // Default to secure: disallow loopback
	v.SetDefault("logging.level", defaults.LogLevel)
	v.SetDefault("synack.targets_url", defaults.SynackTargetsURL)
	v.SetDefault("synack.target_id_field", "id")
	v.SetDefault("synack.target_name_field", "name")
	v.SetDefault("synack.targets_array_path", "")
	v.SetDefault("ui.showSynackSection", false) // Default to false
	v.SetDefault("synack.analytics_enabled", false)
	v.SetDefault("synack.analytics_base_url", "https://platform.synack.com")
	v.SetDefault("synack.analytics_path_pattern", "/api/listing_analytics/categories?listing_id=%s&status=accepted")
	// Defaults for new findings fields
	v.SetDefault("synack.findings_enabled", false)                                                         // Default to false
	v.SetDefault("synack.findings_base_url", "https://platform.synack.com")                                // Example, adjust as needed
	v.SetDefault("synack.findings_path_pattern", "/api/v1/targets/%s/vulnerabilities")                     // Example, adjust as needed
	v.SetDefault("synack.findings_array_path_in_analytics_json", "value.#.exploitable_locations|@flatten") // Corrected default GJSON path

	if cfgFile != "" {
		expandedCfgFile, err := expandTilde(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in config file path '%s': %v. Trying original path.\n", cfgFile, err)
			expandedCfgFile = cfgFile
		}
		v.SetConfigFile(expandedCfgFile)
		v.SetConfigType("yaml")
	} else {
		v.AddConfigPath(defaults.ConfigDir)
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("TOOLKIT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	configUsedMsg := "Using default/environment configuration."
	readErr := v.ReadInConfig()
	if readErr == nil {
		configUsedMsg = fmt.Sprintf("Using config file: %s", v.ConfigFileUsed())
	} else {
		if _, ok := readErr.(viper.ConfigFileNotFoundError); ok {
			if cfgFile != "" {
				fmt.Fprintf(os.Stderr, "Warning: Config file specified by flag (%s) not found: %v\n", cfgFile, readErr)
			} else {
				fmt.Fprintln(os.Stderr, "No default config file found. Using defaults/environment variables.")
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error reading config file %s: %v\n", v.ConfigFileUsed(), readErr)
		}
	}

	if err := v.Unmarshal(&AppConfig); err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Error unmarshalling configuration: %v\n", err)
		return fmt.Errorf("unable to decode config into struct: %w", err)
	}

	// Apply flag overrides
	if flagAppLogPath != "" {
		expandedPath, err := expandTilde(flagAppLogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in --app-log path '%s': %v. Using original path.\n", flagAppLogPath, err)
			AppConfig.Server.LogPath = flagAppLogPath
		} else {
			AppConfig.Server.LogPath = expandedPath
		}
	}
	if flagProxyLogPath != "" {
		expandedPath, err := expandTilde(flagProxyLogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in --proxy-log path '%s': %v. Using original path.\n", flagProxyLogPath, err)
			AppConfig.Proxy.LogPath = flagProxyLogPath
		} else {
			AppConfig.Proxy.LogPath = expandedPath
		}
	}
	if flagLogLevel != "" {
		AppConfig.Logging.Level = strings.ToUpper(flagLogLevel)
	}

	// Expand tilde for paths read from config that might contain it
	var err error
	AppConfig.Database.Path, err = expandTilde(AppConfig.Database.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in database.path '%s': %v.\n", AppConfig.Database.Path, err)
	}
	AppConfig.Proxy.CACertPath, err = expandTilde(AppConfig.Proxy.CACertPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in proxy.ca_cert_path '%s': %v.\n", AppConfig.Proxy.CACertPath, err)
	}
	AppConfig.Proxy.CAKeyPath, err = expandTilde(AppConfig.Proxy.CAKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not expand tilde in proxy.ca_key_path '%s': %v.\n", AppConfig.Proxy.CAKeyPath, err)
	}
	// Note: Log paths are already handled by flag overrides or defaults which use expandTilde.

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(AppConfig.Server.LogPath), 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not create final app log directory %s: %v\n", filepath.Dir(AppConfig.Server.LogPath), err)
	}
	if err := os.MkdirAll(filepath.Dir(AppConfig.Proxy.LogPath), 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not create final proxy log directory %s: %v\n", filepath.Dir(AppConfig.Proxy.LogPath), err)
	}
	if err := os.MkdirAll(defaults.ConfigDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not create main config directory %s: %v\n", defaults.ConfigDir, err)
	}

	// Initialize/Re-initialize loggers
	if err := logger.InitGlobalLoggers(AppConfig.Server.LogPath, AppConfig.Proxy.LogPath, AppConfig.Logging.Level); err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to initialize global loggers with final config: %v\n", err)
		return fmt.Errorf("failed to initialize global loggers with final config: %w", err)
	}

	logger.Info(configUsedMsg)
	if readErr != nil && cfgFile != "" {
		logger.Error("Error occurred reading specified config file '%s': %v", cfgFile, readErr)
	}
	if flagAppLogPath != "" || flagProxyLogPath != "" || flagLogLevel != "" {
		logger.Info("Log path/level flags may have overridden config file/defaults.")
	}

	if AppConfig.Synack.TargetsURL == "" {
		logger.Error("Synack TargetsURL is not configured. Synack target processing will be disabled.")
	} else {
		logger.Info("Synack Target URL configured: %s", AppConfig.Synack.TargetsURL)
	}

	if AppConfig.Synack.AnalyticsEnabled {
		logger.Info("Synack Target Analytics fetching ENABLED. BaseURL: %s, PathPattern: %s", AppConfig.Synack.AnalyticsBaseURL, AppConfig.Synack.AnalyticsPathPattern)
	} else {
		logger.Info("Synack Target Analytics fetching DISABLED.")
	}
	if AppConfig.Synack.FindingsEnabled {
		logger.Info("Synack Individual Findings parsing from Analytics ENABLED. Expected JSON path: '%s'", AppConfig.Synack.FindingsArrayPathInAnalyticsJson)
	} else {
		logger.Info("Synack Individual Findings fetching DISABLED.")
	}
	if AppConfig.Proxy.ModifierSkipTLSVerify {
		logger.Warn("Modifier: TLS certificate verification for outgoing requests is DISABLED.")
	}
	if AppConfig.Proxy.ModifierAllowLoopback {
		logger.Warn("Modifier: Requests to loopback addresses are ALLOWED.")
	}

	logger.Debug("Final AppConfig Initialized: %+v", AppConfig)
	return nil
}
