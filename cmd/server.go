package cmd

import (
	"net/http"
	"strings"
	"toolkit/api"
	"toolkit/logger"

	"github.com/spf13/cobra"
)

var standaloneServerPort string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts the web UI and API server (can be run standalone or as part of 'start')",
	Run: func(cmd *cobra.Command, args []string) {
		portToUse := standaloneServerPort
		if portToUse == "" {
			portToUse = "8778" // Use new default if flag parsing somehow fails
		}

		logger.Info("--- Server Command: Run ---")
		logger.Info("Attempting to start server on port %s...", portToUse)

		logger.Info("Server Command: Calling api.NewRouter()...")
		apiRouter := api.NewRouter()
		if apiRouter == nil {
			logger.Fatal("Server Command: api.NewRouter() returned nil!")
			return
		}
		logger.Info("Server Command: api.NewRouter() returned a handler.")

		staticFileDir := "./static"
		fileServer := http.FileServer(http.Dir(staticFileDir))

		mainMux := http.NewServeMux()

		mainMux.Handle("/api/", http.StripPrefix("/api", apiRouter))
		logger.Info("Server Command: Registered API router under /api/ prefix with StripPrefix.")

		mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				// This shouldn't be hit if the /api/ handle above works, but acts as a safeguard
				logger.Error("Request for %s reached root handler unexpectedly, passing to API router.", r.URL.Path) // Changed Warn to Error
				http.StripPrefix("/api", apiRouter).ServeHTTP(w, r)
				return
			}
			logger.Info("Server Command: Attempting to serve static file for: %s", r.URL.Path)
			fileServer.ServeHTTP(w, r)
		})
		logger.Info("Server Command: Registered static file handler for /.")

		logger.Info("Server Command: API and Static File Handlers configured. Attempting to ListenAndServe on :%s...", portToUse)
		if err := http.ListenAndServe(":"+portToUse, mainMux); err != nil {
			logger.Fatal("Could not start server: %v", err)
		}
		logger.Info("Server Command: ListenAndServe exited (should not happen unless error or shutdown).")
	},
}

func init() {
	// UPDATED default port to 8778
	serverCmd.Flags().StringVarP(&standaloneServerPort, "port", "p", "8778", "Port for the server to listen on (if run standalone)")
	rootCmd.AddCommand(serverCmd)
}