package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	AppLogger   *log.Logger
	ProxyLogger *log.Logger
	WarnLogger  *log.Logger
	ErrorLogger *log.Logger

	logLevel     string
	appLogFile   *os.File
	proxyLogFile *os.File
	initialized  bool
)

func InitGlobalLoggers(appLogPath, proxyLogPath, level string) error {
	if initialized && appLogFile != nil && proxyLogFile != nil && strings.ToUpper(level) == logLevel {
		// Already initialized with same settings, perhaps a redundant call.
		// log.Println("Loggers already initialized with same settings.") // Use standard log for this meta-message
		return nil
	}
	// If files are open, close them before re-initializing
	if appLogFile != nil {
		appLogFile.Close()
		appLogFile = nil
	}
	if proxyLogFile != nil {
		proxyLogFile.Close()
		proxyLogFile = nil
	}

	logLevel = strings.ToUpper(level)
	if logLevel == "" {
		logLevel = "INFO"
	}

	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	actualAppLogPath := appLogPath
	appLogDir := filepath.Dir(appLogPath)
	var appLogWriter io.Writer = io.Discard
	if err := os.MkdirAll(appLogDir, 0750); err != nil {
		ErrorLogger.Printf("Failed to create app log directory %s: %v. App logs (Info/Debug) will be discarded.", appLogDir, err)
		actualAppLogPath = "(discarded)"
	} else {
		var errApp error
		appLogFile, errApp = os.OpenFile(appLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if errApp != nil {
			ErrorLogger.Printf("Failed to open app log file %s: %v. App logs (Info/Debug) will be discarded.", appLogPath, errApp)
			actualAppLogPath = "(discarded)"
		} else {
			appLogWriter = appLogFile // Always use the file if openable
		}
	}
	AppLogger = log.New(appLogWriter, "APP: ", log.Ldate|log.Ltime|log.Lshortfile)

	actualProxyLogPath := proxyLogPath
	proxyLogDir := filepath.Dir(proxyLogPath)
	var proxyLogWriter io.Writer = io.Discard
	if err := os.MkdirAll(proxyLogDir, 0750); err != nil {
		ErrorLogger.Printf("Failed to create proxy log directory %s: %v. Proxy logs (Info/Debug) will be discarded.", proxyLogDir, err)
		actualProxyLogPath = "(discarded)"
	} else {
		var errProxy error
		proxyLogFile, errProxy = os.OpenFile(proxyLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if errProxy != nil {
			ErrorLogger.Printf("Failed to open proxy log file %s: %v. Proxy logs (Info/Debug) will be discarded.", proxyLogPath, errProxy)
			actualProxyLogPath = "(discarded)"
		} else {
			proxyLogWriter = proxyLogFile // Always use the file if openable
		}
	}
	ProxyLogger = log.New(proxyLogWriter, "PROXY: ", log.Ldate|log.Ltime|log.Lshortfile)

	if !initialized { // Print init messages only once
		AppLogger.Printf("App logger initialized. Log level: %s. Output file: %s", logLevel, actualAppLogPath)
		ProxyLogger.Printf("Proxy logger initialized. Log level: %s. Output file: %s", logLevel, actualProxyLogPath)
	}
	initialized = true
	return nil
}

func Info(format string, v ...interface{}) {
	if AppLogger != nil && (logLevel == "INFO" || logLevel == "DEBUG") {
		AppLogger.Printf(format, v...)
	}
}

func Debug(format string, v ...interface{}) {
	if AppLogger != nil && logLevel == "DEBUG" {
		AppLogger.Printf(format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if WarnLogger != nil && (logLevel == "WARN" || logLevel == "INFO" || logLevel == "DEBUG") { // Warnings also show if level is INFO or DEBUG
		AppLogger.Printf(format, v...)
	}
}

func Error(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	if ErrorLogger != nil {
		ErrorLogger.Print(message)
	}
	if AppLogger != nil && appLogFile != nil {
		AppLogger.Print(message)
	}
}

func Fatal(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	if ErrorLogger != nil {
		ErrorLogger.Fatal(message)
	} else {
		log.Fatal(message)
	}
}

func ProxyInfo(format string, v ...interface{}) {
	if ProxyLogger != nil && (logLevel == "INFO" || logLevel == "DEBUG") {
		ProxyLogger.Printf(format, v...)
	}
}

func ProxyDebug(format string, v ...interface{}) {
	if ProxyLogger != nil && logLevel == "DEBUG" {
		ProxyLogger.Printf(format, v...)
	}
}

func ProxyError(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	if ErrorLogger != nil { // All errors go to stderr via ErrorLogger
		ErrorLogger.Print(message)
	}
	if ProxyLogger != nil && proxyLogFile != nil { // Also write to proxy log file
		ProxyLogger.Print(message)
	}
}

func CloseLogFiles() {
	if appLogFile != nil {
		AppLogger.Println("Closing app log file.")
		appLogFile.Close()
		appLogFile = nil // Prevent double close
	}
	if proxyLogFile != nil {
		ProxyLogger.Println("Closing proxy log file.")
		proxyLogFile.Close()
		proxyLogFile = nil // Prevent double close
	}
	initialized = false // Allow re-initialization if needed (e.g. tests)
}
