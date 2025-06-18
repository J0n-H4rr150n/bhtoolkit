// static/views/synackMissionsView.js
import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

let apiService;
let uiService;
let stateService;
let tableService; // Added for table layout and resizing
let viewContentContainer;
let currentFullAppSettings = null; // To store the full settings loaded

// Module-level set to track unique missions seen by the UI in this browser session
let seenMissionIDsThisSession = new Set();

export function initSynackMissionsView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService;
    console.log("[SynackMissionsView] Initialized.");
}

async function loadAndDisplayMissionSettings(containerId = 'synackMissionsSettingsContainer') {
    const settingsContainer = document.getElementById(containerId);
    if (!settingsContainer) {
        console.error(`[SynackMissionsView] Settings container '${containerId}' not found.`);
        return;
    }

    settingsContainer.innerHTML = '<p>Loading settings...</p>';

    try {
        const appSettings = await apiService.getAppSettings();
        currentFullAppSettings = appSettings; // Store for saving
        const missionSettings = appSettings.missions || {};

        settingsContainer.innerHTML = `
            <form id="synackMissionsSettingsForm">
                <div class="form-group" style="margin-bottom: 15px;"> 
                    <label for="missionsEnabledToggle">Enable Mission Polling:</label>
                    <input type="checkbox" id="missionsEnabledToggle" name="enabled" ${missionSettings.Enabled ? 'checked' : ''}>
                </div>
                <div class="form-group">
                    <label for="missionsPollingInterval">Polling Interval (seconds):</label>
                    <input type="number" id="missionsPollingInterval" name="polling_interval_seconds" class="settings-input" value="${escapeHtmlAttribute(missionSettings.PollingIntervalSeconds || 10)}" min="5">
                </div>
                <div class="form-group" style="margin-bottom: 15px;">
                    <label for="missionsListURL">Mission List URL:</label>
                    <input type="text" id="missionsListURL" name="list_url" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ListURL || '')}" style="width: 100%;">
                </div>
                <div class="form-group" style="margin-bottom: 15px;">
                    <label for="missionsClaimURLPattern">Claim URL Pattern:</label>
                    <input type="text" id="missionsClaimURLPattern" name="claim_url_pattern" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ClaimURLPattern || '')}" style="width: 100%;">
                </div>
                <div class="form-group" style="margin-bottom: 15px;">
                    <label for="missionsClaimMinPayout">Min Payout to Claim ($):</label>
                    <input type="number" id="missionsClaimMinPayout" name="claim_min_payout" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ClaimMinPayout || 0)}" step="0.01">
                </div>
                <div class="form-group" style="margin-bottom: 15px;">
                    <label for="missionsClaimMaxPayout">Max Payout to Claim ($):</label>
                    <input type="number" id="missionsClaimMaxPayout" name="claim_max_payout" class="settings-input" value="${escapeHtmlAttribute(missionSettings.ClaimMaxPayout || 50)}" step="0.01">
                </div>
                <button type="submit" class="primary">Save Mission Settings</button>
            </form>
            <div id="synackMissionsMessage" class="message-area" style="margin-top: 10px;"></div>
        `;

        const settingsForm = document.getElementById('synackMissionsSettingsForm');
        if (settingsForm) {
            settingsForm.addEventListener('submit', handleSaveMissionSettings);
        }

    } catch (error) {
        console.error("Error loading mission settings:", error);
        settingsContainer.innerHTML = `<p class="error-message">Failed to load mission settings: ${escapeHtml(error.message)}</p>`;
        currentFullAppSettings = null; // Reset on error
    }
}

