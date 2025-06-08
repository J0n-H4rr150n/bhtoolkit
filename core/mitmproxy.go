package core

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"

	//"errors" // ADDED for os.ErrNotExist
	"fmt"
	"io"
	"log" // Standard log package for goproxy.Logger config
	"math/big"
	"net" // ADDED for CIDR and IP parsing
	"net/http"
	"net/url" // ADDED: For URL parsing
	"os"      // ADDED for os.ErrNotExist

	//"path/filepath" // ADDED for state file path
	// "regexp"        // ADDED for scope matching - Commented out as not currently used
	"regexp" // ADDED for regex matching in global exclusions
	"strings"
	"sync"
	"time"
	"toolkit/config"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"strconv" // For parsing target ID from string

	"github.com/elazarl/goproxy"
	"github.com/tidwall/gjson"
)

var (
	caCert                *x509.Certificate
	caKey                 *rsa.PrivateKey
	sessionIsHTTPS        = make(map[int64]bool)
	muSession             sync.Mutex
	parsedSynackTargetURL *url.URL // Cache the parsed config URL
	parseURLErr           error    // Store any error during parsing

	// For scoped logging
	activeTargetID       *int64
	allActiveScopeRules  []models.ScopeRule // Renamed from activeScopeRules
	scopeMu              sync.RWMutex
	globalExclusionRules []models.ProxyExclusionRule
	analyticsFetchTicker *time.Ticker // Added for rate limiting analytics calls
)

// proxyRequestContextData holds data passed between request and response handlers via ctx.UserData
type proxyRequestContextData struct {
	TrafficLog      *models.HTTPTrafficLog
	SynackAuthToken string // To store the Bearer token from Synack target list request
}

// Initialize the parsed Synack URL once
func init() {
	// Note: config.AppConfig might not be fully populated during Go's init phase.
	// It's better to parse this lazily or after config is loaded.
	// We will parse it inside StartMitmProxy instead.
	// Initialize the ticker for analytics API calls (e.g., 1 request per second)
	analyticsFetchTicker = time.NewTicker(1 * time.Second)
}

// Helper function to find minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// setGoproxyCA function remains the same
func setGoproxyCA(loadedGoproxyCa *tls.Certificate) {
	if loadedGoproxyCa == nil {
		logger.Fatal("setGoproxyCA called with nil certificate")
	}
	goproxy.GoproxyCa = *loadedGoproxyCa
	logger.ProxyInfo("goproxy CA configured (Simplified Mode - Direct Action Construction).")
}

func GenerateAndSaveCA(certPath, keyPath string) error {
	var localCaCert *x509.Certificate
	var localCaKey *rsa.PrivateKey
	var err error

	localCaCert, localCaKey, err = generateCA("toolkit MITM Proxy CA")
	if err != nil {
		logger.Error("Failed to generate CA: %v", err)
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		logger.Error("Failed to open %s for writing: %v", certPath, err)
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: localCaCert.Raw}); err != nil {
		logger.Error("Failed to write CA certificate to %s: %v", certPath, err)
		return fmt.Errorf("failed to write CA certificate to %s: %w", certPath, err)
	}
	fmt.Printf("CA certificate saved to %s\n", certPath)

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Error("Failed to open %s for writing: %v", keyPath, err)
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(localCaKey)
	if err != nil {
		logger.ProxyInfo("Warning: could not marshal private key to PKCS8: %v. Trying PKCS1.", err)
		privBytes = x509.MarshalPKCS1PrivateKey(localCaKey)
		if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
			logger.Error("Failed to write CA RSA private key to %s: %v", keyPath, err)
			return fmt.Errorf("failed to write CA RSA private key to %s: %w", keyPath, err)
		}
	} else {
		if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
			logger.Error("Failed to write CA private key to %s: %v", keyPath, err)
			return fmt.Errorf("failed to write CA private key to %s: %w", keyPath, err)
		}
	}
	fmt.Printf("CA private key saved to %s\n", keyPath)
	return nil
}

func loadCA(certPath, keyPath string) error {
	certPEMBlock, err := os.ReadFile(certPath)
	if err != nil {
		logger.Error("Failed to read CA certificate file %s: %v", certPath, err)
		return fmt.Errorf("failed to read CA certificate file %s: %w", certPath, err)
	}
	certDERBlock, _ := pem.Decode(certPEMBlock)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		logger.Error("Failed to decode CA certificate PEM block from %s", certPath)
		return fmt.Errorf("failed to decode CA certificate PEM block from %s", certPath)
	}
	loadedCaCert, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		logger.Error("Failed to parse CA certificate from %s: %v", certPath, err)
		return fmt.Errorf("failed to parse CA certificate from %s: %w", certPath, err)
	}
	caCert = loadedCaCert

	keyPEMBlock, err := os.ReadFile(keyPath)
	if err != nil {
		logger.Error("Failed to read CA key file %s: %v", keyPath, err)
		return fmt.Errorf("failed to read CA key file %s: %w", keyPath, err)
	}
	keyDERBlock, _ := pem.Decode(keyPEMBlock)
	if keyDERBlock == nil {
		logger.Error("Failed to decode CA key PEM block from %s (key block is nil)", keyPath)
		return fmt.Errorf("failed to decode CA key PEM block from %s (key block is nil)", keyPath)
	}

	var parsedKey interface{}
	if keyDERBlock.Type == "PRIVATE KEY" {
		parsedKey, err = x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	} else if keyDERBlock.Type == "RSA PRIVATE KEY" {
		parsedKey, err = x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
	} else {
		logger.Error("Unknown CA key PEM block type '%s' from %s", keyDERBlock.Type, keyPath)
		return fmt.Errorf("unknown CA key PEM block type '%s' from %s", keyDERBlock.Type, keyPath)
	}

	if err != nil {
		logger.Error("Failed to parse CA private key from %s (type %s): %v", keyPath, keyDERBlock.Type, err)
		return fmt.Errorf("failed to parse CA private key from %s (type %s): %w", keyPath, keyDERBlock.Type, err)
	}

	var ok bool
	loadedCaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		logger.Error("CA key from %s is not an RSA private key after parsing type %s", keyPath, keyDERBlock.Type)
		return fmt.Errorf("CA key from %s is not an RSA private key after parsing type %s", keyPath, keyDERBlock.Type)
	}
	caKey = loadedCaKey

	logger.ProxyInfo("CA certificate and key loaded successfully.")
	return nil
}

