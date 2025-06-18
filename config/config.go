package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"toolkit/logger"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
// DatabaseConfig holds database related configuration.
type DatabaseConfig struct {
	Path string `mapstructure:"path" yaml:"path"`
}

// ServerConfig holds server related configuration.
type ServerConfig struct {
	Port    string `mapstructure:"port" yaml:"port"`
	LogPath string `mapstructure:"log_path" yaml:"log_path"`
}

// ProxyConfig holds proxy related configuration.
type ProxyConfig struct {
	Port                  string `mapstructure:"port" yaml:"port"`
	CACertPath            string `mapstructure:"ca_cert_path" yaml:"ca_cert_path"`
	CAKeyPath             string `mapstructure:"ca_key_path" yaml:"ca_key_path"`
	LogPath               string `mapstructure:"log_path" yaml:"log_path"`
	ModifierSkipTLSVerify bool   `mapstructure:"modifier_skip_tls_verify" yaml:"modifier_skip_tls_verify"`
	ModifierAllowLoopback bool   `mapstructure:"modifier_allow_loopback" yaml:"modifier_allow_loopback"`
}

// LoggingConfig holds logging related configuration.
type LoggingConfig struct {
	Level string `mapstructure:"level" yaml:"level"`
}

// SynackConfig holds Synack integration related configuration.
type SynackConfig struct {
	TargetsURL                       string `mapstructure:"targets_url" yaml:"targets_url"`
	TargetIDField                    string `mapstructure:"target_id_field" yaml:"target_id_field"`
	AnalyticsEnabled                 bool   `mapstructure:"analytics_enabled" yaml:"analytics_enabled"`
	AnalyticsBaseURL                 string `mapstructure:"analytics_base_url" yaml:"analytics_base_url"`
	AnalyticsPathPattern             string `mapstructure:"analytics_path_pattern" yaml:"analytics_path_pattern"`
	TargetNameField                  string `mapstructure:"target_name_field" yaml:"target_name_field"`
	TargetsArrayPath                 string `mapstructure:"targets_array_path" yaml:"targets_array_path"` // GJSON path to the array of targets in the targets_url response
	FindingsEnabled                  bool   `mapstructure:"findings_enabled" yaml:"findings_enabled"`
	FindingsBaseURL                  string `mapstructure:"findings_base_url" yaml:"findings_base_url"`
	FindingsPathPattern              string `mapstructure:"findings_path_pattern" yaml:"findings_path_pattern"`                 // Path for a separate findings endpoint (currently unused if findings are in analytics)
	FindingsArrayPathInAnalyticsJson string `mapstructure:"findings_array_path_in_analytics_json" yaml:"findings_array_path_in_analytics_json"` // GJSON path to findings array within the analytics response
}

// MissionsConfig holds Synack Missions feature related configuration.
type MissionsConfig struct {
	Enabled                bool    `mapstructure:"enabled" yaml:"enabled"`
	PollingIntervalSeconds int     `mapstructure:"polling_interval_seconds" yaml:"polling_interval_seconds"`
	ListURL                string  `mapstructure:"list_url" yaml:"list_url"`
	ClaimURLPattern        string  `mapstructure:"claim_url_pattern" yaml:"claim_url_pattern"` // Pattern like /api/tasks/v1/organizations/%s/listings/%s/campaigns/%s/tasks/%s/transitions
	ClaimMinPayout         float64 `mapstructure:"claim_min_payout" yaml:"claim_min_payout"`   // Minimum payout to consider claiming
	ClaimMaxPayout         float64 `mapstructure:"claim_max_payout" yaml:"claim_max_payout"`   // Maximum payout to consider claiming (e.g., up to $50)
}

// UIConfig holds UI related configuration.
type UIConfig struct {
	ShowSynackSection bool `mapstructure:"showSynackSection" yaml:"showSynackSection"`
}

// Configuration is the main application configuration struct.
type Configuration struct {
	Database DatabaseConfig `mapstructure:"database" yaml:"database"`
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Proxy    ProxyConfig    `mapstructure:"proxy" yaml:"proxy"`
	Logging  LoggingConfig  `mapstructure:"logging" yaml:"logging"`
	Synack   SynackConfig   `mapstructure:"synack" yaml:"synack"`
	Missions MissionsConfig `mapstructure:"missions" yaml:"missions"`
	UI       UIConfig       `mapstructure:"ui" yaml:"ui"`
}

