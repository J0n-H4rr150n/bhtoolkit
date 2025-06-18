import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let localApplyUiSettingsFunc; // To store the function passed from app.js
let stateService;
// tableService might not be directly needed here unless settings involve table layouts directly

// Module-level state for proxy exclusion rules
let currentProxyExclusionRules = [];
let currentFullAppSettings = null; // To store the full settings loaded

// DOM element references (will be queried within functions or passed)
let viewContentContainer; // Main container, passed to load functions

/**
 * Initializes the Settings View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService.
 */
export function initSettingsView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    localApplyUiSettingsFunc = services.applyUiSettingsFunc; // Store the passed function
    console.log("[SettingsView] Initialized.");
}

/**
 * Loads the settings view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadSettingsView(mainViewContainer) {
    console.log('[SettingsView.js] loadSettingsView called. mainViewContainer:', mainViewContainer);
    viewContentContainer = mainViewContainer;

    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadSettingsView!");
        return;
    }
    console.log('[SettingsView.js] viewContentContainer is valid.');

    if (!apiService || !uiService || !stateService) {
        console.error("SettingsView not initialized. Call initSettingsView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SettingsView module not initialized. Critical services are missing.</p>";
        return;
    }
    console.log('[SettingsView.js] Services (api, ui, state) are initialized.');

    viewContentContainer.innerHTML = `
        <h1>Settings</h1>
        <div class="tabs" style="margin-bottom: 20px;">
            <button class="tab-button active" data-tab="proxyExclusionsTab">Proxy Exclusions</button>
            <button class="tab-button" data-tab="tableLayoutsTab">Table Layouts</button>
            <button class="tab-button" data-tab="uiSettingsTab">UI Settings</button>
        </div>

        <div id="proxyExclusionsTab" class="tab-content active">
            <h2>Global Proxy Exclusions</h2>
            <p>Define rules to prevent certain HTTP requests from being saved by the proxy.</p>
            <div id="proxyExclusionsMessage" class="message-area" style="margin-top: 10px;"></div>
            <div id="proxyExclusionsContainer" style="margin-top:15px;">
                <p>Loading proxy exclusion rules...</p>
            </div>
        </div>

        <div id="tableLayoutsTab" class="tab-content">
            <h2>Table Layouts</h2>
            <p>Manage saved column widths for various tables in the application.</p>
            <div id="tableLayoutsSettingsContainer" style="margin-top:15px;">
                <p>Loading table layout settings...</p>
            </div>
             <div id="tableLayoutsMessage" class="message-area" style="margin-top: 10px;"></div>
        </div>

        <div id="uiSettingsTab" class="tab-content">
            <h2>UI Settings</h2>
            <div id="uiSettingsContainer">
                <p>Loading UI settings...</p>
            </div>
        </div>
    `;

    console.log('[SettingsView.js] Tabbed HTML structure set for settings page.');
    await loadAndDisplayUISettings();
    await loadAndDisplayTableLayoutSettings();
    await loadAndDisplayProxyExclusionSettings(); // Placeholder for now

    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.addEventListener('click', () => {
            document.querySelectorAll('.tabs .tab-button').forEach(btn => btn.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
            button.classList.add('active');
            const tabId = button.getAttribute('data-tab');
            document.getElementById(tabId).classList.add('active');
        });
    });
}

async function loadAndDisplayUISettings() {
    const uiSettingsContainer = document.getElementById('uiSettingsContainer');
    if (!uiSettingsContainer) return;

    try {
        // We'll assume getAppSettings fetches all relevant settings including missions.
        // If not, this API call might need to be adjusted or a new one created.
        const appSettings = await apiService.getAppSettings(); // Changed from getUISettings
        currentFullAppSettings = appSettings; // Store the fetched settings
        const uiSpecificSettings = appSettings.ui || {}; // Assuming UI settings are nested under 'ui'
        const missionSettings = appSettings.missions || {}; // Assuming mission settings are nested under 'missions'

        const proxyLogDefaultSavedValue = localStorage.getItem('proxyLogDefaultToResponseTab');
        console.log(`[SettingsView] Loaded 'proxyLogDefaultToResponseTab' from localStorage: ${proxyLogDefaultSavedValue}`);
        uiSettingsContainer.innerHTML = `
            <div class="form-group">
                <label for="showSynackToggle">Show Synack Section in Sidebar:</label>
            <input type="checkbox" id="showSynackToggle" ${uiSpecificSettings.ShowSynackSection ? 'checked' : ''}>
            </div>
            <div class="form-group">
                <label for="proxyLogDefaultToResponseToggleSettings">Default to Response Tab in Proxy Log Detail:</label>
                <input type="checkbox" id="proxyLogDefaultToResponseToggleSettings" ${proxyLogDefaultSavedValue === 'true' ? 'checked' : ''}>
            </div>
            <hr style="margin: 20px 0;">
            <h4>Synack Mission Claiming</h4>
            <div class="form-group">
                <label for="claimMinPayout">Min Payout to Claim ($):</label>
                <input type="number" id="claimMinPayout" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ClaimMinPayout || 0)}" step="0.01">
            </div>
            <div class="form-group">
                <label for="claimMaxPayout">Max Payout to Claim ($):</label>
                <input type="number" id="claimMaxPayout" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ClaimMaxPayout || 50)}" step="0.01">
            </div>
            <button id="saveUISettingsBtn" class="primary">Save UI Settings</button>
            <div id="uiSettingsMessage" class="message-area" style="margin-top: 10px;"></div>
        `;

        document.getElementById('saveUISettingsBtn')?.addEventListener('click', handleSaveUISettings);

    } catch (error) {
        console.error("Error loading UI settings:", error);
        uiSettingsContainer.innerHTML = `<p class="error-message">Failed to load UI settings: ${escapeHtml(error.message)}</p>`;
        currentFullAppSettings = null; // Reset on error
    }
}

async function handleSaveUISettings() {
    const showSynackToggle = document.getElementById('showSynackToggle');
    const proxyLogDefaultToggle = document.getElementById('proxyLogDefaultToResponseToggleSettings');
    const claimMinPayoutInput = document.getElementById('claimMinPayout');
    const claimMaxPayoutInput = document.getElementById('claimMaxPayout');
    const messageArea = document.getElementById('uiSettingsMessage');

    if (!messageArea) return; // Only messageArea is critical for feedback

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    // Handle proxy log detail default tab setting (localStorage)
    if (proxyLogDefaultToggle) {
        localStorage.setItem('proxyLogDefaultToResponseTab', proxyLogDefaultToggle.checked); // Save its state
        console.log(`[SettingsView] Saved 'proxyLogDefaultToResponseTab' to localStorage: ${proxyLogDefaultToggle.checked}`);
    } else {
        console.warn("[SettingsView] 'proxyLogDefaultToResponseToggleSettings' element not found during save.");
    }

    // Ensure currentFullAppSettings is loaded, otherwise fetch it.
    // This is a fallback, ideally it's loaded when the view is displayed.
    if (!currentFullAppSettings) {
        try {
            console.log("[SettingsView] currentFullAppSettings is null, fetching fresh settings before save.");
            currentFullAppSettings = await apiService.getAppSettings();
        } catch (error) {
            messageArea.textContent = `Error: Could not load current settings to perform save. ${escapeHtml(error.message)}`;
            messageArea.classList.add('error-message');
            return;
        }
    }

    // Start with the existing full mission settings, or a default structure if none exist
    // This ensures that fields not present on this specific UI form are preserved.
    const baseMissionSettings = currentFullAppSettings.missions ? { ...currentFullAppSettings.missions } : {
        enabled: false, // Provide sensible defaults if missions section was missing entirely
        polling_interval_seconds: 10,
        list_url: "",
        claim_url_pattern: "",
        claim_min_payout: 0.0,
        claim_max_payout: 50.0
    };

    // Construct the payload for all settings
    const settingsToSave = {
        ui: {
            ShowSynackSection: showSynackToggle ? showSynackToggle.checked : (currentFullAppSettings.ui?.ShowSynackSection || false) // Corrected case
        },
        missions: {
            ...baseMissionSettings, // Spread the existing/default mission settings
            ClaimMinPayout: parseFloat(claimMinPayoutInput.value) || 0,
            ClaimMaxPayout: parseFloat(claimMaxPayoutInput.value) || 0
        }
    };

    try {
        // Assuming saveAppSettings can save the broader settings structure.
        await apiService.saveAppSettings(settingsToSave); // Assuming it resolves on success or throws on error

        // After successful save, update currentFullAppSettings with the saved data
        // This makes the local cache consistent with what was just sent.
        currentFullAppSettings.ui = settingsToSave.ui;
        currentFullAppSettings.missions = settingsToSave.missions;

             messageArea.textContent = 'UI settings saved successfully! Refresh may be needed for sidebar changes.';
             messageArea.classList.add('success-message');
             uiService.showModalMessage('Settings Saved', 'UI settings have been saved. A page refresh might be required to see all changes (like sidebar visibility).');
             // Call the applyUiSettings function passed from app.js
            if (localApplyUiSettingsFunc) {
                localApplyUiSettingsFunc(settingsToSave.ui); // Pass only the UI part to the existing function
            }
    } catch (error) {
        console.error("Error saving UI settings:", error);
        messageArea.textContent = `Network error saving UI settings: ${escapeHtml(error.message)}`;
        messageArea.classList.add('error-message');
    }
}

async function loadAndDisplayTableLayoutSettings() {
    const container = document.getElementById('tableLayoutsSettingsContainer');
    if (!container) return;

    try {
        const layouts = await apiService.getTableLayouts(); // Assuming this fetches all layouts
        let html = `<p>Current table layouts are managed by saving them on their respective pages (e.g., Proxy Log, Checklist).</p>`;
        
        if (Object.keys(layouts).length > 0) {
            html += `<h3>Currently Saved Layouts:</h3><ul>`;
            for (const key in layouts) {
                html += `<li>${escapeHtml(key)}: ${Object.keys(layouts[key]).length} columns configured</li>`;
            }
            html += `</ul>`;
        } else {
            html += `<p>No custom table layouts are currently saved.</p>`;
        }

        html += `<button id="resetAllTableLayoutsBtn" class="danger" style="margin-top: 20px;">Reset All Table Layouts</button>`;
        container.innerHTML = html;

        document.getElementById('resetAllTableLayoutsBtn')?.addEventListener('click', handleResetAllTableLayouts);

    } catch (error) {
        console.error("Error loading table layout settings:", error);
        container.innerHTML = `<p class="error-message">Failed to load table layout settings: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleResetAllTableLayouts() {
    const messageArea = document.getElementById('tableLayoutsMessage');
    if (!messageArea) return;

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    uiService.showModalConfirm(
        "Confirm Reset",
        "Are you sure you want to reset ALL saved table column layouts to their defaults? This action cannot be undone.",
        async () => {
            try {
                // Assuming apiService will have a method like `resetAllTableLayouts`
                // This would call a new backend endpoint, e.g., POST /api/settings/table-column-widths/reset
                const response = await fetch(`${apiService.API_BASE || '/api'}/settings/table-column-widths/reset`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                });

                if (response.ok) {
                    messageArea.textContent = 'All table layouts have been reset successfully.';
                    messageArea.classList.add('success-message');
                    uiService.showModalMessage('Layouts Reset', 'All table layouts have been reset. Refresh pages to see changes.');
                    // Reload this section to reflect the change
                    await loadAndDisplayTableLayoutSettings();
                    // Also update the global state if your stateService holds these
                    if (stateService && typeof stateService.updateState === 'function') {
                        stateService.updateState({ globalTableLayouts: {} });
                    }
                } else {
                    const errorData = await response.json();
                    messageArea.textContent = `Error resetting layouts: ${escapeHtml(errorData.message || response.statusText)}`;
                    messageArea.classList.add('error-message');
                }
            } catch (error) {
                console.error("Error resetting table layouts:", error);
                messageArea.textContent = `Network error resetting layouts: ${escapeHtml(error.message)}`;
                messageArea.classList.add('error-message');
            }
        }
    );
}

async function loadAndDisplayProxyExclusionSettings() {
    const container = document.getElementById('proxyExclusionsContainer');
    if (!container) return;

    try {
        currentProxyExclusionRules = await apiService.getProxyExclusionRules();
        renderProxyExclusionUI(container);
    } catch (error) {
        console.error("Error loading proxy exclusion settings:", error);
        container.innerHTML = `<p class="error-message">Failed to load proxy exclusion settings: ${escapeHtml(error.message)}</p>`;
        document.getElementById('proxyExclusionsMessage').textContent = `Error: ${escapeHtml(error.message)}`;
    }
}

function renderProxyExclusionUI(container) {
    if (!container) container = document.getElementById('proxyExclusionsContainer');
    if (!container) return;

    let headingHTML = '<h4>Current Exclusion Rules</h4>';
    let tableHTML = '';
    if (currentProxyExclusionRules.length === 0) {
        tableHTML = '<p>No global proxy exclusion rules defined.</p>';
    } else {
        tableHTML = `
            <table class="settings-table">
                <thead>
                    <tr>
                        <th>Enabled</th>
                        <th>Type</th>
                        <th>Pattern</th>
                        <th>Description</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
        `;
        currentProxyExclusionRules.forEach(rule => { // Changed rulesHTML to tableHTML
            tableHTML += `
                <tr data-rule-id="${escapeHtmlAttribute(rule.id)}">
                    <td><input type="checkbox" class="proxy-exclusion-enable" ${rule.is_enabled ? 'checked' : ''}></td>
                    <td>${escapeHtml(rule.rule_type)}</td>
                    <td>${escapeHtml(rule.pattern)}</td>
                    <td>${escapeHtml(rule.description)}</td>
                    <td><button class="danger small-button delete-proxy-exclusion-rule">Delete</button></td>
                </tr>
            `;
        });
        tableHTML += '</tbody></table>';
    }

    const saveButtonHTML = `<button id="saveProxyExclusionsBtn" class="primary" style="margin-top: 10px; margin-bottom: 15px;">Save All Proxy Exclusions</button>`;

    // Form for adding new rules remains at the top
    container.innerHTML = `
        <form id="addProxyExclusionForm" style="margin-bottom: 20px; padding: 15px; border: 1px solid #ddd; border-radius: 4px; background-color: #f9f9f9;">
            <h4>Add New Exclusion Rule</h4>
            <div class="form-group">
                <label for="proxyExclusionType">Rule Type:</label>
                <select id="proxyExclusionType" name="rule_type">
                    <option value="file_extension">File Extension (e.g., .css, .jpg)</option>
                    <option value="url_regex">URL Regex (e.g., google-analytics\\.com)</option>
                    <option value="domain">Domain (e.g., ads.example.com)</option>
                </select>
            </div>
            <div class="form-group">
                <label for="proxyExclusionPattern">Pattern:</label>
                <input type="text" id="proxyExclusionPattern" name="pattern" required placeholder="e.g., .png or ^https?://example\\.com/api/v1/ignore">
            </div>
            <div class="form-group">
                <label for="proxyExclusionDescription">Description (Optional):</label>
                <input type="text" id="proxyExclusionDescription" name="description">
            </div>
             <div class="form-group">
                <label for="proxyExclusionIsEnabled">Enabled by default:</label>
                <input type="checkbox" id="proxyExclusionIsEnabled" name="is_enabled" checked>
            </div>
            <button type="submit" class="primary">Add Rule</button>
        </form>
        <div id="proxyExclusionRulesList">
            ${headingHTML}
            ${saveButtonHTML}
            ${tableHTML}
        </div>
    `;

    document.getElementById('addProxyExclusionForm')?.addEventListener('submit', handleAddProxyExclusionRule);
    document.getElementById('saveProxyExclusionsBtn')?.addEventListener('click', handleSaveAllProxyExclusions);
    container.querySelectorAll('.delete-proxy-exclusion-rule').forEach(btn => btn.addEventListener('click', handleDeleteProxyExclusionRule));
    container.querySelectorAll('.proxy-exclusion-enable').forEach(checkbox => checkbox.addEventListener('change', handleToggleProxyExclusionEnable));
}

function handleAddProxyExclusionRule(event) {
    event.preventDefault();
    const form = event.target;
    const ruleType = form.querySelector('#proxyExclusionType').value;
    const pattern = form.querySelector('#proxyExclusionPattern').value.trim();
    const description = form.querySelector('#proxyExclusionDescription').value.trim();
    const isEnabled = form.querySelector('#proxyExclusionIsEnabled').checked;
    const messageArea = document.getElementById('proxyExclusionsMessage');

    if (!pattern) {
        messageArea.textContent = 'Pattern cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    const newRule = {
        id: `temp-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`, // Simple client-side ID
        rule_type: ruleType,
        pattern: pattern,
        description: description,
        is_enabled: isEnabled
    };
    currentProxyExclusionRules.push(newRule);
    form.reset(); // Reset form fields
    document.getElementById('proxyExclusionIsEnabled').checked = true; // Reset checkbox to default
    messageArea.textContent = 'Rule added locally. Click "Save All" to persist changes.';
    messageArea.className = 'message-area info-message';
    renderProxyExclusionUI(); // Re-render the list
}

function handleDeleteProxyExclusionRule(event) {
    const ruleId = event.target.closest('tr').getAttribute('data-rule-id');
    currentProxyExclusionRules = currentProxyExclusionRules.filter(rule => rule.id !== ruleId);
    document.getElementById('proxyExclusionsMessage').textContent = 'Rule marked for deletion. Click "Save All" to persist changes.';
    document.getElementById('proxyExclusionsMessage').className = 'message-area info-message';
    renderProxyExclusionUI();
}

function handleToggleProxyExclusionEnable(event) {
    const ruleId = event.target.closest('tr').getAttribute('data-rule-id');
    const rule = currentProxyExclusionRules.find(r => r.id === ruleId);
    if (rule) {
        rule.is_enabled = event.target.checked;
    }
    document.getElementById('proxyExclusionsMessage').textContent = 'Rule status changed locally. Click "Save All" to persist changes.';
    document.getElementById('proxyExclusionsMessage').className = 'message-area info-message';
    // No need to re-render fully, but "Save All" button should be prominent
}

async function handleSaveAllProxyExclusions() {
    const messageArea = document.getElementById('proxyExclusionsMessage');
    messageArea.textContent = 'Saving proxy exclusion rules...';
    messageArea.className = 'message-area';

    try {
        // Filter out temporary client-side IDs if backend generates its own on creation
        // For simplicity now, we send them as is; backend can re-assign IDs if needed.
        const rulesToSave = currentProxyExclusionRules.map(rule => {
            // If your backend re-assigns IDs and doesn't want temp IDs:
            // if (rule.id.startsWith('temp-')) {
            //     const { id, ...rest } = rule; // Exclude client-side temp ID
            //     return rest;
            // }
            return rule;
        });

        await apiService.setProxyExclusionRules(rulesToSave);
        messageArea.textContent = 'Proxy exclusion rules saved successfully!';
        messageArea.classList.add('success-message');
        // Fetch again to get server-assigned IDs if any and re-render
        await loadAndDisplayProxyExclusionSettings();
    } catch (error) {
        console.error("Error saving proxy exclusion rules:", error);
        messageArea.textContent = `Error saving rules: ${escapeHtml(error.message)}`;
        messageArea.classList.add('error-message');
    }
}