func generateCA(commonName string) (*x509.Certificate, *rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"toolkit Development CA"},
			CommonName:   commonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse generated CA certificate: %w", err)
	}
	return cert, privKey, nil
}

// matchesRule is a helper to check if a request matches a single scope rule.
func matchesRule(requestURL *url.URL, hostname, path string, rule models.ScopeRule) bool {
	pattern := rule.Pattern
	match := false

	switch rule.ItemType {
	case "domain":
		if hostname == pattern {
			match = true
		}
	case "subdomain":
		if strings.HasPrefix(pattern, "*.") {
			domainPart := strings.TrimPrefix(pattern, "*.")
			if hostname == domainPart || strings.HasSuffix(hostname, "."+domainPart) {
				match = true
			}
		} else if hostname == pattern {
			match = true
		}
	case "url_path":
		rulePatternPath := rule.Pattern
		// Normalize rulePatternPath: ensure it starts with a slash
		if rulePatternPath != "" && !strings.HasPrefix(rulePatternPath, "/") {
			rulePatternPath = "/" + rulePatternPath
		}

		// Normalize requestPathNormalized: ensure it starts with a slash
		requestPathNormalized := path
		if requestPathNormalized != "" && !strings.HasPrefix(requestPathNormalized, "/") {
			requestPathNormalized = "/" + requestPathNormalized
		}

		// Ensure rulePatternPath ends with a slash if it's meant to match a directory-like path
		// and the request path also has a slash there or is an exact match.
		if strings.HasSuffix(rulePatternPath, "/") {
			if strings.HasPrefix(requestPathNormalized, rulePatternPath) {
				match = true
			}
		} else {
			// If rule pattern doesn't end with a slash, it should match exactly that path segment
			// or that path segment followed by a slash.
			if requestPathNormalized == rulePatternPath || strings.HasPrefix(requestPathNormalized, rulePatternPath+"/") {
				match = true
			}
		}
	case "ip_address":
		if hostname == pattern { // IP addresses are often used as hostnames
			match = true
		}
	case "cidr":
		_, cidrNet, err := net.ParseCIDR(pattern)
		if err == nil {
			ip := net.ParseIP(hostname) // Hostname can be an IP
			if ip != nil && cidrNet.Contains(ip) {
				match = true
			}
		}
	}
	return match
}

// matchesGlobalExclusionRule checks if a request URL matches a global exclusion rule.
func matchesGlobalExclusionRule(requestURL *url.URL, rule models.ProxyExclusionRule) bool {
	if !rule.IsEnabled || requestURL == nil {
		return false
	}

	pattern := rule.Pattern
	hostname := requestURL.Hostname()
	path := requestURL.Path
	fullURL := requestURL.String()

	switch rule.RuleType {
	case "file_extension":
		// Ensure pattern starts with a dot if it's meant to be an extension
		if !strings.HasPrefix(pattern, ".") {
			pattern = "." + pattern
		}
		if strings.HasSuffix(strings.ToLower(path), strings.ToLower(pattern)) {
			return true
		}
	case "url_regex":
		// Consider compiling regexes once at startup if performance becomes an issue
		re, err := regexp.Compile(pattern)
		if err != nil {
			logger.ProxyError("Invalid regex pattern in global exclusion rule ID %s: %s", rule.ID, pattern)
			return false // Invalid regex shouldn't match anything
		}
		if re.MatchString(fullURL) {
			return true
		}
	case "domain":
		return strings.ToLower(hostname) == strings.ToLower(pattern)
	}
	return false
}

