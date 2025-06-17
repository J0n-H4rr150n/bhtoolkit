package core

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log" // Standard log package for goproxy.Logger config
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"toolkit/config"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"strconv"

	"github.com/elazarl/goproxy"
	"github.com/tidwall/gjson"
)

var (
	caCert                *x509.Certificate
	caKey                 *rsa.PrivateKey
	sessionIsHTTPS        = make(map[int64]bool)
	muSession             sync.Mutex
	parsedSynackTargetURL *url.URL
	parseURLErr           error

	activeTargetID        *int64
	allActiveScopeRules   []models.ScopeRule
	scopeMu               sync.RWMutex
	globalExclusionRules  []models.ProxyExclusionRule
	activeRecordingPageID *int64
	// Added for rate limiting analytics calls
	analyticsFetchTicker *time.Ticker
)

// proxyRequestContextData holds data passed between request and response handlers via ctx.UserData
type proxyRequestContextData struct {
	TrafficLog      *models.HTTPTrafficLog
	SynackAuthToken string // To store the Bearer token from Synack target list request
}

// SetActivePageSitemapRecordingID is called by the Page Sitemap feature to indicate a recording has started.
func SetActivePageSitemapRecordingID(pageID int64) {
	// Using scopeMu for simplicity, or create a new mutex for this
	scopeMu.Lock()
	defer scopeMu.Unlock()
	// Allow clearing by passing 0
	if pageID == 0 {
		activeRecordingPageID = nil
		logger.ProxyInfo("Page Sitemap recording stopped (or no active page).")
	} else {
		activeRecordingPageID = &pageID
		logger.ProxyInfo("Page Sitemap recording started for Page ID: %d", pageID)
	}
}

// ClearActivePageSitemapRecordingID is called when a recording stops.
func ClearActivePageSitemapRecordingID() {
	scopeMu.Lock()
	defer scopeMu.Unlock()
	activeRecordingPageID = nil
	logger.ProxyInfo("Page Sitemap recording explicitly stopped.")
}

func init() {
	// Note: config.AppConfig might not be fully populated during Go's init phase.
	// It's better to parse this lazily or after config is loaded.
	// We will parse it inside StartMitmProxy instead.
	analyticsFetchTicker = time.NewTicker(1 * time.Second)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

func matchesRule(requestURL *url.URL, hostname, path string, rule models.ScopeRule) bool {
	pattern := rule.Pattern
	match := false

	switch rule.ItemType {
	case "domain":
		if strings.HasPrefix(pattern, "*.") {
			domainPart := strings.TrimPrefix(pattern, "*.")
			if hostname == domainPart || strings.HasSuffix(hostname, "."+domainPart) {
				match = true
			}
		} else if hostname == pattern {
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
		if rulePatternPath != "" && !strings.HasPrefix(rulePatternPath, "/") {
			rulePatternPath = "/" + rulePatternPath
		}

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
		// IP addresses are often used as hostnames
		if hostname == pattern {
			match = true
		}
	case "cidr":
		_, cidrNet, err := net.ParseCIDR(pattern)
		if err == nil {
			ip := net.ParseIP(hostname)
			if ip != nil && cidrNet.Contains(ip) {
				match = true
			}
		}
	}
	return match
}

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
			// Invalid regex shouldn't match anything
			return false
		}
		if re.MatchString(fullURL) {
			return true
		}
	case "domain":
		return strings.ToLower(hostname) == strings.ToLower(pattern)
	}
	return false
}

