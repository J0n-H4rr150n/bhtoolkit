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
            <button class="tab-button active" data-tab="uiSettingsSubTab">UI Settings</button>
            <button class="tab-button" data-tab="proxyExclusionsSubTab">Proxy Exclusions</button>
            <button class="tab-button" data-tab="tableLayoutsSubTab">Table Layouts</button>
            <button class="tab-button" data-tab="vulnerabilityTypesSubTab">Vulnerability Types</button>
            <button class="tab-button" data-tab="tagsManagementSubTab">Manage Tags</button>
        </div>

        <div id="uiSettingsSubTab" class="settings-sub-tab-content active">
            <h2>UI & Application Settings</h2>
            <div id="uiSettingsContainer">
                <p>Loading UI settings...</p>
            </div>
        </div>
        
        <div id="proxyExclusionsSubTab" class="settings-sub-tab-content">
            <h2>Global Proxy Exclusions</h2>
            <p>Define rules to prevent certain HTTP requests from being saved by the proxy.</p>
            <div id="proxyExclusionsMessage" class="message-area" style="margin-top: 10px;"></div>
            <div id="proxyExclusionsContainer" style="margin-top:15px;">
                <p>Loading proxy exclusion rules...</p>
            </div>
        </div>

        <div id="tableLayoutsSubTab" class="settings-sub-tab-content">
            <h2>Table Layouts</h2>
            <p>Manage saved column widths for various tables in the application.</p>
            <div id="tableLayoutsSettingsContainer" style="margin-top:15px;">
                <p>Loading table layout settings...</p>
            </div>
             <div id="tableLayoutsMessage" class="message-area" style="margin-top: 10px;"></div>
        </div>

        <div id="vulnerabilityTypesSubTab" class="settings-sub-tab-content">
            <h2>Manage Vulnerability Types</h2>
            <div id="vulnerabilityTypesContainer" style="margin-top:15px;"></div>
        </div>

        <div id="tagsManagementSubTab" class="settings-sub-tab-content">
            <h2>Global Tag Management</h2>
            <p>Create, edit, and delete tags used throughout the application.</p>
            <div id="tagsManagementMessage" class="message-area" style="margin-top: 10px;"></div>
            <div id="tagsManagementContainer" style="margin-top:15px;">
                <p>Loading tags...</p>
            </div>
        </div>
    `;

    console.log('[SettingsView.js] Tabbed HTML structure set for settings page.');
    
    // Initial tab display logic: Ensure only the active tab content is visible
    // and others are hidden. This makes the functionality independent of CSS display rules
    // tied to the 'active' class for content panes.
    document.querySelectorAll('.settings-sub-tab-content').forEach(content => {
        if (content.classList.contains('active')) {
            content.style.display = 'block';
        } else {
            content.style.display = 'none';
        }
    });
    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.addEventListener('click', () => {
            const tabId = button.getAttribute('data-tab');
            document.querySelectorAll('.tabs .tab-button').forEach(btn => btn.classList.remove('active'));
            document.querySelectorAll('.settings-sub-tab-content').forEach(content => content.classList.remove('active'));
            button.classList.add('active');
            document.getElementById(tabId).classList.add('active');
            
            // Explicitly manage display style for tab content
            document.querySelectorAll('.settings-sub-tab-content').forEach(content => {
                if (content.id === tabId) {
                    content.style.display = 'block';
                } else {
                    content.style.display = 'none';
                }
            });
            switch (tabId) {
                case 'uiSettingsSubTab':
                    loadAndDisplayUISettings();
                    break;
                case 'proxyExclusionsSubTab':
                    loadAndDisplayProxyExclusionSettings();
                    break;
                case 'tableLayoutsSubTab':
                    loadAndDisplayTableLayoutSettings();
                    break;
                case 'vulnerabilityTypesSubTab':
                    loadAndDisplayVulnerabilityTypes();
                    break;
                case 'tagsManagementSubTab':
                    loadAndDisplayTagsManagement();
                    break;
            }
        });
    });

    // Load the default active tab's content
    loadAndDisplayUISettings(); // Default active tab
}

async function loadAndDisplayUISettings() {
    const uiSettingsContainer = document.getElementById('uiSettingsContainer');
    if (!uiSettingsContainer) return;

    try {
        const appSettings = await apiService.getAppSettings();
        currentFullAppSettings = appSettings; 
        const uiSpecificSettings = appSettings.ui || {}; 
        const missionSettings = appSettings.missions || {}; 

        const proxyLogDefaultSavedValue = localStorage.getItem('proxyLogDefaultToResponseTab');
        uiSettingsContainer.innerHTML = `
            <div class="form-group">
                <label for="showSynackToggle">Show Synack Section in Sidebar:</label>
            <input type="checkbox" id="showSynackToggle" ${uiSpecificSettings.ShowSynackSection ? 'checked' : ''}>
            </div>
            <div class="form-group">
                <label for="defaultThemeToggle">Default to Dark Mode:</label>
                <input type="checkbox" id="defaultThemeToggle" ${uiSpecificSettings.DefaultTheme === 'dark' ? 'checked' : ''}>
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
        currentFullAppSettings = null; 
    }
}

