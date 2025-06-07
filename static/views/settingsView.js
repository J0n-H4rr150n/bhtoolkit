import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
// tableService might not be directly needed here unless settings involve table layouts directly

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
    console.log("[SettingsView] Initialized.");
}

/**
 * Loads the settings view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadSettingsView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadSettingsView!");
        return;
    }
    if (!apiService || !uiService || !stateService) {
        console.error("SettingsView not initialized. Call initSettingsView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SettingsView module not initialized. Critical services are missing.</p>";
        return;
    }

    // Placeholder content for now. This needs to be expanded to include actual settings controls.
    viewContentContainer.innerHTML = `
        <h1>Settings</h1>
        <p>Configuration settings UI will be here.</p>
        
        <div class="settings-section">
            <h2>UI Settings</h2>
            <div id="uiSettingsContainer">
                <p>Loading UI settings...</p>
            </div>
        </div>

        <div class="settings-section" style="margin-top: 30px;">
            <h2>Other Settings</h2>
            <p>Future application-wide settings can be managed here.</p>
            <!-- Example: <button id="resetAllTableLayoutsBtn" class="danger">Reset All Table Layouts</button> -->
        </div>
    `;

    await loadAndDisplayUISettings();
    // Add event listeners for any settings controls here
    // e.g., document.getElementById('resetAllTableLayoutsBtn')?.addEventListener('click', handleResetAllTableLayouts);
}

async function loadAndDisplayUISettings() {
    const uiSettingsContainer = document.getElementById('uiSettingsContainer');
    if (!uiSettingsContainer) return;

    try {
        const settings = await apiService.getUISettings();
        uiSettingsContainer.innerHTML = `
            <div class="form-group">
                <label for="showSynackToggle">Show Synack Section in Sidebar:</label>
                <input type="checkbox" id="showSynackToggle" ${settings.showSynackSection ? 'checked' : ''}>
            </div>
            <button id="saveUISettingsBtn" class="primary">Save UI Settings</button>
            <div id="uiSettingsMessage" class="message-area" style="margin-top: 10px;"></div>
        `;

        document.getElementById('saveUISettingsBtn')?.addEventListener('click', handleSaveUISettings);

    } catch (error) {
        console.error("Error loading UI settings:", error);
        uiSettingsContainer.innerHTML = `<p class="error-message">Failed to load UI settings: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleSaveUISettings() {
    const showSynackToggle = document.getElementById('showSynackToggle');
    const messageArea = document.getElementById('uiSettingsMessage');
    if (!showSynackToggle || !messageArea) return;

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    const newSettings = {
        showSynackSection: showSynackToggle.checked
    };

    try {
        // Assuming your apiService will have a method like `saveUISettings`
        // For now, let's mock this or assume it's part of a general settings update endpoint.
        // If you have a specific endpoint like PUT /api/ui-settings:
        const response = await fetch(`${apiService.API_BASE || '/api'}/ui-settings`, { // Assuming API_BASE is accessible or default
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newSettings)
        });

        if (response.ok) {
            messageArea.textContent = 'UI settings saved successfully! Refresh may be needed for sidebar changes.';
            messageArea.classList.add('success-message');
            uiService.showModalMessage('Settings Saved', 'UI settings have been saved. A page refresh might be required to see all changes (like sidebar visibility).');
            // Optionally, trigger a soft refresh or update relevant parts of the UI if possible without full reload
        } else {
            const errorData = await response.json();
            messageArea.textContent = `Error saving UI settings: ${escapeHtml(errorData.message || response.statusText)}`;
            messageArea.classList.add('error-message');
        }
    } catch (error) {
        console.error("Error saving UI settings:", error);
        messageArea.textContent = `Network error saving UI settings: ${escapeHtml(error.message)}`;
        messageArea.classList.add('error-message');
    }
}