func isRequestEffectivelyInScope(requestURL *url.URL, allRules []models.ScopeRule) bool {
	if requestURL == nil {
		return false
	}

	hostname := requestURL.Hostname()
	path := requestURL.Path

	for _, rule := range allRules {
		// allRules contains both IN and OUT rules.
		// The logic correctly handles this by checking rule.IsInScope below.
		pattern := rule.Pattern
		match := matchesRule(requestURL, hostname, path, rule)

		if match && !rule.IsInScope {
			logger.ProxyDebug("Request URL '%s' matched OUT OF SCOPE rule: Type=%s, Pattern='%s'", requestURL.String(), rule.ItemType, pattern)
			return false
		}
	}

	// Separate IN_SCOPE rules for clarity in logic below
	var inScopeRules []models.ScopeRule
	for _, rule := range allRules {
		if rule.IsInScope {
			inScopeRules = append(inScopeRules, rule)
		}
	}

	// If there are no IN SCOPE rules defined for the target, consider everything in scope (default allow).
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
	// If no IN_SCOPE rules matched (and no OUT_OF_SCOPE rules matched earlier), it's effectively out of scope.
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
		allActiveScopeRules, err = database.GetAllScopeRulesForTarget(*activeTargetID)
		if err != nil {
			logger.ProxyError("Failed to load all scope rules for target %d: %v. Logging for this target might be affected.", *activeTargetID, err)
		} else {
			logger.ProxyInfo("Loaded %d total scope rules for target %d.", len(allActiveScopeRules), *activeTargetID)
		}
	}
	scopeMu.Unlock()

	var errLoadExclusions error
	globalExclusionRules, errLoadExclusions = database.GetProxyExclusionRules()
	if errLoadExclusions != nil {
		logger.ProxyError("Failed to load global proxy exclusion rules: %v. Proxy will not apply global exclusions.", errLoadExclusions)
		// Ensure it's an empty slice
		globalExclusionRules = []models.ProxyExclusionRule{}
	} else {
		logger.ProxyInfo("Loaded %d global proxy exclusion rules.", len(globalExclusionRules))
	}

	if config.AppConfig.Synack.TargetsURL != "" {
		parsedSynackTargetURL, parseURLErr = url.Parse(config.AppConfig.Synack.TargetsURL)
		if parseURLErr != nil {
			logger.Error("Failed to parse configured Synack TargetsURL '%s': %v. Synack processing disabled.", config.AppConfig.Synack.TargetsURL, parseURLErr)
			parsedSynackTargetURL = nil
		} else {
			logger.ProxyInfo("Parsed Synack Target URL for matching: Scheme=%s, Host=%s, Path=%s", parsedSynackTargetURL.Scheme, parsedSynackTargetURL.Host, parsedSynackTargetURL.Path)
		}
	} else {
		parsedSynackTargetURL = nil
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

			for _, rule := range globalExclusionRules {
				if matchesGlobalExclusionRule(r.URL, rule) {
					logger.ProxyInfo("REQ: %s %s - GLOBALLY EXCLUDED by rule ID %s (Type: %s, Pattern: %s). Skipping.", r.Method, r.URL.String(), rule.ID, rule.RuleType, rule.Pattern)
					return r, nil
				}
			}

			scopeMu.RLock()
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

			currentTargetIDForLog := activeTargetID
			currentAllScopeRules := allActiveScopeRules
			scopeMu.RUnlock()

			if currentTargetIDForLog != nil && *currentTargetIDForLog != 0 {
				if !isRequestEffectivelyInScope(r.URL, currentAllScopeRules) {
					logger.ProxyDebug("REQ: %s %s (HTTPS: %t) - OUT OF SCOPE for active target %d.", r.Method, r.URL.String(), sessionIsHTTPS[ctx.Session], *currentTargetIDForLog)
					if !isSynackTargetListURL {
						return r, nil
					}
					logger.ProxyDebug("Synack target list URL is out of scope for active target %d, but will be processed with no target association.", *currentTargetIDForLog)
					currentTargetIDForLog = nil
				} else {
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

			serverSeenURL := r.URL.String()
			var requestFullURLWithFragment sql.NullString

			// Ensure this is being set
			if toolkitFullURL := r.Header.Get("X-Toolkit-Full-URL"); toolkitFullURL != "" && r.Header.Get("X-Toolkit-Initiated") == "true" {
				requestFullURLWithFragment.String = toolkitFullURL
				requestFullURLWithFragment.Valid = true
				logger.ProxyDebug("OnRequest: Populated requestFullURLWithFragment: %+v", requestFullURLWithFragment)
				logger.ProxyDebug("X-Toolkit-Full-URL found: %s. Server-seen URL: %s", toolkitFullURL, serverSeenURL)
			}

			logSource := "mitmproxy"
			var pageIDForLog sql.NullInt64

			if r.Header.Get("X-Toolkit-Source") == "JS-Path-Sender" {
				logSource = "JS Analysis"
			}

			scopeMu.RLock()
			if activeRecordingPageID != nil {
				logSource = "Page Sitemap"
				pageIDForLog = sql.NullInt64{Int64: *activeRecordingPageID, Valid: true}
			}
			scopeMu.RUnlock()

			requestData := &models.HTTPTrafficLog{
				TargetID:                   currentTargetIDForLog,
				Timestamp:                  startTime,
				RequestMethod:              models.NullString(r.Method),
				RequestURL:                 models.NullString(serverSeenURL),
				RequestHTTPVersion:         models.NullString(r.Proto),
				RequestHeaders:             models.NullString(string(reqHeadersJson)),
				RequestBody:                reqBodyBytes,
				ClientIP:                   models.NullString(r.RemoteAddr),
				IsHTTPS:                    isCurrentSessionHTTPS,
				RequestFullURLWithFragment: requestFullURLWithFragment,
			}

			requestData.LogSource = models.NullString(logSource)
			requestData.PageSitemapID = pageIDForLog
			ctxData := &proxyRequestContextData{TrafficLog: requestData}

			if isSynackTargetListURL {
				authToken := r.Header.Get("Authorization")
				if strings.HasPrefix(authToken, "Bearer ") {
					ctxData.SynackAuthToken = authToken
					logger.ProxyInfo("Captured Authorization token for Synack target list request.")
				} else if authToken != "" {
					logger.ProxyInfo("WARNING: Found Authorization header for Synack target list, but it's not a Bearer token.")
				}
			}
			ctx.UserData = ctxData

			logger.ProxyInfo("REQ: %s %s (HTTPS: %t)", r.Method, serverSeenURL, isCurrentSessionHTTPS)
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
			requestData := pCtxData.TrafficLog

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
			requestData.ResponseReasonPhrase = models.NullString(strings.TrimPrefix(resp.Status, fmt.Sprintf("%d ", resp.StatusCode)))
			requestData.ResponseHTTPVersion = models.NullString(resp.Proto)
			requestData.ResponseHeaders = models.NullString(string(respHeadersJson))
			requestData.ResponseBody = respBodyBytes
			requestData.ResponseContentType = models.NullString(resp.Header.Get("Content-Type"))
			requestData.ResponseBodySize = int64(len(respBodyBytes))
			requestData.DurationMs = duration.Milliseconds()

			if requestData.ResponseContentType.Valid && strings.Contains(strings.ToLower(requestData.ResponseContentType.String), "text/html") {
				requestData.IsPageCandidate = true
			}

			logHttpTraffic(requestData)

			isSynackTargetListResp := false
			var reqPathNorm, configPathNorm string

			if parsedSynackTargetURL != nil && ctx.Req != nil && ctx.Req.URL != nil {
				reqPathNorm = strings.TrimRight(ctx.Req.URL.Path, "/")
				configPathNorm = strings.TrimRight(parsedSynackTargetURL.Path, "/")
				if ctx.Req.URL.Scheme == parsedSynackTargetURL.Scheme &&
					ctx.Req.URL.Hostname() == parsedSynackTargetURL.Hostname() &&
					reqPathNorm == configPathNorm {
					isSynackTargetListResp = true
				}
			}

			if isSynackTargetListResp &&
				resp.StatusCode == http.StatusOK &&
				requestData.ResponseContentType.Valid && strings.Contains(strings.ToLower(requestData.ResponseContentType.String), "application/json") {

				authTokenForAnalytics := pCtxData.SynackAuthToken
				logger.ProxyInfo("Synack target list URL detected: %s. Using auth token: %t", ctx.Req.URL.String(), authTokenForAnalytics != "")
				go processSynackTargetList(respBodyBytes, authTokenForAnalytics)
			}

			logger.ProxyInfo("RESP: %d %s for %s %s (Size: %d, Duration: %s)", resp.StatusCode, requestData.ResponseContentType.String, ctx.Req.Method, ctx.Req.URL.String(), requestData.ResponseBodySize, duration)

			muSession.Lock()
			delete(sessionIsHTTPS, ctx.Session)
			muSession.Unlock()

			return resp
		})

	logger.ProxyInfo("MITM Proxy server starting on :%s", port)
	return http.ListenAndServe(":"+port, proxy)
}