// isRequestEffectivelyInScope evaluates if a request URL is effectively in scope
// considering both IN and OUT OF SCOPE rules.
// OUT OF SCOPE rules take precedence.
func isRequestEffectivelyInScope(requestURL *url.URL, allRules []models.ScopeRule) bool {
	if requestURL == nil {
		return false
	}

	hostname := requestURL.Hostname()
	path := requestURL.Path

	for _, rule := range allRules {
		// Already filtered for is_in_scope by GetInScopeRulesForTarget
		pattern := rule.Pattern
		match := matchesRule(requestURL, hostname, path, rule)

		if match && !rule.IsInScope { // Matched an OUT OF SCOPE rule
			logger.ProxyDebug("Request URL '%s' matched OUT OF SCOPE rule: Type=%s, Pattern='%s'", requestURL.String(), rule.ItemType, pattern)
			return false // Explicitly out of scope
		}
	}

	// If no OUT OF SCOPE rules matched, check IN SCOPE rules
	// Separate IN_SCOPE rules for clarity in logic below
	var inScopeRules []models.ScopeRule
	for _, rule := range allRules {
		if rule.IsInScope {
			inScopeRules = append(inScopeRules, rule)
		}
	}

	// If there are no IN SCOPE rules defined for the target, consider everything in scope (default allow)
	if len(inScopeRules) == 0 {
		logger.ProxyDebug("Request URL '%s' is IN SCOPE by default (no specific IN_SCOPE rules defined and no OUT_OF_SCOPE match).", requestURL.String())
		return true
	}

	// If IN SCOPE rules exist, at least one must match
	for _, rule := range inScopeRules {
		if matchesRule(requestURL, hostname, path, rule) {
			logger.ProxyDebug("Request URL '%s' matched IN SCOPE rule: Type=%s, Pattern='%s'", requestURL.String(), rule.ItemType, rule.Pattern)
			return true
		}
	}
	logger.ProxyDebug("Request URL '%s' did not match any IN_SCOPE rules (and no OUT_OF_SCOPE match). Effectively OUT OF SCOPE.", requestURL.String())
	return false
}