var AppConfig Configuration
var configFileUsedPath string // Stores the path of the config file viper actually used/tried to use

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
	// Defaults for new missions fields
	v.SetDefault("missions.enabled", false)
	v.SetDefault("missions.polling_interval_seconds", 10)
	v.SetDefault("missions.list_url", "https://platform.synack.com/api/tasks?perPage=20&viewed=true&page=1&status=PUBLISHED&sort=CLAIMABLE&sortDir=DESC&includeAssignedBySynackUser=true")
	v.SetDefault("missions.claim_url_pattern", "https://platform.synack.com/api/tasks/v1/organizations/%s/listings/%s/campaigns/%s/tasks/%s/transitions") // orgId, listingId, campaignId, taskId
	v.SetDefault("missions.claim_min_payout", 0.0)   // Default to claim any mission with a payout (can be set higher)
	v.SetDefault("missions.claim_max_payout", 50.0)  // Default to claim missions with payout $50 or less

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

	var actualCfgFileUsed string
	configUsedMsg := "Using default/environment configuration."
	readErr := v.ReadInConfig()

	if readErr == nil {
		actualCfgFileUsed = v.ConfigFileUsed()
		configUsedMsg = fmt.Sprintf("Using config file: %s", actualCfgFileUsed)
	} else {
		if cfgFile != "" { // User specified a file
			// v.ConfigFileUsed() should return the path set by SetConfigFile, even if read failed
			actualCfgFileUsed = v.ConfigFileUsed()
			if actualCfgFileUsed == "" { // If SetConfigFile was called with empty or somehow viper cleared it
				actualCfgFileUsed = cfgFile // Fallback to the original flag value
			}
			if _, ok := readErr.(viper.ConfigFileNotFoundError); ok {
				configUsedMsg = fmt.Sprintf("Config file specified (%s) not found. Will create on save.", actualCfgFileUsed)
				fmt.Fprintf(os.Stderr, "Warning: %s\n", configUsedMsg)
			} else {
				configUsedMsg = fmt.Sprintf("Error reading specified config file %s: %v. Will attempt to overwrite on save.", actualCfgFileUsed, readErr)
				fmt.Fprintf(os.Stderr, "Warning: %s\n", configUsedMsg)
			}
		} else { // No specific file, Viper searched defaults
			if _, ok := readErr.(viper.ConfigFileNotFoundError); ok {
				actualCfgFileUsed = "" // Indicate no file was used, save will use default path.
				configUsedMsg = "No default config file found. Using defaults/environment variables. Will save to default path."
				fmt.Fprintln(os.Stderr, configUsedMsg)
			} else { // Error reading a default file that was found.
				actualCfgFileUsed = v.ConfigFileUsed()
				configUsedMsg = fmt.Sprintf("Error reading default config file %s: %v. Will attempt to overwrite on save.", actualCfgFileUsed, readErr)
				fmt.Fprintf(os.Stderr, "Warning: %s\n", configUsedMsg)
			}
		}
	}
	configFileUsedPath = actualCfgFileUsed // Set the global package variable

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

// SaveAppConfig persists the current AppConfig to the configuration file.
func SaveAppConfig() error {
	filePathToSave := configFileUsedPath

	if filePathToSave == "" {
		// No config file was loaded or specified initially, so save to the default location.
		defaults := GetDefaultConfigPaths()
		// Ensure the default config directory exists first.
		if err := os.MkdirAll(defaults.ConfigDir, 0750); err != nil {
			return fmt.Errorf("failed to create default config directory %s: %w", defaults.ConfigDir, err)
		}
		filePathToSave = filepath.Join(defaults.ConfigDir, "config.yaml") // Viper's default name and type
		logger.Info("No specific config file was loaded. Saving to default path: %s", filePathToSave)
	}

	// Ensure the directory for the target file path exists.
	dir := filepath.Dir(filePathToSave)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s for config file: %w", dir, err)
	}

	yamlData, err := yaml.Marshal(&AppConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal AppConfig to YAML: %w", err)
	}

	logger.Info("Attempting to save application configuration to %s", filePathToSave)
	return os.WriteFile(filePathToSave, yamlData, 0640) // Permissions like 0640 (rw-r-----) or 0600 (rw-------)
}
