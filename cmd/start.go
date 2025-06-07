package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"toolkit/api"
	"toolkit/config"
	"toolkit/core"
	"toolkit/logger"

	"github.com/spf13/cobra"
)

var (
	startServerPort    string
	startProxyPort     string
	startProxyTargetID int64
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts all toolkit services (API server and MITM proxy)",
	Long: `Starts both the web UI/API server and the MITM proxy concurrently.
Press Ctrl+C to gracefully shut down all services.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("--- Start Command: Run ---")

		// --- Determine Ports to Use ---
		actualServerPort := startServerPort
		if !cmd.Flags().Changed("server-port") {
			actualServerPort = config.AppConfig.Server.Port
			logger.Info("Start Command: Server port flag not set, using config value: %s", actualServerPort)
		} else {
			logger.Info("Start Command: Server port flag was set, using flag value: %s", actualServerPort)
		}
		if actualServerPort == "" {
			logger.Error("Start Command: Server port is empty after checking flag and config, defaulting to 8778") // Changed Warn to Error
			actualServerPort = "8778"
		}

		actualProxyPort := startProxyPort
		if !cmd.Flags().Changed("proxy-port") {
			actualProxyPort = config.AppConfig.Proxy.Port
			logger.Info("Start Command: Proxy port flag not set, using config value: %s", actualProxyPort)
		} else {
			logger.Info("Start Command: Proxy port flag was set, using flag value: %s", actualProxyPort)
		}
		if actualProxyPort == "" {
			logger.Error("Start Command: Proxy port is empty after checking flag and config, defaulting to 8777") // Changed Warn to Error
			actualProxyPort = "8777"
		}

		actualProxyTargetID := startProxyTargetID
		logger.Info("Start Command: Final ports determined - Server: %s, Proxy: %s", actualServerPort, actualProxyPort)

		var wg sync.WaitGroup

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// --- Start API Server Goroutine ---
		wg.Add(1)
		go func(parentCtx context.Context) {
			defer wg.Done()
			logger.Info("Start Command Goroutine(API): Attempting to start API server on port %s...", actualServerPort)

			apiRouter := api.NewRouter()
			staticFileDir := "./static"
			fileServer := http.FileServer(http.Dir(staticFileDir))
			mainMux := http.NewServeMux()
			mainMux.Handle("/api/", http.StripPrefix("/api", apiRouter))
			mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					logger.Error("Request for %s reached root handler unexpectedly, passing to API router.", r.URL.Path) // Changed Warn to Error
					http.StripPrefix("/api", apiRouter).ServeHTTP(w, r)
					return
				}
				logger.Info("Start Command Goroutine(API): Attempting to serve static file for: %s", r.URL.Path)
				fileServer.ServeHTTP(w, r)
			})

			server := &http.Server{
				Addr:    ":" + actualServerPort,
				Handler: mainMux,
			}

			go func() {
				<-parentCtx.Done()
				logger.Info("Start Command Goroutine(API): Shutdown signal received...")
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				if err := server.Shutdown(shutdownCtx); err != nil {
					logger.Error("Start Command Goroutine(API): Graceful shutdown failed: %v", err)
				} else {
					logger.Info("Start Command Goroutine(API): Gracefully stopped.")
				}
			}()

			logger.Info("Start Command Goroutine(API): Listening on :%s", actualServerPort)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Start Command Goroutine(API): ListenAndServe error: %v", err)
				cancel()
			}
			logger.Info("Start Command Goroutine(API): Finished.")
		}(ctx)

		// --- Start MITM Proxy Goroutine ---
		wg.Add(1)
		go func(parentCtx context.Context) {
			defer wg.Done()
			logger.ProxyInfo("Start Command Goroutine(Proxy): Attempting to start MITM proxy on port %s...", actualProxyPort)
			if actualProxyTargetID != 0 {
				logger.ProxyInfo("Start Command Goroutine(Proxy): Associating traffic with Target ID: %d", actualProxyTargetID)
			}

			caCertPath := config.AppConfig.Proxy.CACertPath
			caKeyPath := config.AppConfig.Proxy.CAKeyPath
			if caCertPath == "" || caKeyPath == "" {
				logger.Error("Start Command Goroutine(Proxy): CA certificate or key path not configured. Check config or run 'proxy init-ca' first.")
				cancel()
				return
			}
			logger.ProxyInfo("Start Command Goroutine(Proxy): Using CA Cert: %s, CA Key: %s", caCertPath, caKeyPath)

			proxyErrChan := make(chan error, 1)
			go func() {
				logger.ProxyInfo("Start Command Goroutine(Proxy): Calling core.StartMitmProxy...")
				proxyErrChan <- core.StartMitmProxy(actualProxyPort, actualProxyTargetID, caCertPath, caKeyPath)
			}()

			select {
			case err := <-proxyErrChan:
				if err != nil {
					logger.Error("Start Command Goroutine(Proxy): core.StartMitmProxy returned error: %v", err)
					cancel()
				} else {
					logger.ProxyInfo("Start Command Goroutine(Proxy): core.StartMitmProxy exited normally (unexpected unless error occurred).")
				}
			case <-parentCtx.Done():
				logger.ProxyInfo("Start Command Goroutine(Proxy): Shutdown signal received...")
			}
			logger.ProxyInfo("Start Command Goroutine(Proxy): Finished.")
		}(ctx)

		// --- Wait for termination signal ---
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		logger.Info("Start Command: All service goroutines launched. Press Ctrl+C to exit.")

		select {
		case sig := <-sigs:
			logger.Info("Start Command: Received signal: %s. Initiating shutdown...", sig)
		case <-ctx.Done():
			logger.Info("Start Command: Context cancelled (likely due to a service error). Initiating shutdown...")
		}

		cancel()

		shutdownComplete := make(chan struct{})
		go func() {
			logger.Info("Start Command: Waiting for goroutines to finish...")
			wg.Wait()
			logger.Info("Start Command: WaitGroup finished.")
			close(shutdownComplete)
		}()

		select {
		case <-shutdownComplete:
			logger.Info("Start Command: All services shut down.")
		case <-time.After(10 * time.Second):
			logger.Error("Start Command: Shutdown timed out. Forcing exit.")
		}

		logger.Info("Start Command: Exited.")
	},
}

func init() {
	startCmd.Flags().StringVar(&startServerPort, "server-port", "8778", "Port for the API server (overrides config)")
	startCmd.Flags().StringVar(&startProxyPort, "proxy-port", "8777", "Port for the MITM proxy server (overrides config)")
	startCmd.Flags().Int64Var(&startProxyTargetID, "proxy-target-id", 0, "Target ID for the proxy to associate traffic with (optional)")
	rootCmd.AddCommand(startCmd)
}