func StartMitmProxy(port string, cliTargetID int64, caCertPath string, caKeyPath string) error {
	if err := loadCA(caCertPath, caKeyPath); err != nil {
		return fmt.Errorf("could not load CA certificate/key: %w. Please run 'proxy init-ca' or check config.", err)
	}

	setGoproxyCA(&tls.Certificate{
		Certificate: [][]byte{caCert.Raw},
		PrivateKey:  caKey,
		Leaf:        caCert,
	})

	// Determine active target ID and load its scope rules
	scopeMu.Lock()
	if cliTargetID != 0 {
		activeTargetID = &cliTargetID
		logger.ProxyInfo("Proxy started with explicit target ID from CLI: %d", *activeTargetID)
	} else {
		targetIDStr, err := database.GetSetting(models.CurrentTargetIDKey)
		if err != nil {
			logger.ProxyError("Failed to read current target ID from database settings: %v. Proxy will log traffic unassociated.", err)
		} else if targetIDStr != "" && targetIDStr != "0" {
			dbTargetID, parseErr := strconv.ParseInt(targetIDStr, 10, 64)
			if parseErr != nil {
				logger.ProxyError("Failed to parse current target ID '%s' from database: %v. Proxy will log traffic unassociated.", targetIDStr, parseErr)
			} else if dbTargetID != 0 {
				activeTargetID = &dbTargetID
				logger.ProxyInfo("Proxy using current target ID from database: %d", *activeTargetID)
			} else {
				logger.ProxyInfo("No current target ID set in database (or it's 0). Proxy will log traffic unassociated.")
			}
		} else {
			logger.ProxyInfo("No current target ID set in database. Proxy will log traffic unassociated.")
		}

	}

	if activeTargetID != nil && *activeTargetID != 0 {
		var err error
		allActiveScopeRules, err = database.GetAllScopeRulesForTarget(*activeTargetID) // Use new function
		if err != nil {
			logger.ProxyError("Failed to load all scope rules for target %d: %v. Logging for this target might be affected.", *activeTargetID, err)
		} else {
			logger.ProxyInfo("Loaded %d total scope rules for target %d.", len(allActiveScopeRules), *activeTargetID)
		}
	}
	scopeMu.Unlock()

	// Load global exclusion rules
	var errLoadExclusions error
	globalExclusionRules, errLoadExclusions = database.GetProxyExclusionRules()
	if errLoadExclusions != nil {
		logger.ProxyError("Failed to load global proxy exclusion rules: %v. Proxy will not apply global exclusions.", errLoadExclusions)
		globalExclusionRules = []models.ProxyExclusionRule{} // Ensure it's an empty slice
	} else {
		logger.ProxyInfo("Loaded %d global proxy exclusion rules.", len(globalExclusionRules))
	}

	// Parse the configured Synack URL once on startup
	if config.AppConfig.Synack.TargetsURL != "" {
		parsedSynackTargetURL, parseURLErr = url.Parse(config.AppConfig.Synack.TargetsURL)
		if parseURLErr != nil {
			logger.Error("Failed to parse configured Synack TargetsURL '%s': %v. Synack processing disabled.", config.AppConfig.Synack.TargetsURL, parseURLErr)
			parsedSynackTargetURL = nil // Ensure it's nil on error
		} else {
			logger.ProxyInfo("Parsed Synack Target URL for matching: Scheme=%s, Host=%s, Path=%s", parsedSynackTargetURL.Scheme, parsedSynackTargetURL.Host, parsedSynackTargetURL.Path)
		}
	} else {
		parsedSynackTargetURL = nil // Ensure it's nil if not configured
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Logger = log.New(io.Discard, "", 0)

	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		muSession.Lock()
		sessionIsHTTPS[ctx.Session] = true
		muSession.Unlock()
		logger.ProxyDebug("HandleConnect for session %d, host %s", ctx.Session, host)
		return &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxy.GoproxyCa)}, host
	}))

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			startTime := time.Now()

			// Check global exclusion rules first
			for _, rule := range globalExclusionRules {
				if matchesGlobalExclusionRule(r.URL, rule) {
					logger.ProxyInfo("REQ: %s %s - GLOBALLY EXCLUDED by rule ID %s (Type: %s, Pattern: %s). Skipping.", r.Method, r.URL.String(), rule.ID, rule.RuleType, rule.Pattern)
					return r, nil // Pass through without logging or further processing by our handlers
				}
			}

			scopeMu.RLock()
			// Determine if the current request URL matches the configured Synack target list URL
			isSynackTargetListURL := false
			if parsedSynackTargetURL != nil && r.URL != nil {
				reqPathNorm := strings.TrimRight(r.URL.Path, "/")
				configPathNorm := strings.TrimRight(parsedSynackTargetURL.Path, "/")
				if r.URL.Scheme == parsedSynackTargetURL.Scheme &&
					r.URL.Hostname() == parsedSynackTargetURL.Hostname() &&
					reqPathNorm == configPathNorm {
					isSynackTargetListURL = true
				}
			}

			currentTargetIDForLog := activeTargetID     // Use the snapshot
			currentAllScopeRules := allActiveScopeRules // Use the renamed variable
			scopeMu.RUnlock()

			// If there's an active target context, check scope
			if currentTargetIDForLog != nil && *currentTargetIDForLog != 0 {
				if !isRequestEffectivelyInScope(r.URL, currentAllScopeRules) { // Call updated function
					logger.ProxyDebug("REQ: %s %s (HTTPS: %t) - OUT OF SCOPE for active target %d.", r.Method, r.URL.String(), sessionIsHTTPS[ctx.Session], *currentTargetIDForLog)
					// If it's the Synack target list URL, we still want to process it for token capture and list data,
					// but we won't associate its http_traffic_log entry with the currently active (and mismatched) target.
					if !isSynackTargetListURL {
						return r, nil // Not Synack list and out of scope, so skip fully.
					}
					logger.ProxyDebug("Synack target list URL is out of scope for active target %d, but will be processed with no target association.", *currentTargetIDForLog)
					currentTargetIDForLog = nil // It's the Synack list, but out of scope for current target; log with nil TargetID.
				} else {
					// In scope for the active target
					logger.ProxyDebug("REQ: %s %s (HTTPS: %t) - IN SCOPE for active target %d.", r.Method, r.URL.String(), sessionIsHTTPS[ctx.Session], *currentTargetIDForLog)
				}
			} else if isSynackTargetListURL {
				logger.ProxyDebug("Processing Synack target list URL with no active target.")
			}

			reqBodyBytes, errReadReq := io.ReadAll(r.Body)
			if errReadReq != nil {
				logger.ProxyError("REQ: Error reading request body for %s %s: %v", r.Method, r.URL.String(), errReadReq)
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))

			reqHeadersMap := make(map[string][]string)
			for k, v := range r.Header {
				reqHeadersMap[k] = v
			}
			reqHeadersJson, _ := json.Marshal(reqHeadersMap)

			muSession.Lock()
			isCurrentSessionHTTPS := sessionIsHTTPS[ctx.Session]
			muSession.Unlock()

			if r.Method == http.MethodConnect {
				isCurrentSessionHTTPS = true
			}

			requestData := &models.HTTPTrafficLog{
				TargetID:           currentTargetIDForLog, // Use the determined target ID
				Timestamp:          startTime,
				RequestMethod:      r.Method,
				RequestURL:         r.URL.String(),
				RequestHTTPVersion: r.Proto,
				RequestHeaders:     string(reqHeadersJson),
				RequestBody:        reqBodyBytes,
				ClientIP:           r.RemoteAddr,
				IsHTTPS:            isCurrentSessionHTTPS,
			}

			// Wrap requestData and potentially auth token for Synack calls
			ctxData := &proxyRequestContextData{TrafficLog: requestData}

			// If this is the Synack target list request, try to capture its Authorization header
			if isSynackTargetListURL { // Use the pre-calculated boolean
				authToken := r.Header.Get("Authorization")
				if strings.HasPrefix(authToken, "Bearer ") {
					ctxData.SynackAuthToken = authToken
					logger.ProxyInfo("Captured Authorization token for Synack target list request.")
				} else if authToken != "" { // The Authorization header was present but not a Bearer token
					logger.ProxyInfo("WARNING: Found Authorization header for Synack target list, but it's not a Bearer token.")
				} // No "else" here, token might be absent.
			}
			ctx.UserData = ctxData

			logger.ProxyInfo("REQ: %s %s (HTTPS: %t)", r.Method, r.URL.String(), isCurrentSessionHTTPS)
			return r, nil
		})

	proxy.OnResponse().DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			// If UserData is nil, it means the request was deemed out of scope.
			// Or if it's not our custom context data type.
			pCtxData, ok := ctx.UserData.(*proxyRequestContextData)
			if !ok || pCtxData == nil || pCtxData.TrafficLog == nil {
				logger.ProxyDebug("RESP: Skipping response processing for out-of-scope or non-TrafficLog request: %s %s", ctx.Req.Method, ctx.Req.URL.String())
				return resp
			}
			requestData := pCtxData.TrafficLog // This is the *models.HTTPTrafficLog

			if resp == nil {
				logger.ProxyError("RESP: Nil response for %s %s", ctx.Req.Method, ctx.Req.URL.String())
				requestData.ResponseStatusCode = 0
				logHttpTraffic(requestData)
				return resp
			}

			respBodyBytes, errReadResp := io.ReadAll(resp.Body)
			if errReadResp != nil {
				logger.ProxyError("RESP: Error reading response body for %s %s: %v", ctx.Req.Method, ctx.Req.URL.String(), errReadResp)
			}
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewBuffer(respBodyBytes))

			respHeadersMap := make(map[string][]string)
			for k, v := range resp.Header {
				respHeadersMap[k] = v
			}
			respHeadersJson, _ := json.Marshal(respHeadersMap)

			duration := time.Since(requestData.Timestamp)

			requestData.ResponseStatusCode = resp.StatusCode
			requestData.ResponseReasonPhrase = strings.TrimPrefix(resp.Status, fmt.Sprintf("%d ", resp.StatusCode))
			requestData.ResponseHTTPVersion = resp.Proto
			requestData.ResponseHeaders = string(respHeadersJson)
			requestData.ResponseBody = respBodyBytes
			requestData.ResponseContentType = resp.Header.Get("Content-Type")
			requestData.ResponseBodySize = int64(len(respBodyBytes))
			requestData.DurationMs = duration.Milliseconds()

			if strings.Contains(strings.ToLower(requestData.ResponseContentType), "text/html") {
				requestData.IsPageCandidate = true
			}

			logHttpTraffic(requestData)

			// MODIFIED Synack Target List Processing Check
			isSynackTargetListResp := false
			var reqPathNorm, configPathNorm string // For debug logging

			if parsedSynackTargetURL != nil && ctx.Req != nil && ctx.Req.URL != nil {
				reqPathNorm = strings.TrimRight(ctx.Req.URL.Path, "/")
				configPathNorm = strings.TrimRight(parsedSynackTargetURL.Path, "/")
				if ctx.Req.URL.Scheme == parsedSynackTargetURL.Scheme &&
					ctx.Req.URL.Hostname() == parsedSynackTargetURL.Hostname() &&
					reqPathNorm == configPathNorm {
					isSynackTargetListResp = true
				}
			}

			if isSynackTargetListResp && // Use the boolean determined above
				resp.StatusCode == http.StatusOK &&
				strings.Contains(strings.ToLower(requestData.ResponseContentType), "application/json") {

				authTokenForAnalytics := pCtxData.SynackAuthToken // Get the captured token
				logger.ProxyInfo("Synack target list URL detected: %s. Using auth token: %t", ctx.Req.URL.String(), authTokenForAnalytics != "")
				go processSynackTargetList(respBodyBytes, authTokenForAnalytics)
			}
			//	} else if parsedSynackTargetURL != nil && ctx.Req != nil && ctx.Req.URL != nil && !isSynackTargetListResp { // Log only if it wasn't a match but conditions were met
			//logger.ProxyDebug("URL did not match Synack target list: ReqPathNorm=[%s], ConfigPathNorm=[%s], ReqFull=[%s], Status=%d, ContentType=%s",
			//	reqPathNorm, configPathNorm, ctx.Req.URL.String(),
			//	resp.StatusCode, requestData.ResponseContentType)
			//}

			logger.ProxyInfo("RESP: %d %s for %s %s (Size: %d, Duration: %s)", resp.StatusCode, requestData.ResponseContentType, ctx.Req.Method, ctx.Req.URL.String(), requestData.ResponseBodySize, duration)

			muSession.Lock()
			delete(sessionIsHTTPS, ctx.Session)
			muSession.Unlock()

			return resp
		})

	logger.ProxyInfo("MITM Proxy server starting on :%s", port)
	return http.ListenAndServe(":"+port, proxy)
}

