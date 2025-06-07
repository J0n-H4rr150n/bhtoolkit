package cmd

import (
	"fmt"
	// "os" // Not used
	"toolkit/config"
	"toolkit/core"
	"toolkit/logger"

	"github.com/spf13/cobra"
)

var standaloneProxyPort string
var standaloneProxyTargetID int64

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manages the MITM proxy server (can be run standalone or as part of 'start')",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the MITM proxy server",
	Long: `Starts the Man-in-the-Middle proxy to intercept and log HTTP/S traffic.
You will need to configure your browser or system to use this proxy.
A CA certificate (e.g., mytool-ca.crt) must be generated (using 'proxy init-ca') and trusted by your client.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Determine port to use: flag > config > default
		portToUse := standaloneProxyPort // Start with flag value
		if !cmd.Flags().Changed("port") { // Check if the flag was set by the user
			portToUse = config.AppConfig.Proxy.Port // Use config value if flag wasn't set
			logger.Debug("Using proxy port from config: %s", portToUse)
		} else {
			logger.Debug("Using proxy port from flag: %s", portToUse)
		}
		// Final fallback if still empty
		if portToUse == "" {
			portToUse = "8777" // Use new default
		}

		targetIDToUse := standaloneProxyTargetID

		logger.ProxyInfo("Attempting to start MITM proxy on port %s...", portToUse)
		if targetIDToUse != 0 {
			logger.ProxyInfo("Proxy will associate traffic with Target ID: %d", targetIDToUse)
		}

		caCertPath := config.AppConfig.Proxy.CACertPath
		caKeyPath := config.AppConfig.Proxy.CAKeyPath
		if caCertPath == "" || caKeyPath == "" {
			logger.Error("Proxy CA certificate or key path not configured. Check config or run 'proxy init-ca' first.")
			return
		}
		logger.ProxyInfo("Proxy using CA Cert: %s, CA Key: %s", caCertPath, caKeyPath)

		err := core.StartMitmProxy(portToUse, targetIDToUse, caCertPath, caKeyPath)
		if err != nil {
			logger.ProxyError("Error starting proxy: %v", err)
		}
	},
}

var proxyInitCACmd = &cobra.Command{
	Use:   "init-ca",
	Short: "Initializes (generates) the root CA certificate and key for the MITM proxy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing Proxy CA...")
		certPath := config.AppConfig.Proxy.CACertPath
		keyPath := config.AppConfig.Proxy.CAKeyPath

		if certPath == "" || keyPath == "" {
			logger.Error("CA certificate or key path is not defined in configuration.")
			logger.Error("Please check your config setup (e.g., ensure $HOME/.config/toolkit directory can be created or provide paths via flags/config file).")
			return
		}

		err := core.GenerateAndSaveCA(certPath, keyPath)
		if err != nil {
			fmt.Printf("Error generating CA. Check logs for details: %v\n", err)
			return
		}
		fmt.Println("Please import the CA certificate (e.g., mytool-ca.crt) into your browser/system's trust store.")
	},
}

func init() {
	// UPDATED default port to 8777
	proxyStartCmd.Flags().StringVarP(&standaloneProxyPort, "port", "p", "8777", "Port for the proxy server to listen on (overrides config)")
	proxyStartCmd.Flags().Int64VarP(&standaloneProxyTargetID, "target-id", "t", 0, "Target ID to associate logged traffic with (optional)")

	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyInitCACmd)
	rootCmd.AddCommand(proxyCmd)
}