package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
	"toolkit/config"
	"database/sql"
	"toolkit/logger"
	"toolkit/models" // Corrected, was commented out
)

// SynackMissionService handles polling for and claiming Synack missions.
type SynackMissionService struct {
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	conf            *config.MissionsConfig
	synackConf      *config.SynackConfig
	db              *sql.DB // Or your specific database handler type
	authToken       string
	mu              sync.Mutex
	isPollingActive bool
}

// NewSynackMissionService creates a new instance of the SynackMissionService.
func NewSynackMissionService(appCtx context.Context, appConfig *config.Configuration, db *sql.DB) *SynackMissionService {
	ctx, cancel := context.WithCancel(appCtx)
	return &SynackMissionService{
		ctx:        ctx,
		cancel:     cancel,
		conf:       &appConfig.Missions,
		synackConf: &appConfig.Synack, // Assuming Synack API base URLs might be in the general Synack config
		db:         db,
	}
}

// SetAuthToken updates the authentication token used by the service.
func (s *SynackMissionService) SetAuthToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authToken != token {
		s.authToken = token
		if token != "" {
			logger.Debug("SynackMissionService: Auth token updated.")
		} else {
			logger.Debug("SynackMissionService: Auth token cleared.")
		}
	}
}

// Start begins the mission polling loop.
func (s *SynackMissionService) Start() {
	if !s.conf.Enabled {
		logger.Info("Synack Mission Polling Service is disabled in configuration.")
		return
	}

	s.mu.Lock()
	if s.isPollingActive {
		s.mu.Unlock()
		logger.Warn("Synack Mission Polling Service is already active.")
		return
	}
	s.isPollingActive = true
	s.mu.Unlock()

	logger.Info("Synack Mission Polling Service starting...")
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			s.isPollingActive = false
			s.mu.Unlock()
			logger.Info("Synack Mission Polling goroutine finished.")
		}()

		pollingIntervalSeconds := s.conf.PollingIntervalSeconds
		if pollingIntervalSeconds < 10 {
			logger.Info("SynackMissionService: Configured polling interval (%ds) is less than minimum (10s). Using 10s.", pollingIntervalSeconds)
			pollingIntervalSeconds = 10
		}
		ticker := time.NewTicker(time.Duration(pollingIntervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				logger.Info("Synack Mission Polling Service: context cancelled, exiting poll loop.")
				return
			case <-ticker.C:
				s.fetchAndProcessMissions()
			}
		}
	}()
}

// Stop gracefully stops the mission polling service.
func (s *SynackMissionService) Stop() {
	logger.Info("Synack Mission Polling Service stopping...")
	s.cancel()
	s.wg.Wait()
	logger.Info("Synack Mission Polling Service stopped.")
}

func (s *SynackMissionService) fetchAndProcessMissions() {
	s.mu.Lock()
	token := s.authToken
	s.mu.Unlock()

	if token == "" {
		logger.Debug("SynackMissionService: No auth token available, skipping mission fetch.")
		return
	}

	if s.conf.ListURL == "" {
		logger.Error("SynackMissionService: Mission list URL is not configured.")
		return
	}

	logger.Debug("SynackMissionService: Fetching missions from %s", s.conf.ListURL)

	req, err := http.NewRequestWithContext(s.ctx, "GET", s.conf.ListURL, nil)
	if err != nil {
		logger.Error("SynackMissionService: Error creating request for missions: %v", err)
		return
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/json")
	// Add other headers from your Python example if necessary

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("SynackMissionService: Error fetching missions: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("SynackMissionService: Error fetching missions, status code %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}

	var missions []models.SynackAPIMission
	if err := json.NewDecoder(resp.Body).Decode(&missions); err != nil {
		logger.Error("SynackMissionService: Error decoding missions response: %v", err)
		return
	}

	logger.Info("SynackMissionService: Fetched %d available missions.", len(missions))

	if len(missions) == 0 {
		return
	}

	for _, mission := range missions {
		// Check if we should attempt to claim this mission
		// Using s.conf.ClaimMinPayout (e.g., if payout is <= ClaimMinPayout)
		// Now using both min and max payout.
		payout := mission.Payout.Amount
		if payout >= s.conf.ClaimMinPayout && payout <= s.conf.ClaimMaxPayout && payout > 0 { // Ensure payout is positive and within range
			logger.Info("SynackMissionService: Mission '%s' (ID: %s, Payout: %.2f %s) meets claim criteria (Min: %.2f, Max: %.2f). Attempting to claim.",
				mission.Title, mission.ID, payout, mission.Payout.Currency, s.conf.ClaimMinPayout, s.conf.ClaimMaxPayout)
			
			// Before attempting to claim, we should check if we've already tried to claim or successfully claimed this mission.
			// This requires a database lookup. For now, we'll proceed directly to attemptClaim.
			// exists, err := database.CheckIfMissionExists(s.db, mission.ID) // Example function
			// if err != nil {
			//  logger.Error("SynackMissionService: Error checking if mission %s exists in DB: %v", mission.ID, err)
			//  continue
			// }
			// if exists {
			//  logger.Debug("SynackMissionService: Mission %s already processed/exists in DB. Skipping.", mission.ID)
			//  continue
			// }
			s.attemptClaimMission(mission)
		} else {
			logger.Debug("SynackMissionService: Mission '%s' (ID: %s, Payout: %.2f %s) does not meet claim criteria (Min: %.2f, Max: %.2f). Skipping.",
				mission.Title, mission.ID, payout, mission.Payout.Currency, s.conf.ClaimMinPayout, s.conf.ClaimMaxPayout)
		}
	}
}

// attemptClaimMission will handle the logic to make the API call to claim a mission.
// It will also handle saving the mission details to the database.
func (s *SynackMissionService) attemptClaimMission(mission models.SynackAPIMission) {
	logger.Info("SynackMissionService: Placeholder - Attempting to claim mission ID: %s, Title: %s", mission.ID, mission.Title)
	// TODO: Implement mission claim logic:
	// 1. Construct claim URL using s.conf.ClaimURLPattern and mission details.
	// 2. Make HTTP POST/PUT request to the claim URL with appropriate payload.
	// 3. Handle response (success/failure).
	// 4. If successful, save mission details to DB using a function from database/synack_db.go
	//    - Map models.SynackAPIMission to models.SynackMission
	//    - Call a database function like database.SaveClaimedMission(s.db, missionToSave)
	// 5. Send Slack notification.
}