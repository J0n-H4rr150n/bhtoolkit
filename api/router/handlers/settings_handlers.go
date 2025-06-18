package handlers

import (
	"encoding/json"
	"fmt"
	"io" // Import the io package
	"net/http"
	"strconv"
	"toolkit/database"
	"toolkit/config" // Import the config package
	"toolkit/logger"
	"toolkit/models"
) // Ensure models is imported if TableLayoutConfig is there

// GetCurrentTargetSettingHandler retrieves the currently set target ID.
func GetCurrentTargetSettingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("GetCurrentTargetSettingHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetIDStr, err := database.GetSetting(models.CurrentTargetIDKey)
	if err != nil {
		logger.Error("GetCurrentTargetSettingHandler: Error getting current target setting: %v", err)
		http.Error(w, "Failed to retrieve current target setting", http.StatusInternalServerError)
		return
	}

	var response struct {
		TargetID *int64 `json:"target_id"`
	}

	if targetIDStr != "" {
		targetID, convErr := strconv.ParseInt(targetIDStr, 10, 64)
		if convErr == nil {
			response.TargetID = &targetID
		} else {
			logger.Error("GetCurrentTargetSettingHandler: Error converting stored target_id '%s' to int: %v", targetIDStr, convErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetCurrentTargetSettingHandler sets the current target ID.
func SetCurrentTargetSettingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("SetCurrentTargetSettingHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TargetID *int64 `json:"target_id"` // Pointer to handle null/unset
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("SetCurrentTargetSettingHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var targetIDStr string
	if req.TargetID != nil {
		targetIDStr = strconv.FormatInt(*req.TargetID, 10)
		// Optional: Validate if the target ID actually exists in the database
		var exists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", *req.TargetID).Scan(&exists)
		if err != nil || !exists {
			logger.Error("SetCurrentTargetSettingHandler: Target ID %d does not exist or DB error: %v", *req.TargetID, err)
			http.Error(w, fmt.Sprintf("Target ID %d not found or invalid.", *req.TargetID), http.StatusBadRequest)
			return
		}
	} else {
		targetIDStr = "" // Explicitly set to empty string to clear the setting
	}

	if err := database.SetSetting(models.CurrentTargetIDKey, targetIDStr); err != nil {
		logger.Error("SetCurrentTargetSettingHandler: Error saving current target setting: %v", err)
		http.Error(w, "Failed to save current target setting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Current target updated successfully."})
}

// GetCustomHeadersSettingHandler retrieves the custom HTTP headers setting.
func GetCustomHeadersSettingHandler(w http.ResponseWriter, r *http.Request) {
	headersJSON, err := database.GetSetting(models.CustomHTTPHeadersKey)
	if err != nil {
		logger.Error("GetCustomHeadersSettingHandler: Error getting custom headers setting: %v", err)
		http.Error(w, "Failed to retrieve custom headers setting", http.StatusInternalServerError)
		return
	}

	var headersMap map[string]string
	if headersJSON == "" {
		headersMap = make(map[string]string)
	} else {
		if err := json.Unmarshal([]byte(headersJSON), &headersMap); err != nil {
			logger.Error("GetCustomHeadersSettingHandler: Error unmarshalling custom headers JSON: %v. Stored value: %s", err, headersJSON)
			headersMap = make(map[string]string) // Fallback to empty map
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(headersMap)
}

// SetCustomHeadersSettingHandler saves the custom HTTP headers setting.
func SetCustomHeadersSettingHandler(w http.ResponseWriter, r *http.Request) {
	var headersMap map[string]string
	if err := json.NewDecoder(r.Body).Decode(&headersMap); err != nil {
		logger.Error("SetCustomHeadersSettingHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	headersJSON, err := json.Marshal(headersMap)
	if err != nil {
		logger.Error("SetCustomHeadersSettingHandler: Error marshalling custom headers to JSON: %v", err)
		http.Error(w, "Failed to process custom headers", http.StatusInternalServerError)
		return
	}

	if err := database.SetSetting(models.CustomHTTPHeadersKey, string(headersJSON)); err != nil {
		logger.Error("SetCustomHeadersSettingHandler: Error saving custom headers setting: %v", err)
		http.Error(w, "Failed to save custom headers setting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Custom HTTP headers saved successfully."})
}

// GetTableColumnWidthsHandler retrieves the custom table column widths settings.
func GetTableColumnWidthsHandler(w http.ResponseWriter, r *http.Request) {
	widthsJSON, err := database.GetSetting(models.TableColumnWidthsKey)
	if err != nil {
		logger.Error("GetTableColumnWidthsHandler: Error getting column widths: %v", err)
		http.Error(w, "Failed to retrieve column widths", http.StatusInternalServerError)
		return
	}

	var payload models.AllTableLayouts // Use the new model type
	if widthsJSON == "" {
		payload = make(models.AllTableLayouts)
	} else {
		if err := json.Unmarshal([]byte(widthsJSON), &payload); err != nil {
			logger.Error("GetTableColumnWidthsHandler: Error unmarshalling column widths JSON: %v. Stored value: %s", err, widthsJSON)
			payload = make(models.AllTableLayouts) // Fallback
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// SetTableColumnWidthsHandler saves the custom table column widths settings.
func SetTableColumnWidthsHandler(w http.ResponseWriter, r *http.Request) {
	var payload models.AllTableLayouts // Use the new model type

	// Read the body into a byte slice first for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("SetTableColumnWidthsHandler: Error reading request body: %v", err)
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close() // Ensure body is closed

	// Log the raw body content
	logger.Debug("SetTableColumnWidthsHandler: Received raw body: %s", string(bodyBytes))

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		logger.Error("SetTableColumnWidthsHandler: Error decoding request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request payload: " + err.Error()})
		return
	}

	widthsJSON, err := json.Marshal(payload)
	if err != nil {
		logger.Error("SetTableColumnWidthsHandler: Error marshalling column widths to JSON: %v", err)
		http.Error(w, "Failed to process column widths", http.StatusInternalServerError)
		return
	}

	if err := database.SetSetting(models.TableColumnWidthsKey, string(widthsJSON)); err != nil {
		logger.Error("SetTableColumnWidthsHandler: Error saving column widths: %v", err)
		http.Error(w, "Failed to save column widths", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Table column widths saved successfully."})
}

// ResetTableColumnWidthsHandler resets all table column widths to default (empty JSON).
func ResetTableColumnWidthsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("ResetTableColumnWidthsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := database.SetSetting(models.TableColumnWidthsKey, "{}"); err != nil {
		logger.Error("ResetTableColumnWidthsHandler: Error resetting column widths: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to reset table column widths."})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "All table column widths have been reset."})
	logger.Info("All table column widths have been reset in database.")
}

// GetUISettingsHandler retrieves all UI-related settings.
func GetUISettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("GetUISettingsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := make(map[string]interface{})
	var errs []string

	// Current Target ID
	targetIDStr, err := database.GetSetting(models.CurrentTargetIDKey)
	if err != nil {
		logger.Error("GetUISettingsHandler: Error getting current_target_id: %v", err)
		errs = append(errs, "current_target_id")
		settings[models.CurrentTargetIDKey] = nil // Explicitly set to null on error
	} else if targetIDStr == "" {
		settings[models.CurrentTargetIDKey] = nil // Explicitly set to null if empty
	} else {
		targetID, convErr := strconv.ParseInt(targetIDStr, 10, 64)
		if convErr != nil {
			logger.Error("GetUISettingsHandler: Error converting current_target_id '%s': %v", targetIDStr, convErr)
			errs = append(errs, "current_target_id_conversion")
			settings[models.CurrentTargetIDKey] = nil
		} else {
			settings[models.CurrentTargetIDKey] = targetID
		}
	}

	// Custom HTTP Headers
	headersJSON, err := database.GetSetting(models.CustomHTTPHeadersKey)
	var headersMap map[string]string
	if err != nil {
		logger.Error("GetUISettingsHandler: Error getting custom_http_headers: %v", err)
		errs = append(errs, "custom_http_headers")
		headersMap = make(map[string]string) // Default to empty
	} else if headersJSON == "" {
		headersMap = make(map[string]string) // Default to empty
	} else {
		if err := json.Unmarshal([]byte(headersJSON), &headersMap); err != nil {
			logger.Error("GetUISettingsHandler: Error unmarshalling custom_http_headers: %v", err)
			errs = append(errs, "custom_http_headers_unmarshal")
			headersMap = make(map[string]string) // Default to empty
		}
	}
	settings[models.CustomHTTPHeadersKey] = headersMap

	// Table Column Widths
	widthsJSON, err := database.GetSetting(models.TableColumnWidthsKey)
	var widthsMap models.AllTableLayouts
	if err != nil {
		logger.Error("GetUISettingsHandler: Error getting table_column_widths: %v", err)
		errs = append(errs, "table_column_widths")
		widthsMap = make(models.AllTableLayouts)
	} else if widthsJSON == "" {
		widthsMap = make(models.AllTableLayouts)
	} else {
		if err := json.Unmarshal([]byte(widthsJSON), &widthsMap); err != nil {
			logger.Error("GetUISettingsHandler: Error unmarshalling table_column_widths: %v", err)
			errs = append(errs, "table_column_widths_unmarshal") // Corrected line
			widthsMap = make(models.AllTableLayouts)             // Default to empty
		}
	}
	settings[models.TableColumnWidthsKey] = widthsMap

	// UI Settings (like showSynackSection)
	uiSettingsJSON, err := database.GetSetting(models.UISettingsKey)
	var generalUISettings map[string]interface{} // To hold settings like showSynackSection
	if err != nil {
		logger.Error("GetUISettingsHandler: Error getting ui_settings: %v", err)
		errs = append(errs, "ui_settings")
		generalUISettings = map[string]interface{}{"showSynackSection": false} // Default on error
	} else if uiSettingsJSON == "" {
		generalUISettings = map[string]interface{}{"showSynackSection": false} // Default if not set
	} else {
		if err := json.Unmarshal([]byte(uiSettingsJSON), &generalUISettings); err != nil {
			logger.Error("GetUISettingsHandler: Error unmarshalling ui_settings: %v. JSON: %s", err, uiSettingsJSON)
			errs = append(errs, "ui_settings_unmarshal")
			generalUISettings = map[string]interface{}{"showSynackSection": false} // Default on unmarshal error
		}
	}
	// Merge generalUISettings into the main settings map
	for k, v := range generalUISettings {
		settings[k] = v
	}

	if len(errs) > 0 {
		logger.Info("GetUISettingsHandler: WARN: Encountered errors fetching some UI settings: %v", errs)
		// Decide if you want to return a partial success or an error.
		// For UI settings, often partial success is acceptable.
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// SetUISettingsHandler saves various UI settings.
func SetUISettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		logger.Error("SetUISettingsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed (use PUT or POST)", http.StatusMethodNotAllowed)
		return
	}

	var newSettings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		logger.Error("SetUISettingsHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// For now, we only handle 'showSynackSection'.
	// This could be expanded to save the whole map or individual keys.
	if showSynack, ok := newSettings["showSynackSection"].(bool); ok {
		// Example: Storing as a JSON blob under a single key "ui_settings"
		// Or, store individual keys like "ui_settings_show_synack"
		// For simplicity, let's assume we update a general ui_settings JSON.
		// Fetch existing, merge, then save. Or just overwrite if simple.
		// Here, we'll just save this specific setting.
		// A more robust approach would merge with existing UI settings.
		uiSettingsToSave := map[string]interface{}{"showSynackSection": showSynack}
		settingsJSON, err := json.Marshal(uiSettingsToSave)
		if err != nil {
			logger.Error("SetUISettingsHandler: Error marshalling UI settings: %v", err)
			http.Error(w, "Failed to process UI settings", http.StatusInternalServerError)
			return
		}
		if err := database.SetSetting(models.UISettingsKey, string(settingsJSON)); err != nil {
			logger.Error("SetUISettingsHandler: Error saving UI settings: %v", err)
			http.Error(w, "Failed to save UI settings", http.StatusInternalServerError)
			return
		}
	} // Add more settings handling here as needed

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "UI settings saved successfully."})
	logger.Info("UI settings updated.")
}

// GetProxyExclusionRulesHandler retrieves the list of global proxy exclusion rules.
func GetProxyExclusionRulesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("GetProxyExclusionRulesHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rules, err := database.GetProxyExclusionRules()
	if err != nil {
		logger.Error("GetProxyExclusionRulesHandler: Error getting rules: %v", err)
		http.Error(w, "Failed to retrieve proxy exclusion rules", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
	logger.Info("Successfully served %d proxy exclusion rules.", len(rules))
}

// SetProxyExclusionRulesHandler saves the list of global proxy exclusion rules.
func SetProxyExclusionRulesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut { // Allow POST or PUT
		logger.Error("SetProxyExclusionRulesHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed (use POST or PUT)", http.StatusMethodNotAllowed)
		return
	}

	var rules []models.ProxyExclusionRule
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		logger.Error("SetProxyExclusionRulesHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := database.SetProxyExclusionRules(rules); err != nil {
		logger.Error("SetProxyExclusionRulesHandler: Error saving rules: %v", err)
		http.Error(w, "Failed to save proxy exclusion rules", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Proxy exclusion rules saved successfully."})
	logger.Info("Successfully saved %d proxy exclusion rules.", len(rules))
}

// ApplicationSettingsResponse defines the structure for the /settings/app endpoint.
type ApplicationSettingsResponse struct {
	UI       config.UIConfig       `json:"ui"`
	Missions config.MissionsConfig `json:"missions"`
	// Add other sections as needed
}

// GetApplicationSettingsHandler retrieves the current application settings.
func GetApplicationSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("GetApplicationSettingsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := ApplicationSettingsResponse{
		UI:       config.AppConfig.UI,
		Missions: config.AppConfig.Missions,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("GetApplicationSettingsHandler: Error encoding response: %v", err)
		http.Error(w, "Failed to encode settings", http.StatusInternalServerError)
	}
}

// SaveApplicationSettingsHandler saves the application settings.
func SaveApplicationSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		logger.Error("SaveApplicationSettingsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newSettings ApplicationSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		logger.Error("SaveApplicationSettingsHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Update the global AppConfig
	config.AppConfig.UI = newSettings.UI
	config.AppConfig.Missions = newSettings.Missions

	// Persist the changes to the config file
	// This function needs to be implemented in your config package.
	if err := config.SaveAppConfig(); err != nil {
		logger.Error("SaveApplicationSettingsHandler: Error saving application configuration: %v", err)
		http.Error(w, "Failed to save application settings", http.StatusInternalServerError)
		return
	}

	// TODO: Consider if any services need to be re-initialized or notified of config changes.
	// For example, if mission polling interval changes, the SynackMissionService might need a restart or update.

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Application settings saved successfully."})
	logger.Info("Application settings updated and saved to config file.")
}