// logHttpTraffic function remains the same
func logHttpTraffic(logEntry *models.HTTPTrafficLog) {
	if database.DB == nil {
		logger.ProxyError("logHttpTraffic: Database is not initialized.")
		return
	}
	_, err := database.DB.Exec(`INSERT INTO http_traffic_log (
		target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
		response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
		response_content_type, response_body_size, duration_ms, client_ip, is_https, is_page_candidate, notes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logEntry.TargetID, logEntry.Timestamp, logEntry.RequestMethod, logEntry.RequestURL,
		logEntry.RequestHTTPVersion, logEntry.RequestHeaders, logEntry.RequestBody,
		logEntry.ResponseStatusCode, logEntry.ResponseReasonPhrase, logEntry.ResponseHTTPVersion,
		logEntry.ResponseHeaders, logEntry.ResponseBody, logEntry.ResponseContentType,
		logEntry.ResponseBodySize, logEntry.DurationMs, logEntry.ClientIP, logEntry.IsHTTPS,
		logEntry.IsPageCandidate, logEntry.Notes,
	)
	if err != nil {
		logger.ProxyError("DB log error on response for %s %s: %v", logEntry.RequestMethod, logEntry.RequestURL, err)
	}
}

// Example structure for an individual finding from a hypothetical Synack API
type rawSynackFindingItem struct {
	// Fields from "exploitable_locations"
	Type       string `json:"type"`       // e.g., "url"
	Value      string `json:"value"`      // This will be our VulnerabilityURL
	CreatedAt  int64  `json:"created_at"` // Unix timestamp
	Status     string `json:"status"`     // e.g., "fixed"
	OutOfScope *bool  `json:"outOfScope"` // Pointer to handle null

	// Fields that might not be present in exploitable_locations, but are in models.SynackFinding
	// These will likely remain empty/zero unless mapped from elsewhere or if the actual structure is richer.
	ID          string          `json:"id"`       // If Synack provides a unique finding ID here
	Title       string          `json:"title"`    // If a title is provided
	Category    string          `json:"category"` // If a category is provided
	Severity    string          `json:"severity"` // If severity is provided
	FullDetails json.RawMessage `json:"-"`        // To store the whole finding JSON, we'll marshal the item itself
}

// processSynackTargetList function now accepts authToken
func processSynackTargetList(jsonData []byte, authToken string) {
	logger.ProxyInfo("Processing Synack target list JSON (%d bytes)...", len(jsonData))

	var rawTargets []map[string]interface{}
	httpClient := &http.Client{Timeout: 20 * time.Second} // Client for fetching analytics

	if !config.AppConfig.Synack.AnalyticsEnabled {
		logger.ProxyInfo("Synack target analytics fetching is disabled in config.")
	}

	arrayPath := config.AppConfig.Synack.TargetsArrayPath
	if arrayPath == "" {
		arrayPath = "@this"
	}

	result := gjson.GetBytes(jsonData, arrayPath)
	if !result.IsArray() {
		logger.ProxyError("Synack target list: Expected an array at path '%s', but got: %s", arrayPath, result.Type.String())
		if config.AppConfig.Synack.TargetsArrayPath != "" {
			resultRoot := gjson.ParseBytes(jsonData)
			if resultRoot.IsArray() {
				logger.ProxyInfo("Synack target list: Falling back to parsing root as array.")
				result = resultRoot
			} else {
				logger.ProxyError("Synack target list: Root is also not an array. Type: %s", resultRoot.Type.String())
				return
			}
		} else {
			return
		}
	}

	err := json.Unmarshal([]byte(result.Raw), &rawTargets)
	if err != nil {
		logger.ProxyError("Synack target list: Error unmarshalling into []map[string]interface{}: %v", err)
		return
	}

	if len(rawTargets) == 0 {
		logger.ProxyInfo("Synack target list: No targets found in JSON after parsing.")
		if err := database.DeactivateMissingSynackTargets([]string{}, time.Now()); err != nil {
			logger.ProxyError("Synack target list: Error deactivating all targets on empty list: %v", err)
		}
		return
	}

	var currentSeenIDs []string
	for _, targetMap := range rawTargets {
		idField := config.AppConfig.Synack.TargetIDField
		if idField == "" {
			idField = "id"
		}
		synackID, ok := targetMap[idField].(string)
		if !ok {
			if idNum, numOk := targetMap[idField].(float64); numOk {
				synackID = fmt.Sprintf("%.0f", idNum)
				ok = true
			} else {
				logger.ProxyError("Synack target list: Target missing or has invalid type for configured ID field '%s'. Skipping. Data: %+v", idField, targetMap)
				continue
			}
		}
		currentSeenIDs = append(currentSeenIDs, synackID)

		dbID, errUpsert := database.UpsertSynackTarget(targetMap)
		if errUpsert != nil {
			logger.ProxyError("Synack target list: Error upserting target with Synack ID '%s': %v", synackID, err)
			continue // Skip analytics/findings if target upsert failed
		}

		// --- Analytics Category Processing (Current Logic - may need adjustment/removal) ---
		// Successfully upserted (dbID is valid), now try to fetch analytics if enabled
		if config.AppConfig.Synack.AnalyticsEnabled && config.AppConfig.Synack.AnalyticsBaseURL != "" && config.AppConfig.Synack.AnalyticsPathPattern != "" {
			analyticsURL := fmt.Sprintf(config.AppConfig.Synack.AnalyticsBaseURL+config.AppConfig.Synack.AnalyticsPathPattern, synackID)
			logger.ProxyInfo("Fetching analytics for Synack target '%s' from URL: %s", synackID, analyticsURL)

			req, err := http.NewRequest("GET", analyticsURL, nil)
			if err != nil {
				logger.ProxyError("Synack Analytics: Error creating request for target '%s': %v", synackID, err)
				// continue // Don't skip findings processing if analytics categories fail
			}
			if authToken != "" {
				req.Header.Set("Authorization", authToken)
				logger.ProxyDebug("Synack Analytics: Added Authorization header to request for target '%s'", synackID)
			}

			// Check if analytics already exist and are "fresh" (for now, just exist)
			lastFetchTime, errCheck := database.GetSynackTargetAnalyticsFetchTime(dbID)
			if errCheck != nil {
				logger.ProxyError("Synack Analytics: Error checking existing analytics for target DB_ID %d (Synack ID %s): %v", dbID, synackID, errCheck)
				// Decide if we should proceed. For now, let's try to fetch if checking failed.
			}

			// For now, if analytics exist at all (lastFetchTime is not nil), don't refetch.
			// Later, a freshness check can be added: e.g., if lastFetchTime != nil && time.Since(*lastFetchTime) < 24*time.Hour
			if lastFetchTime != nil {
				logger.ProxyInfo("Synack Analytics: Analytics data already exists for target DB_ID %d (Synack ID %s), fetched at %s. Skipping fetch.", dbID, synackID, lastFetchTime.Format(time.RFC3339))
			} else {
				// Rate limit the API calls using the ticker.
				// This will also space out the subsequent database operations for each target.
				logger.ProxyDebug("Synack Analytics: Waiting for analyticsFetchTicker for target DB_ID %d (Synack ID %s)...", dbID, synackID)
				<-analyticsFetchTicker.C
				logger.ProxyInfo("Synack Analytics: No existing analytics found for target DB_ID %d (Synack ID %s) or fetch time is nil. Proceeding to fetch.", dbID, synackID)
				resp, errDo := httpClient.Do(req)
				if errDo != nil {
					logger.ProxyError("Synack Analytics: Error fetching analytics for target DB_ID %d (Synack ID %s) from %s: %v", dbID, synackID, analyticsURL, errDo)
					// continue
				}
				defer resp.Body.Close()

				body, errRead := io.ReadAll(resp.Body)
				if errRead != nil {
					logger.ProxyError("Synack Analytics: Error reading analytics response body for target DB_ID %d (Synack ID %s) (status %d): %v", dbID, synackID, resp.StatusCode, errRead)
					// continue
				}
				logger.ProxyInfo("Synack Analytics: Received status %d for target DB_ID %d (Synack ID %s). Response (first 500 chars): %s", resp.StatusCode, dbID, synackID, string(body[:min(len(body), 500)]))

				// Attempt to extract the array of categories, assuming it's nested.
				// Common key names are "categories", "data", "results". We'll try "categories".
				// Based on new info, the key is "value"
				analyticsArrayPath := "value"
				categoriesResult := gjson.GetBytes(body, analyticsArrayPath)

				if !categoriesResult.Exists() {
					logger.ProxyError("Synack Analytics: Expected key '%s' not found in analytics JSON response for target DB_ID %d (Synack ID %s). The response might be a direct array or use a different key. Response (first 500 chars): %s", analyticsArrayPath, dbID, synackID, string(body[:min(len(body), 500)]))
				} else if !categoriesResult.IsArray() {
					logger.ProxyError("Synack Analytics: Value at key '%s' in analytics JSON is not an array (type: %s) for target DB_ID %d (Synack ID %s). Response (first 500 chars): %s", analyticsArrayPath, categoriesResult.Type.String(), dbID, synackID, string(body[:min(len(body), 500)]))
				} else {
					// Define an intermediate struct to match the JSON structure of items in the "value" array
					var rawAnalyticsItems []struct {
						Categories           []string        `json:"categories"`
						ExploitableLocations json.RawMessage `json:"exploitable_locations"` // We need to count items here
						// Other fields like count, total_amount_paid, average_amount_paid from the old structure are not in the new example.
						// If they exist in other responses, this struct might need to be more flexible or have more fields.
					}

					if errUnmarshal := json.Unmarshal([]byte(categoriesResult.Raw), &rawAnalyticsItems); errUnmarshal != nil {
						logger.ProxyError("Synack Analytics: Error unmarshalling analytics items from key '%s' for target DB_ID %d (Synack ID %s): %v", analyticsArrayPath, dbID, synackID, errUnmarshal)
					} else {
						aggregatedCategories := make(map[string]int64) // Map to store aggregated counts
						for _, rawItem := range rawAnalyticsItems {

							// Count items in exploitable_locations for this category group
							var exploitableLocationsArray []interface{}
							if rawItem.ExploitableLocations != nil {
								if errParseEL := json.Unmarshal(rawItem.ExploitableLocations, &exploitableLocationsArray); errParseEL != nil {
									logger.Error("Synack Analytics (Proxy): Could not parse exploitable_locations for category group (target DB_ID %d): %v", dbID, errParseEL)
								}
							}
							findingCountInGroup := int64(len(exploitableLocationsArray))

							if len(rawItem.Categories) > 0 {
								for _, categoryName := range rawItem.Categories {
									aggregatedCategories[categoryName] += findingCountInGroup
								}
							} // No "else" needed, if no categories, we just don't add anything for this rawItem.

							// --- Individual Findings Processing for THIS rawItem ---
							if config.AppConfig.Synack.FindingsEnabled {
								logger.ProxyInfo("Synack Findings: Processing 'exploitable_locations' for target DB_ID %d, category group with categories: %v", dbID, rawItem.Categories)
								if config.AppConfig.Synack.FindingsArrayPathInAnalyticsJson != "" {
									logger.ProxyInfo("Warning: Synack Findings: Config key 'findings_array_path_in_analytics_json' is set but currently ignored. Findings are sourced from 'exploitable_locations' within each category group.")
								}

								if rawItem.ExploitableLocations != nil {
									var exploitableLocationsList []rawSynackFindingItem // This struct should match the items in exploitable_locations

									// Log the raw JSON of exploitable_locations before attempting to unmarshal
									logger.ProxyDebug("Synack Findings: Raw exploitable_locations JSON for target DB_ID %d (first 500 chars): %s", dbID, string(rawItem.ExploitableLocations[:min(len(rawItem.ExploitableLocations), 500)]))

									if errUnmarshalEL := json.Unmarshal(rawItem.ExploitableLocations, &exploitableLocationsList); errUnmarshalEL != nil {
										logger.ProxyError("Synack Findings: Error unmarshalling exploitable_locations for category group (target DB_ID %d): %v. Raw JSON: %s", dbID, errUnmarshalEL, string(rawItem.ExploitableLocations[:min(len(rawItem.ExploitableLocations), 200)]))
									} else {
										logger.ProxyInfo("Synack Findings: Found %d exploitable_locations for this category group (target DB_ID %d).", len(exploitableLocationsList), dbID) // Corrected line
										if len(exploitableLocationsList) == 0 && len(rawItem.ExploitableLocations) > 2 {                                                                    // >2 to avoid logging for empty array "[]"
											logger.ProxyInfo("Warning: Synack Findings: Unmarshalled 0 exploitable_locations, but raw JSON was not empty for target DB_ID %d. Check rawSynackFindingItem struct against API response.", dbID)
										}
										primaryCategoryName := ""
										if len(rawItem.Categories) > 0 {
											primaryCategoryName = rawItem.Categories[0]
										}

										for i, exploitableLocation := range exploitableLocationsList {
											logger.ProxyDebug("Synack Findings: Processing exploitable_location #%d: URL='%s', Status='%s', CreatedAt=%d", i+1, exploitableLocation.Value, exploitableLocation.Status, exploitableLocation.CreatedAt)

											findingToStore := models.SynackFinding{
												SynackTargetDBID: dbID,
												SynackFindingID:  exploitableLocation.Value,                               // Use URL as the finding ID
												Title:            fmt.Sprintf("Finding at %s", exploitableLocation.Value), // Construct a title
												CategoryName:     primaryCategoryName,
												// Severity is not in the exploitable_locations example
												Status:           exploitableLocation.Status,
												VulnerabilityURL: exploitableLocation.Value,
												// AmountPaid is not in the example
											}

											// Marshal the exploitableLocation itself to store as JSON details
											rawExploitableLocationJsonBytes, errJson := json.Marshal(exploitableLocation)
											if errJson == nil {
												findingToStore.RawJSONDetails = string(rawExploitableLocationJsonBytes)
											} else {
												logger.ProxyError("Synack Findings: Could not marshal exploitableLocation to JSON for finding URL '%s': %v", exploitableLocation.Value, errJson)
											}

											if exploitableLocation.CreatedAt > 0 {
												t := time.Unix(exploitableLocation.CreatedAt, 0).UTC()
												findingToStore.ReportedAt = &t
											}

											logger.ProxyDebug("Synack Findings: Attempting to upsert finding: TargetDBID=%d, FindingID(URL)='%s', Category='%s', Status='%s'",
												findingToStore.SynackTargetDBID, findingToStore.SynackFindingID, findingToStore.CategoryName, findingToStore.Status)

											insertedDBID, errStoreFinding := database.UpsertSynackFinding(findingToStore)
											if errStoreFinding != nil {
												logger.ProxyError("Synack Findings: Error upserting finding (URL: '%s') for target DB_ID %d: %v", exploitableLocation.Value, dbID, errStoreFinding)
											} else {
												logger.ProxyInfo("Synack Findings: Successfully upserted finding (URL: '%s'), DB ID: %d, for target DB_ID %d.", exploitableLocation.Value, insertedDBID, dbID)
											}
										}
									}
								} else { // rawItem.ExploitableLocations was nil
									logger.ProxyDebug("Synack Findings: No 'exploitable_locations' data for this category group (target DB_ID %d).", dbID)
								}
							} else {
								logger.ProxyInfo("Synack Findings: Individual findings processing is DISABLED in config for target DB_ID %d.", dbID)
							}
							// --- End of Individual Findings Processing for this rawItem ---
						} // End loop through rawAnalyticsItems

						// Convert aggregated map to slice for storing
						var finalAnalyticsCategories []models.SynackTargetAnalyticsCategory
						// The following block was an erroneous comment and a leftover variable declaration.
						for name, count := range aggregatedCategories {
							finalAnalyticsCategories = append(finalAnalyticsCategories, models.SynackTargetAnalyticsCategory{CategoryName: name, Count: count})
						}

						if errStore := database.StoreSynackTargetAnalytics(dbID, finalAnalyticsCategories); errStore != nil {
							logger.ProxyError("Synack Analytics: Error storing analytics for target DB_ID %d (Synack ID %s): %v", dbID, synackID, errStore)
						} else {
							logger.ProxyInfo("Synack Analytics: Successfully stored %d analytics categories for target DB_ID %d (Synack ID %s)", len(finalAnalyticsCategories), dbID, synackID)
						}
					} // End else for categoriesResult.IsArray()
				} // End if resp.StatusCode == http.StatusOK
			} // End else for lastFetchTime != nil (meaning, proceed to fetch)
		}
	} // End loop through rawTargets

	if len(currentSeenIDs) > 0 {
		if err := database.DeactivateMissingSynackTargets(currentSeenIDs, time.Now()); err != nil {
			logger.ProxyError("Synack target list: Error deactivating missing targets: %v", err)
		}
	} else {
		logger.ProxyInfo("Synack target list: No valid target IDs found in the current list, skipping deactivation step.")
	}

	logger.ProxyInfo("Finished processing Synack target list. Saw %d targets.", len(currentSeenIDs))
}

// Add to your config.AppConfig struct (e.g., in config/config.go)
// SynackConfig struct {
//    ... other synack fields ...
//    FindingsEnabled      bool   `mapstructure:"findings_enabled"`
//    FindingsBaseURL      string `mapstructure:"findings_base_url"`      // e.g., "https://platform.synack.com"
//    FindingsPathPattern  string `mapstructure:"findings_path_pattern"`  // e.g., "/api/extension/v1/listings/%s/findings"
// }
