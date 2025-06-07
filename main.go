package main

import (
	"fmt" 
	"os"
	"toolkit/cmd"
	"toolkit/config" 
	"toolkit/logger"
)

func main() {
	cfgPaths := config.GetDefaultConfigPaths() 
	if err := logger.InitGlobalLoggers(cfgPaths.LogPathApp, cfgPaths.LogPathProxy, cfgPaths.LogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize default global loggers: %v\n", err)
		os.Exit(1) 
	}
	defer logger.CloseLogFiles() 

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic recovered in main: %v\n", r)
			logger.CloseLogFiles() 
			os.Exit(1)
		}
	}()


	cmd.Execute()
}