async function handleSaveMissionSettings(event) {
    event.preventDefault();
    const form = event.target;
    const messageArea = document.getElementById('synackMissionsMessage');
    if (!messageArea) return;

    messageArea.textContent = 'Saving...';
    messageArea.className = 'message-area info-message';

    // Ensure currentFullAppSettings is loaded, otherwise fetch it.
    if (!currentFullAppSettings) {
        try {
            currentFullAppSettings = await apiService.getAppSettings();
        } catch (error) {
            messageArea.textContent = `Error: Could not load current settings to perform save. ${escapeHtml(error.message)}`;
            messageArea.className = 'message-area error-message';
            return;
        }
    }
    
    // Construct the new mission settings from the form
    const newMissionSettings = {
        Enabled: form.elements.enabled.checked,
        PollingIntervalSeconds: parseInt(form.elements.polling_interval_seconds.value, 10) || 10,
        ListURL: form.elements.list_url.value.trim(),
        ClaimURLPattern: form.elements.claim_url_pattern.value.trim(),
        ClaimMinPayout: parseFloat(form.elements.claim_min_payout.value) || 0,
        ClaimMaxPayout: parseFloat(form.elements.claim_max_payout.value) || 0,
    };

    // Prepare the full payload, preserving other settings (like UI)
    const settingsToSave = {
        ...currentFullAppSettings, // Spread existing settings (includes UI, etc.)
        missions: newMissionSettings // Override with new mission settings
    };
    
    // Ensure the 'ui' part is present, even if it's just the existing one
    if (!settingsToSave.ui && currentFullAppSettings && currentFullAppSettings.ui) {
        settingsToSave.ui = currentFullAppSettings.ui;
    } else if (!settingsToSave.ui) {
        settingsToSave.ui = { ShowSynackSection: false }; // Default UI if none existed
    }


    try {
        await apiService.saveAppSettings(settingsToSave);
        currentFullAppSettings.missions = newMissionSettings; // Update local cache

        messageArea.textContent = 'Mission settings saved successfully! Restart the application for changes to take full effect.';
        messageArea.className = 'message-area success-message';
        uiService.showModalMessage('Settings Saved', 'Mission settings have been saved. A restart of the backend application is usually required for polling changes to take effect.');

    } catch (error) {
        console.error("Error saving mission settings:", error);
        messageArea.textContent = `Error saving mission settings: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
    }
}

async function loadAndDisplayMissionList(containerId = 'synackMissionsListContainer') {
    const listContainer = document.getElementById(containerId);
    if (!listContainer) {
        console.error(`[SynackMissionsView] Mission list container '${containerId}' not found.`);
        return;
    }

    listContainer.innerHTML = '<p>Loading observed missions...</p>';

    const appState = stateService.getState(); // Removed duplicated line from previous suggestion
    // Ensure pagination state for synackMissionsView exists
    if (!appState.paginationState.synackMissionsView) {
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                synackMissionsView: {
                    currentPage: 1,
                    limit: 25, // Default limit
                    sortBy: 'created_at', // Default sort
                    sortOrder: 'DESC',
                    totalPages: 0,
                    totalRecords: 0,
                }
            }
        });
    }
    const { currentPage, limit, sortBy, sortOrder } = appState.paginationState.synackMissionsView;
    const tableKey = 'synackMissionsTable'; // For layout saving
    const columnConfig = appState.paginationState.synackMissionsTableLayout; // From stateService.js defaults
    const globalTableLayouts = appState.globalTableLayouts || {};
    const savedTableWidths = globalTableLayouts[tableKey]?.columns || {};


    try {
        const params = { page: currentPage, limit, sort_by: sortBy, sort_order: sortOrder };
        const response = await apiService.getObservedMissions(params);
        const missions = response.Records || []; // Access the Records field

        // Update unique missions seen this session
        if (missions.length > 0) {
            missions.forEach(mission => {
                if (mission.SynackTaskID) { // Ensure SynackTaskID exists
                    seenMissionIDsThisSession.add(mission.SynackTaskID);
                }
            });
        }
        const totalUniqueMissionsSeenCount = seenMissionIDsThisSession.size;
        stateService.updateState({ totalMissionsSeenThisSession: totalUniqueMissionsSeenCount }); // Update stateService with the count

        // Create and display the status message
        const statusMessageText = `Total unique missions observed this session: ${totalUniqueMissionsSeenCount}. Last UI refresh: ${new Date().toLocaleTimeString()}.`;
        const statusElement = document.getElementById('synackMissionsStatus');
        if (statusElement) {
            statusElement.textContent = statusMessageText;
        }

        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                synackMissionsView: {
                    ...appState.paginationState.synackMissionsView,
                    currentPage: response.Page || 1,
                    limit: response.Limit || limit,
                    totalPages: response.TotalPages || 0,
                    totalRecords: response.TotalRecords || 0,
                }
            }
        });

        if (missions.length === 0) {
            listContainer.innerHTML = '<p>No missions observed or recorded yet.</p>';
            renderMissionListPaginationControls(document.getElementById('synackMissionsPaginationControls'));
            return;
        }

        let tableHTML = `
            <div style="margin-bottom: 10px; text-align: right;">
                <button id="saveSynackMissionsLayoutBtn" class="secondary small-button">Save Column Layout</button>
            </div>
            <table class="settings-table" style="table-layout: fixed;">
                <thead id="synackMissionsTableHead">
                    <tr>
                        <th style="width: ${savedTableWidths.id?.width || columnConfig.id.default};" class="sortable" data-sort-key="id" data-col-key="id" id="${columnConfig.id.id}">${columnConfig.id.label}</th>
                        <th style="width: ${savedTableWidths.title?.width || columnConfig.title.default};" class="sortable" data-sort-key="title" data-col-key="title" id="${columnConfig.title.id}">${columnConfig.title.label}</th>
                        <th style="width: ${savedTableWidths.payout_amount?.width || columnConfig.payout_amount.default};" class="sortable" data-sort-key="payout_amount" data-col-key="payout_amount" id="${columnConfig.payout_amount.id}">${columnConfig.payout_amount.label}</th>
                        <th style="width: ${savedTableWidths.status?.width || columnConfig.status.default};" class="sortable" data-sort-key="status" data-col-key="status" id="${columnConfig.status.id}">${columnConfig.status.label}</th>
                        <th style="width: ${savedTableWidths.updated_at?.width || columnConfig.updated_at.default};" class="sortable" data-sort-key="updated_at" data-col-key="updated_at" id="${columnConfig.updated_at.id}">${columnConfig.updated_at.label}</th>
                        <th style="width: ${savedTableWidths.claimed_by_toolkit_at?.width || columnConfig.claimed_by_toolkit_at.default};" class="sortable" data-sort-key="claimed_by_toolkit_at" data-col-key="claimed_by_toolkit_at" id="${columnConfig.claimed_by_toolkit_at.id}">${columnConfig.claimed_by_toolkit_at.label}</th>
                    </tr>
                </thead>
                <tbody>
        `;
        missions.forEach(mission => {
            // Use PascalCase field names from models.SynackMission
            tableHTML += `
                <tr>
                    <td>${escapeHtml(mission.SynackTaskID)}</td>
                    <td title="${escapeHtmlAttribute(mission.Title)}">${escapeHtml(mission.Title?.substring(0, 70) + (mission.Title?.length > 70 ? '...' : ''))}</td>
                    <td>${escapeHtml(mission.PayoutAmount.toFixed(2))} ${escapeHtml(mission.PayoutCurrency)}</td>
                    <td>${escapeHtml(mission.Status)}</td>
                    <td>${mission.UpdatedAt ? new Date(mission.UpdatedAt).toLocaleString() : (mission.CreatedAt ? new Date(mission.CreatedAt).toLocaleString() : 'N/A')}</td>
                    <td>${mission.ClaimedByToolkitAt ? new Date(mission.ClaimedByToolkitAt).toLocaleString() : 'N/A'}</td>
                </tr>
            `;
        });
        tableHTML += '</tbody></table>';
        listContainer.innerHTML = tableHTML;

        const saveLayoutBtn = document.getElementById('saveSynackMissionsLayoutBtn');
        if (saveLayoutBtn) {
            saveLayoutBtn.addEventListener('click', () => {
                tableService.saveCurrentTableLayout(tableKey, 'synackMissionsTableHead');
            });
        }

        document.querySelectorAll('#synackMissionsTableHead th.sortable').forEach(th => {
            th.addEventListener('click', (event) => handleMissionListSort(event.currentTarget.dataset.sortKey));
        });

        if (tableService) {
            tableService.makeTableColumnsResizable('synackMissionsTableHead', columnConfig);
        }
        renderMissionListPaginationControls(document.getElementById('synackMissionsPaginationControls'));

    } catch (error) {
        console.error("Error loading observed missions:", error);
        listContainer.innerHTML = `<p class="error-message">Failed to load observed missions: ${escapeHtml(error.message)}</p>`;
    }
}

function handleMissionListSort(sortKey) {
    if (!sortKey) return;
    const appState = stateService.getState();
    const currentSortState = appState.paginationState.synackMissionsView;
    let newSortOrder = 'ASC';
    if (currentSortState.sortBy === sortKey && currentSortState.sortOrder === 'ASC') {
        newSortOrder = 'DESC';
    }
    stateService.updateState({
        paginationState: {
            ...appState.paginationState,
            synackMissionsView: { ...currentSortState, sortBy: sortKey, sortOrder: newSortOrder, currentPage: 1 }
        }
    });
    loadAndDisplayMissionList('synackMissionsListContainer');
}

function renderMissionListPaginationControls(container) {
    if (!container) return;
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords } = appState.paginationState.synackMissionsView;

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total mission(s) found.</p>` : '';
        return;
    }
    let paginationHTML = `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total missions)</p>`;
    const buildHash = (page) => `#synack-missions?page=${page}`; // Simplified for now, add sort/filter later

    const prevButton = uiService.createButton('&laquo; Previous', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); }, { disabled: currentPage <= 1, classNames: ['secondary'], marginRight: '5px' });
    const nextButton = uiService.createButton('Next &raquo;', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); }, { disabled: currentPage >= totalPages, classNames: ['secondary'] });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    if (currentPage > 1) container.appendChild(prevButton);
    if (currentPage < totalPages) container.appendChild(nextButton);

    // TODO: Add items per page selector if needed
}