async function handleSaveUISettings() {
    const showSynackToggle = document.getElementById('showSynackToggle');
    const proxyLogDefaultToggle = document.getElementById('proxyLogDefaultToResponseToggleSettings');
    const defaultThemeToggle = document.getElementById('defaultThemeToggle'); 
    const claimMinPayoutInput = document.getElementById('claimMinPayout');
    const claimMaxPayoutInput = document.getElementById('claimMaxPayout');
    const messageArea = document.getElementById('uiSettingsMessage');

    if (!messageArea) return; 

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    if (proxyLogDefaultToggle) {
        localStorage.setItem('proxyLogDefaultToResponseTab', proxyLogDefaultToggle.checked); 
    }

    if (!currentFullAppSettings) {
        try {
            currentFullAppSettings = await apiService.getAppSettings();
        } catch (error) {
            messageArea.textContent = `Error: Could not load current settings to perform save. ${escapeHtml(error.message)}`;
            messageArea.classList.add('error-message');
            return;
        }
    }

    const baseMissionSettings = currentFullAppSettings.missions ? { ...currentFullAppSettings.missions } : {
        enabled: false, 
        polling_interval_seconds: 10,
        list_url: "",
        claim_url_pattern: "",
        claim_min_payout: 0.0,
        claim_max_payout: 50.0
    };

    const settingsToSave = {
        ui: {
            ShowSynackSection: showSynackToggle ? showSynackToggle.checked : (currentFullAppSettings.ui?.ShowSynackSection || false),
            DefaultTheme: defaultThemeToggle ? (defaultThemeToggle.checked ? 'dark' : 'light') : (currentFullAppSettings.ui?.DefaultTheme || 'light')
        },
        missions: {
            ...baseMissionSettings, 
            ClaimMinPayout: parseFloat(claimMinPayoutInput.value) || 0,
            ClaimMaxPayout: parseFloat(claimMaxPayoutInput.value) || 0
        }
    };

    try {
        await apiService.saveAppSettings(settingsToSave); 
        currentFullAppSettings.ui = settingsToSave.ui;
        currentFullAppSettings.missions = settingsToSave.missions;

             messageArea.textContent = 'UI settings saved successfully! Refresh may be needed for sidebar changes.';
             messageArea.classList.add('success-message');
             uiService.showModalMessage('Settings Saved', 'UI settings have been saved. A page refresh might be required to see all changes (like sidebar visibility).');
            if (localApplyUiSettingsFunc) {
                localApplyUiSettingsFunc(settingsToSave.ui); 
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
        const layouts = await apiService.getTableLayouts(); 
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
                const response = await fetch(`${apiService.API_BASE || '/api'}/settings/table-column-widths/reset`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                });

                if (response.ok) {
                    messageArea.textContent = 'All table layouts have been reset successfully.';
                    messageArea.classList.add('success-message');
                    uiService.showModalMessage('Layouts Reset', 'All table layouts have been reset. Refresh pages to see changes.');
                    await loadAndDisplayTableLayoutSettings();
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
        currentProxyExclusionRules.forEach(rule => { 
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
        tableHTML += `</tbody></table>`;
    }

    const saveButtonHTML = `<button id="saveProxyExclusionsBtn" class="primary" style="margin-top: 10px; margin-bottom: 15px;">Save All Proxy Exclusions</button>`;

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
        id: `temp-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`, 
        rule_type: ruleType,
        pattern: pattern,
        description: description,
        is_enabled: isEnabled
    };
    currentProxyExclusionRules.push(newRule);
    form.reset(); 
    document.getElementById('proxyExclusionIsEnabled').checked = true; 
    messageArea.textContent = 'Rule added locally. Click "Save All" to persist changes.';
    messageArea.className = 'message-area info-message';
    renderProxyExclusionUI(); 
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
}

async function handleSaveAllProxyExclusions() {
    const messageArea = document.getElementById('proxyExclusionsMessage');
    messageArea.textContent = 'Saving proxy exclusion rules...';
    messageArea.className = 'message-area';

    try {
        const rulesToSave = currentProxyExclusionRules.map(rule => {
            return rule;
        });

        await apiService.setProxyExclusionRules(rulesToSave);
        messageArea.textContent = 'Proxy exclusion rules saved successfully!';
        messageArea.classList.add('success-message');
        await loadAndDisplayProxyExclusionSettings();
    } catch (error) {
        console.error("Error saving proxy exclusion rules:", error);
        messageArea.textContent = `Error saving rules: ${escapeHtml(error.message)}`;
        messageArea.classList.add('error-message');
    }
}

// --- Tags Management Functions ---

async function loadAndDisplayTagsManagement() {
    const container = document.getElementById('tagsManagementContainer');
    if (!container) return;

    container.innerHTML = `
        <div id="addTagFormContainer" style="margin-bottom: 20px; padding: 15px; border: 1px solid #ddd; border-radius: 4px;">
            <h4>Add New Tag</h4>
            <form id="addGlobalTagForm" class="inline-form">
                <div class="form-group">
                    <label for="newGlobalTagName">Name:</label>
                    <input type="text" id="newGlobalTagName" name="name" required>
                </div>
                <div class="form-group">
                    <label for="newGlobalTagColor">Color:</label>
                    <input type="color" id="newGlobalTagColor" name="color" value="#6c757d"> 
                </div>
                <button type="submit" class="primary">Add Tag</button>
            </form>
            <div id="addGlobalTagMessage" class="message-area" style="margin-top: 10px;"></div>
        </div>
        <h4>Existing Tags</h4>
        <div id="existingTagsListContainer">
            <p>Loading tags...</p>
        </div>
    `;

    document.getElementById('addGlobalTagForm')?.addEventListener('submit', handleAddGlobalTag);
    await fetchAndRenderGlobalTags();
}

async function fetchAndRenderGlobalTags() {
    const listContainer = document.getElementById('existingTagsListContainer');
    if (!listContainer) return;

    try {
        const tags = await apiService.getAllTags();
        if (tags && tags.length > 0) {
            let tableHTML = `<table class="settings-table">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Color</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>`;
            tags.forEach(tag => {
                tableHTML += `
                    <tr data-tag-id="${tag.id}">
                        <td class="global-tag-name" data-current-name="${escapeHtmlAttribute(tag.name)}">${escapeHtml(tag.name)}</td>
                        <td class="global-tag-color" data-current-color="${escapeHtmlAttribute(tag.color?.String || '#6c757d')}">
                            <span style="display: inline-block; width: 20px; height: 20px; background-color: ${escapeHtmlAttribute(tag.color?.String || '#6c757d')}; border: 1px solid #ccc; border-radius: 3px; vertical-align: middle;"></span>
                            <span style="margin-left: 5px;">${escapeHtml(tag.color?.String || '#6c757d')}</span>
                        </td>
                        <td>
                            <button class="action-button edit-global-tag" data-tag-id="${tag.id}" title="Edit Tag">✏️</button>
                            <button class="action-button delete-global-tag" data-tag-id="${tag.id}" title="Delete Tag">🗑️</button>
                        </td>
                    </tr>`;
            });
            tableHTML += `</tbody></table>`;
            listContainer.innerHTML = tableHTML;

            listContainer.querySelectorAll('.edit-global-tag').forEach(btn => btn.addEventListener('click', handleEditGlobalTagClick));
            listContainer.querySelectorAll('.delete-global-tag').forEach(btn => btn.addEventListener('click', handleDeleteGlobalTagClick));
        } else {
            listContainer.innerHTML = '<p>No tags created yet.</p>';
        }
    } catch (error) {
        console.error("Error fetching global tags:", error);
        listContainer.innerHTML = `<p class="error-message">Error loading tags: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleAddGlobalTag(event) {
    event.preventDefault();
    const form = event.target;
    const tagName = form.elements.name.value.trim();
    const tagColor = form.elements.color.value;
    const messageArea = document.getElementById('addGlobalTagMessage');

    if (!tagName) {
        messageArea.textContent = 'Tag name cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    try {
        await apiService.createTag({ name: tagName, color: tagColor });
        messageArea.textContent = `Tag "${escapeHtml(tagName)}" added successfully.`;
        messageArea.className = 'message-area success-message';
        form.reset();
        document.getElementById('newGlobalTagColor').value = '#6c757d'; 
        await fetchAndRenderGlobalTags(); 
    } catch (error) {
        console.error("Error adding global tag:", error);
        messageArea.textContent = `Error adding tag: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
    }
}

function handleEditGlobalTagClick(event) {
    const tagId = event.target.closest('button').dataset.tagId;
    const row = event.target.closest('tr');
    if (!row.classList.contains('editing-tag')) {
        const currentlyEditingRow = document.querySelector('tr.editing-tag');
        if (currentlyEditingRow && currentlyEditingRow !== row) {
            cancelTagEdit(currentlyEditingRow.dataset.tagId);
        }

        row.classList.add('editing-tag');
        const nameCell = row.querySelector('.global-tag-name');
        const colorCell = row.querySelector('.global-tag-color');
        const actionsCell = row.querySelector('td:last-child');

        const currentName = nameCell.dataset.currentName;
        const currentColor = colorCell.dataset.currentColor;

        nameCell.innerHTML = `<input type="text" class="edit-tag-name-input" value="${escapeHtmlAttribute(currentName)}" style="width: 90%;">`;
        colorCell.innerHTML = `<input type="color" class="edit-tag-color-input" value="${escapeHtmlAttribute(currentColor)}">`;
        actionsCell.innerHTML = `
            <button class="action-button save-tag-edit" data-tag-id="${tagId}" title="Save Changes">✔️</button>
            <button class="action-button cancel-tag-edit" data-tag-id="${tagId}" title="Cancel Edit">❌</button>
        `;

        actionsCell.querySelector('.save-tag-edit').addEventListener('click', () => saveTagEdit(tagId));
        actionsCell.querySelector('.cancel-tag-edit').addEventListener('click', () => cancelTagEdit(tagId));
        nameCell.querySelector('input').focus();
    }
}

async function saveTagEdit(tagId) {
    const row = document.querySelector(`tr[data-tag-id="${tagId}"]`);
    if (!row) return;

    const nameInput = row.querySelector('.edit-tag-name-input');
    const colorInput = row.querySelector('.edit-tag-color-input');
    const messageArea = document.getElementById('tagsManagementMessage');

    const newName = nameInput.value.trim();
    const newColor = colorInput.value;

    if (!newName) {
        messageArea.textContent = 'Tag name cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    try {
        await apiService.updateTag(tagId, { name: newName, color: newColor });
        messageArea.textContent = `Tag ID ${tagId} updated successfully.`;
        messageArea.className = 'message-area success-message';
        await fetchAndRenderGlobalTags(); 
    } catch (error) {
        console.error(`Error updating tag ${tagId}:`, error);
        messageArea.textContent = `Error updating tag: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
        cancelTagEdit(tagId); 
    }
}

function cancelTagEdit(tagId) {
    const row = document.querySelector(`tr[data-tag-id="${tagId}"]`);
    if (!row || !row.classList.contains('editing-tag')) return;

    row.classList.remove('editing-tag');
    fetchAndRenderGlobalTags(); 
}

async function handleDeleteGlobalTagClick(event) {
    const tagId = event.target.closest('button').dataset.tagId;
    const row = event.target.closest('tr');
    const tagName = row.querySelector('.global-tag-name').dataset.currentName;
    const messageArea = document.getElementById('tagsManagementMessage');

    uiService.showModalConfirm(
        "Confirm Delete Tag",
        `Are you sure you want to delete the tag "${escapeHtml(tagName)}" (ID: ${tagId})? This will also remove its associations from all items.`,
        async () => {
            try {
                await apiService.deleteTag(tagId);
                messageArea.textContent = `Tag "${escapeHtml(tagName)}" deleted successfully.`;
                messageArea.className = 'message-area success-message';
                await fetchAndRenderGlobalTags(); 
            } catch (error) {
                console.error(`Error deleting tag ${tagId}:`, error);
                messageArea.textContent = `Error deleting tag: ${escapeHtml(error.message)}`;
                messageArea.className = 'message-area error-message';
            }
        }
    );
}

// --- Vulnerability Types Management ---

async function loadAndDisplayVulnerabilityTypes() {
    const container = document.getElementById('vulnerabilityTypesContainer');
    if (!container) return;

    container.innerHTML = `
        <div id="addVulnerabilityTypeFormContainer" style="margin-bottom: 20px; padding: 15px; border: 1px solid #ddd; border-radius: 4px;">
            <h4>Add New Vulnerability Type</h4>
            <form id="addVulnerabilityTypeForm" class="inline-form">
                <div class="form-group">
                    <label for="newVulnerabilityTypeName">Name:</label>
                    <input type="text" id="newVulnerabilityTypeName" name="name" required>
                </div>
                <div class="form-group">
                    <label for="newVulnerabilityTypeDescription">Description:</label>
                    <input type="text" id="newVulnerabilityTypeDescription" name="description">
                </div>
                <button type="submit" class="primary">Add Type</button>
            </form>
            <div id="addVulnerabilityTypeMessage" class="message-area" style="margin-top: 10px;"></div>
        </div>
        <h4>Existing Vulnerability Types</h4>
        <div id="existingVulnerabilityTypesListContainer">
            <p>Loading vulnerability types...</p>
        </div>
    `;

    document.getElementById('addVulnerabilityTypeForm')?.addEventListener('submit', handleAddVulnerabilityType);
    await fetchAndRenderVulnerabilityTypes();
}

async function fetchAndRenderVulnerabilityTypes() {
    const listContainer = document.getElementById('existingVulnerabilityTypesListContainer');
    if (!listContainer) return;

    try {
        const types = await apiService.getAllVulnerabilityTypes();
        if (types && types.length > 0) {
            let tableHTML = `<table class="settings-table">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Name</th>
                        <th>Description</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>`;
            types.forEach(type => {
                tableHTML += `
                    <tr data-type-id="${type.id}">
                        <td>${type.id}</td>
                        <td class="vuln-type-name" data-current-name="${escapeHtmlAttribute(type.name)}">${escapeHtml(type.name)}</td>
                        <td class="vuln-type-description" data-current-description="${escapeHtmlAttribute(type.description?.String || '')}">${escapeHtml(type.description?.String || '')}</td>
                        <td>
                            <button class="action-button edit-vuln-type" data-type-id="${type.id}" title="Edit Type">✏️</button>
                            <button class="action-button delete-vuln-type" data-type-id="${type.id}" title="Delete Type">🗑️</button>
                        </td>
                    </tr>`;
            });
            tableHTML += `</tbody></table>`;
            listContainer.innerHTML = tableHTML;

            listContainer.querySelectorAll('.edit-vuln-type').forEach(btn => btn.addEventListener('click', handleEditVulnerabilityTypeClick));
            listContainer.querySelectorAll('.delete-vuln-type').forEach(btn => btn.addEventListener('click', handleDeleteVulnerabilityTypeClick));
        } else {
            listContainer.innerHTML = '<p>No vulnerability types defined yet.</p>';
        }
    } catch (error) {
        console.error("Error fetching vulnerability types:", error);
        listContainer.innerHTML = `<p class="error-message">Error loading vulnerability types: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleAddVulnerabilityType(event) {
    event.preventDefault();
    const form = event.target;
    const name = form.elements.name.value.trim();
    const description = form.elements.description.value.trim();
    const messageArea = document.getElementById('addVulnerabilityTypeMessage');

    if (!name) {
        messageArea.textContent = 'Vulnerability type name cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    try {
        await apiService.createVulnerabilityType({ name, description: description || null });
        messageArea.textContent = `Vulnerability type "${escapeHtml(name)}" added successfully.`;
        messageArea.className = 'message-area success-message';
        form.reset();
        await fetchAndRenderVulnerabilityTypes();
    } catch (error) {
        messageArea.textContent = `Error adding type: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
    }
}

function handleEditVulnerabilityTypeClick(event) {
    const typeId = event.target.closest('button').dataset.typeId;
    const row = event.target.closest('tr');
    if (!row || row.classList.contains('editing-vuln-type')) return;

    // Cancel any other active edit in this table
    const currentlyEditingRow = document.querySelector('tr.editing-vuln-type');
    if (currentlyEditingRow && currentlyEditingRow !== row) {
        cancelVulnerabilityTypeEdit();
    }

    row.classList.add('editing-vuln-type');
    const nameCell = row.querySelector('.vuln-type-name');
    const descriptionCell = row.querySelector('.vuln-type-description');
    const actionsCell = row.querySelector('td:last-child');

    const currentName = nameCell.dataset.currentName;
    const currentDescription = descriptionCell.dataset.currentDescription;

    nameCell.innerHTML = `<input type="text" class="edit-vuln-type-name-input" value="${escapeHtmlAttribute(currentName)}" style="width: 95%;">`;
    descriptionCell.innerHTML = `<input type="text" class="edit-vuln-type-description-input" value="${escapeHtmlAttribute(currentDescription)}" style="width: 95%;">`;
    actionsCell.innerHTML = `
        <button class="action-button save-vuln-type-edit" data-type-id="${typeId}" title="Save Changes">✔️</button>
        <button class="action-button cancel-vuln-type-edit" data-type-id="${typeId}" title="Cancel Edit">❌</button>
    `;

    actionsCell.querySelector('.save-vuln-type-edit').addEventListener('click', () => saveVulnerabilityTypeEdit(typeId));
    actionsCell.querySelector('.cancel-vuln-type-edit').addEventListener('click', () => cancelVulnerabilityTypeEdit());
    nameCell.querySelector('input').focus();
}

async function saveVulnerabilityTypeEdit(typeId) {
    const row = document.querySelector(`tr[data-type-id="${typeId}"]`);
    if (!row) return;

    const nameInput = row.querySelector('.edit-vuln-type-name-input');
    const descriptionInput = row.querySelector('.edit-vuln-type-description-input');
    const messageArea = document.getElementById('addVulnerabilityTypeMessage');

    const newName = nameInput.value.trim();
    const newDescription = descriptionInput.value.trim();

    if (!newName) {
        messageArea.textContent = 'Vulnerability type name cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    try {
        await apiService.updateVulnerabilityType(typeId, { name: newName, description: newDescription });
        messageArea.textContent = `Vulnerability type ID ${typeId} updated successfully.`;
        messageArea.className = 'message-area success-message';
        await fetchAndRenderVulnerabilityTypes(); // Refresh the list to show updated data and remove edit fields
    } catch (error) {
        console.error(`Error updating vulnerability type ${typeId}:`, error);
        messageArea.textContent = `Error updating type: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
        cancelVulnerabilityTypeEdit(); // Revert on error
    }
}

function cancelVulnerabilityTypeEdit() {
    // The simplest way to cancel is to just re-render the whole list.
    // This ensures data consistency and removes the edit-mode row.
    fetchAndRenderVulnerabilityTypes();
}

async function handleDeleteVulnerabilityTypeClick(event) {
    const typeId = event.target.closest('button').dataset.typeId;
    const row = event.target.closest('tr');
    const typeName = row.querySelector('.vuln-type-name')?.dataset.currentName || `ID ${typeId}`;
    const messageArea = document.getElementById('addVulnerabilityTypeMessage'); // Reuse message area

    uiService.showModalConfirm(
        "Confirm Delete",
        `Are you sure you want to delete vulnerability type "${escapeHtml(typeName)}"? This might affect existing findings.`,
        async () => {
            try {
                await apiService.deleteVulnerabilityType(typeId);
                if(messageArea) {
                    messageArea.textContent = `Vulnerability type "${escapeHtml(typeName)}" deleted.`;
                    messageArea.className = 'message-area success-message';
                }
                await fetchAndRenderVulnerabilityTypes();
            } catch (error) {
                if(messageArea) {
                    messageArea.textContent = `Error deleting type: ${escapeHtml(error.message)}`;
                    messageArea.className = 'message-area error-message';
                }
                 // If the error indicates it's in use, show a more specific message
                if (error.message && error.message.includes("FOREIGN KEY constraint failed")) {
                    uiService.showModalMessage("Deletion Failed", `Cannot delete type "${escapeHtml(typeName)}" as it is currently associated with one or more findings.`);
                }
            }
        }
    );
}