func logHttpTraffic(logEntry *models.HTTPTrafficLog) {
	if database.DB == nil {
		logger.ProxyError("logHttpTraffic: Database is not initialized.")
		return
	}
	logger.Debug("logHttpTraffic: Attempting to save log entry. RequestURL: '%s', RequestFullURLWithFragment: {String: '%s', Valid: %t}",
		logEntry.RequestURL.String, logEntry.RequestFullURLWithFragment.String, logEntry.RequestFullURLWithFragment.Valid)
	_, err := database.DB.Exec(`INSERT INTO http_traffic_log (
		target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body, request_full_url_with_fragment,
		response_status_code, response_reason_phrase, response_http_version, response_headers, response_body, response_content_type,
		response_body_size, duration_ms, client_ip, is_https, is_page_candidate, notes, log_source, page_sitemap_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logEntry.TargetID, logEntry.Timestamp, logEntry.RequestMethod, logEntry.RequestURL,
		logEntry.RequestHTTPVersion, logEntry.RequestHeaders, logEntry.RequestBody,
		logEntry.RequestFullURLWithFragment,
		logEntry.ResponseStatusCode, logEntry.ResponseReasonPhrase, logEntry.ResponseHTTPVersion,
		logEntry.ResponseHeaders, logEntry.ResponseBody, logEntry.ResponseContentType,
		logEntry.ResponseBodySize, logEntry.DurationMs, logEntry.ClientIP, logEntry.IsHTTPS,
		logEntry.IsPageCandidate, logEntry.Notes,
		logEntry.LogSource, logEntry.PageSitemapID)
	// Note: is_favorite defaults to FALSE in schema, not explicitly set here.
	if err != nil {
		logger.ProxyError("DB log error on response for %s %s: %v", logEntry.RequestMethod.String, logEntry.RequestURL.String, err)
	}
}

type rawSynackFindingItem struct {
	// Fields from "exploitable_locations"
	Type       string `json:"type"` // e.g., "url"
	Value      string `json:"value"`
	CreatedAt  int64  `json:"created_at"` // Unix timestamp
	Status     string `json:"status"`     // e.g., "fixed"
	OutOfScope *bool  `json:"outOfScope"` // Pointer to handle null

	// Fields that might not be present in exploitable_locations, but are in models.SynackFinding
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Category    string          `json:"category"`
	Severity    string          `json:"severity"`
	FullDetails json.RawMessage `json:"-"` // To store the whole finding JSON
}

func processSynackTargetList(jsonData []byte, authToken string) {
	logger.ProxyInfo("Processing Synack target list JSON (%d bytes)...", len(jsonData))

	var rawTargets []map[string]interface{}
	httpClient := &http.Client{Timeout: 20 * time.Second}

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
			logger.ProxyError("Synack target list: Error upserting target with Synack ID '%s': %v", synackID, errUpsert)
			continue
		}

		if config.AppConfig.Synack.AnalyticsEnabled && config.AppConfig.Synack.AnalyticsBaseURL != "" && config.AppConfig.Synack.AnalyticsPathPattern != "" {
			analyticsURL := fmt.Sprintf(config.AppConfig.Synack.AnalyticsBaseURL+config.AppConfig.Synack.AnalyticsPathPattern, synackID)
			logger.ProxyInfo("Fetching analytics for Synack target '%s' from URL: %s", synackID, analyticsURL)

			req, err := http.NewRequest("GET", analyticsURL, nil)
			if err != nil {
				logger.ProxyError("Synack Analytics: Error creating request for target '%s': %v", synackID, err)
			}
			if authToken != "" {
				req.Header.Set("Authorization", authToken)
				logger.ProxyDebug("Synack Analytics: Added Authorization header to request for target '%s'", synackID)
			}

			lastFetchTime, errCheck := database.GetSynackTargetAnalyticsFetchTime(dbID)
			if errCheck != nil {
				logger.ProxyError("Synack Analytics: Error checking existing analytics for target DB_ID %d (Synack ID %s): %v", dbID, synackID, errCheck)
			}

			// For now, if analytics exist at all (lastFetchTime is not nil), don't refetch.
			// Later, a freshness check can be added: e.g., if lastFetchTime != nil && time.Since(*lastFetchTime) < 24*time.Hour
			if lastFetchTime != nil {
				logger.ProxyInfo("Synack Analytics: Analytics data already exists for target DB_ID %d (Synack ID %s), fetched at %s. Skipping fetch.", dbID, synackID, lastFetchTime.Format(time.RFC3339))
			} else {
				// Rate limit the API calls using the ticker.
				logger.ProxyDebug("Synack Analytics: Waiting for analyticsFetchTicker for target DB_ID %d (Synack ID %s)...", dbID, synackID)
				<-analyticsFetchTicker.C
				logger.ProxyInfo("Synack Analytics: No existing analytics found for target DB_ID %d (Synack ID %s) or fetch time is nil. Proceeding to fetch.", dbID, synackID)
				resp, errDo := httpClient.Do(req)
				if errDo != nil {
					logger.ProxyError("Synack Analytics: Error fetching analytics for target DB_ID %d (Synack ID %s) from %s: %v", dbID, synackID, analyticsURL, errDo)
				}
				defer resp.Body.Close()

				body, errRead := io.ReadAll(resp.Body)
				if errRead != nil {
					logger.ProxyError("Synack Analytics: Error reading analytics response body for target DB_ID %d (Synack ID %s) (status %d): %v", dbID, synackID, resp.StatusCode, errRead)
				}
				logger.ProxyInfo("Synack Analytics: Received status %d for target DB_ID %d (Synack ID %s). Response (first 500 chars): %s", resp.StatusCode, dbID, synackID, string(body[:min(len(body), 500)]))

				analyticsArrayPath := "value"
				categoriesResult := gjson.GetBytes(body, analyticsArrayPath)

				if !categoriesResult.Exists() {
					logger.ProxyError("Synack Analytics: Expected key '%s' not found in analytics JSON response for target DB_ID %d (Synack ID %s). The response might be a direct array or use a different key. Response (first 500 chars): %s", analyticsArrayPath, dbID, synackID, string(body[:min(len(body), 500)]))
				} else if !categoriesResult.IsArray() {
					logger.ProxyError("Synack Analytics: Value at key '%s' in analytics JSON is not an array (type: %s) for target DB_ID %d (Synack ID %s). Response (first 500 chars): %s", analyticsArrayPath, categoriesResult.Type.String(), dbID, synackID, string(body[:min(len(body), 500)]))
				} else {
					var rawAnalyticsItems []struct {
						Categories           []string        `json:"categories"`
						ExploitableLocations json.RawMessage `json:"exploitable_locations"`
						// Other fields like count, total_amount_paid, average_amount_paid from the old structure are not in the new example.
						// If they exist in other responses, this struct might need to be more flexible or have more fields.
					}

					if errUnmarshal := json.Unmarshal([]byte(categoriesResult.Raw), &rawAnalyticsItems); errUnmarshal != nil {
						logger.ProxyError("Synack Analytics: Error unmarshalling analytics items from key '%s' for target DB_ID %d (Synack ID %s): %v", analyticsArrayPath, dbID, synackID, errUnmarshal)
					} else {
						aggregatedCategories := make(map[string]int64)
						for _, rawItem := range rawAnalyticsItems {

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
							}

							if config.AppConfig.Synack.FindingsEnabled {
								logger.ProxyInfo("Synack Findings: Processing 'exploitable_locations' for target DB_ID %d, category group with categories: %v", dbID, rawItem.Categories)
								if config.AppConfig.Synack.FindingsArrayPathInAnalyticsJson != "" {
									logger.ProxyInfo("Warning: Synack Findings: Config key 'findings_array_path_in_analytics_json' is set but currently ignored. Findings are sourced from 'exploitable_locations' within each category group.")
								}

								if rawItem.ExploitableLocations != nil {
									var exploitableLocationsList []rawSynackFindingItem

									logger.ProxyDebug("Synack Findings: Raw exploitable_locations JSON for target DB_ID %d (first 500 chars): %s", dbID, string(rawItem.ExploitableLocations[:min(len(rawItem.ExploitableLocations), 500)]))

									if errUnmarshalEL := json.Unmarshal(rawItem.ExploitableLocations, &exploitableLocationsList); errUnmarshalEL != nil {
										logger.ProxyError("Synack Findings: Error unmarshalling exploitable_locations for category group (target DB_ID %d): %v. Raw JSON: %s", dbID, errUnmarshalEL, string(rawItem.ExploitableLocations[:min(len(rawItem.ExploitableLocations), 200)]))
									} else {
										logger.ProxyInfo("Synack Findings: Found %d exploitable_locations for this category group (target DB_ID %d).", len(exploitableLocationsList), dbID)
										// >2 to avoid logging for empty array "[]"
										if len(exploitableLocationsList) == 0 && len(rawItem.ExploitableLocations) > 2 {
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
												SynackFindingID:  exploitableLocation.Value,
												Title:            fmt.Sprintf("Finding at %s", exploitableLocation.Value),
												CategoryName:     primaryCategoryName,
												Status:           exploitableLocation.Status,
												VulnerabilityURL: exploitableLocation.Value,
											}

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
								} else {
									logger.ProxyDebug("Synack Findings: No 'exploitable_locations' data for this category group (target DB_ID %d).", dbID)
								}
							} else {
								logger.ProxyInfo("Synack Findings: Individual findings processing is DISABLED in config for target DB_ID %d.", dbID)
							}
						}

						var finalAnalyticsCategories []models.SynackTargetAnalyticsCategory
						for name, count := range aggregatedCategories {
							finalAnalyticsCategories = append(finalAnalyticsCategories, models.SynackTargetAnalyticsCategory{CategoryName: name, Count: count})
						}

						if errStore := database.StoreSynackTargetAnalytics(dbID, finalAnalyticsCategories); errStore != nil {
							logger.ProxyError("Synack Analytics: Error storing analytics for target DB_ID %d (Synack ID %s): %v", dbID, synackID, errStore)
						} else {
							logger.ProxyInfo("Synack Analytics: Successfully stored %d analytics categories for target DB_ID %d (Synack ID %s)", len(finalAnalyticsCategories), dbID, synackID)
						}
					}
				}
			}
		}
	}

	if len(currentSeenIDs) > 0 {
		if err := database.DeactivateMissingSynackTargets(currentSeenIDs, time.Now()); err != nil {
			logger.ProxyError("Synack target list: Error deactivating missing targets: %v", err)
		}
	} else {
		logger.ProxyInfo("Synack target list: No valid target IDs found in the current list, skipping deactivation step.")
	}

	logger.ProxyInfo("Finished processing Synack target list. Saw %d targets.", len(currentSeenIDs))
}

func SendGETRequestsThroughProxy(targetID int64, urls []string) error {
	if len(urls) == 0 {
		return nil
	}

	// IMPORTANT: Configure this client to use your running proxy.
	// The proxy address and port should come from your application's config.
	// If config.AppConfig.Proxy.Port is a string (e.g., "8778")
	proxyURLString := fmt.Sprintf("http://localhost:%s", config.AppConfig.Proxy.Port)
	proxyURL, err := url.Parse(proxyURLString)
	if err != nil {
		return fmt.Errorf("invalid proxy URL from config: %w", err)
	}

	// Create a new CertPool and add the proxy's CA certificate to it
	customCAPool := x509.NewCertPool()
	if caCert != nil {
		customCAPool.AddCert(caCert)
	} else {
		logger.Error("Core: SendGETRequestsThroughProxy - caCert is nil, cannot add to custom CA pool. HTTPS requests might fail verification.")
		// Potentially return an error here or proceed with system CAs only
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			// Configure TLS to trust our custom CA
			TLSClientConfig: &tls.Config{RootCAs: customCAPool},
		},
		// Set a reasonable timeout
		Timeout: 15 * time.Second,
	}

	logger.Info("Core: Attempting to send %d GET requests for target %d via proxy.", len(urls), targetID)

	for _, u := range urls {
		logger.Info("Core: Sending GET request to: %s (TargetID: %d)", u, targetID)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			logger.Error("Core: Error creating request for %s: %v", u, err)
			// Skip this URL
			continue
		}

		// Add custom headers to identify these requests and potentially
		// prevent them from triggering further analysis loops in the proxy handler.
		req.Header.Set("X-Toolkit-Initiated", "true")
		req.Header.Set("X-Toolkit-Source", "JS-Path-Sender")
		// Ensure this is being set
		req.Header.Set("X-Toolkit-Full-URL", u)
		req.Header.Set("User-Agent", "BHToolkit-Path-Tester/1.0")

		// Note: We are NOT explicitly setting the TargetID here in the request headers.
		// The proxy's OnRequest handler will need to be modified if you want these
		// specific requests to be logged with the provided `targetID` instead of
		// the globally active one or no target ID if out of scope.
		// For now, they will be logged based on the proxy's standard scope rules.
		logger.Info("Core: SendGETRequestsThroughProxy - Sending request with X-Toolkit-Full-URL header set to: %s", u)

		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Core: Error sending request to %s: %v", u, err)
			// Log this error. The request might still appear in the proxy log
			// if it reached the proxy but failed to reach the target.
			continue
		}
		if resp != nil && resp.Body != nil {
			// Read and discard body to reuse connection
			io.Copy(io.Discard, resp.Body)
			// Ensure response body is closed
			resp.Body.Close()
		}
		logger.Debug("Core: Request to %s completed with status: %s", u, resp.Status)
		// The actual logging of this request/response happens in the main proxy handlers
		// in mitmproxy.go's OnRequest/OnResponse DoFuncs. This function just initiates the request.
	}
	logger.Info("Core: Finished sending %d GET requests for target %d.", len(urls), targetID)
	return nil
}