export async function loadSynackMissionsView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("[SynackMissionsView] viewContentContainer not provided!");
        return;
    }

    if (!apiService || !uiService || !stateService || !tableService) {
        console.error("[SynackMissionsView] Not initialized. Call initSynackMissionsView first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SynackMissionsView module not initialized.</p>";
        return;
    }

    viewContentContainer.innerHTML = `
        <h1>Synack Missions Configuration</h1>
        <div class="tabs" style="margin-bottom: 20px;">
            <button class="tab-button active" data-tab="synackMissionsListTab">Missions List</button>
            <button class="tab-button" data-tab="synackMissionsSettingsTab">Settings</button>
        </div>

        <div id="synackMissionsListTab" class="tab-content active">
            <h2>Observed Missions</h2>
            <div id="synackMissionsStatus" style="margin-bottom: 10px; padding: 8px; background-color: #f0f0f0; border-radius: 4px; font-style: italic;">Loading status...</div>
            <p>This table shows missions that have been observed and recorded by the toolkit.</p>
            <div id="synackMissionsListContainer" style="margin-top:15px;">
                <p>Loading mission list...</p>
            </div>
            <div id="synackMissionsPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
        </div>

        <div id="synackMissionsSettingsTab" class="tab-content">
            <h2>Mission Polling Settings</h2>
            <p>Configure the settings for the Synack mission polling and auto-claiming features.</p>
            <div id="synackMissionsSettingsContainer" style="margin-top:15px;">
                <p>Loading settings...</p>
            </div>
        </div>
    `;

    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.addEventListener('click', () => {
            document.querySelectorAll('.tabs .tab-button').forEach(btn => btn.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
            button.classList.add('active');
            document.getElementById(button.getAttribute('data-tab')).classList.add('active');
        });
    });

    // Initialize the status message area when the view loads
    const initialStatusElement = document.getElementById('synackMissionsStatus');
    if (initialStatusElement) {
        const currentCount = stateService.getState().totalMissionsSeenThisSession || 0;
        initialStatusElement.textContent = `Total unique missions observed this session: ${currentCount}.`;
    }
    await loadAndDisplayMissionList('synackMissionsListContainer'); // Load mission list by default
    await loadAndDisplayMissionSettings('synackMissionsSettingsContainer'); // Load settings for its tab
}
