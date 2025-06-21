// static/app.js
import { initState, getState, updateState } from './stateService.js';
import { initRouter } from './router.js';
import { initUIService, showModalMessage, showModalConfirm, updateBreadcrumbs, createButton, createSelect } from './uiService.js';
import * as apiService from './apiService.js'; // Keep as is
import { initTableService, saveCurrentTableLayout, makeTableColumnsResizable, getIsResizing } from './tableService.js'; // Import getIsResizing
import { escapeHtml, escapeHtmlAttribute, debounce, copyToClipboard, downloadCSV } from './utils.js';

// View Module Imports
import {
    loadPlatformsView as loadPlatformsViewModule,
    handleAddPlatform as handleAddPlatformModule,
    fetchAndDisplayPlatforms as fetchAndDisplayPlatformsModule
} from './views/platformView.js';
import { initTargetView, loadTargetsView, cancelActiveTargetEdit, importScopeRulesFromClipboard } from './views/targetView.js';
import { initProxyLogView, loadProxyLogView, loadProxyLogDetailView, fetchAndDisplayProxyLogs } from './views/proxyLogView.js';
import { initSynackView, loadSynackTargetsView, loadSynackAnalyticsView } from './views/synackView.js';
import { initChecklistView, fetchAndDisplayChecklistItems, cancelActiveChecklistItemEdit } from './views/checklistView.js';
import { initChecklistTemplateView, loadChecklistTemplatesView } from './views/checklistTemplateView.js';
import { initSettingsView, loadSettingsView } from './views/settingsView.js';
import { initSitemapView, loadSitemapView } from './views/sitemapView.js';
import { initModifierView, loadModifierView } from './views/modifierView.js';
import { initPageSitemapView, loadPageSitemapView } from './views/pageSitemapView.js';
import { initDomainDetailView, loadDomainDetailView } from './views/domainDetailView.js';
import { initVisualizerView, loadVisualizerView } from './views/visualizerView.js';
import { initDomainsView, loadDomainsView } from './views/domainsView.js';
import { initSynackMissionsView, loadSynackMissionsView } from './views/synackMissionsView.js';

// For EasyMDE instances
let easyMDEInstances = {};

document.addEventListener('DOMContentLoaded', async function() {
    const sidebar = document.getElementById('leftSidebar');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const viewContentContainer = document.getElementById('viewContentContainer');
    const sidebarItems = document.querySelectorAll('.sidebar-item');
    console.log('[App.js] DOMContentLoaded. viewContentContainer:', viewContentContainer); // Log container
    const currentTargetDisplay = document.getElementById('currentPlatformTarget');

    const modalOverlay = document.getElementById('customModal');
    const modalTitleElem = document.getElementById('modalTitle');
    const modalMessageElem = document.getElementById('modalMessage');
    const modalConfirmBtnElem = document.getElementById('modalConfirmBtn');
    const modalCancelBtnElem = document.getElementById('modalCancelBtn');
    const modalOkBtnElem = document.getElementById('modalOkBtn');
    const breadcrumbContainerElem = document.getElementById('breadcrumbContainer');
    const themeToggleBtn = document.getElementById('themeToggleBtn');

    const API_BASE_URL_CONST = '/api';

    if (sidebarToggle && sidebar) {
        sidebarToggle.addEventListener('click', function() {
            sidebar.classList.toggle('collapsed');
            const isCollapsed = sidebar.classList.contains('collapsed');
            const toggleButtonText = sidebarToggle.querySelector('.sidebar-item-text');
            if (toggleButtonText) {
                toggleButtonText.textContent = isCollapsed ? "Expand Menu" : "Collapse Menu";
            }
            localStorage.setItem('sidebarCollapsed', isCollapsed);
        });
        if (localStorage.getItem('sidebarCollapsed') === 'true') {
            sidebar.classList.add('collapsed');
            const toggleButtonText = sidebarToggle.querySelector('.sidebar-item-text');
             if (toggleButtonText) {
                toggleButtonText.textContent = "Expand Menu";
             }
        }
    }

    // Theme Toggling Logic
    const THEME_KEY = 'bhtoolkit-theme';
    const DARK_MODE_CLASS = 'dark-mode';
    const SUN_ICON = '☀️';
    const MOON_ICON = '🌙'; // Default icon for light mode

    function applyTheme(theme) {
        if (theme === 'dark') {
            console.log('[App.js] Applying dark mode.');
            document.body.classList.add(DARK_MODE_CLASS);
            if (themeToggleBtn) themeToggleBtn.innerHTML = SUN_ICON;
        } else {
            document.body.classList.remove(DARK_MODE_CLASS);
            if (themeToggleBtn) themeToggleBtn.innerHTML = MOON_ICON;
        }
        localStorage.setItem(THEME_KEY, theme);
    }

    const stateServiceAPI = {
        getState,
        updateState
    };

    const uiServiceAPI = {
        showModalMessage,
        showModalConfirm,
        updateBreadcrumbs,
        createButton,
        createSelect
    };

    const tableServiceAPI = {
        saveCurrentTableLayout,
        makeTableColumnsResizable,
        getIsResizing // Add getIsResizing here
    };

    apiService.initApiService(API_BASE_URL_CONST);
    initUIService({
        modalOverlay: modalOverlay,
        modalTitle: modalTitleElem,
        modalMessage: modalMessageElem,
        modalConfirmBtn: modalConfirmBtnElem,
        modalCancelBtn: modalCancelBtnElem,
        modalOkBtn: modalOkBtnElem,
        breadcrumbContainer: breadcrumbContainerElem
    });
    initTableService({
        showModalMessage: showModalMessage,
        saveTableLayouts: apiService.saveTableLayouts,
        getGlobalTableLayouts: () => {
            const state = getState();
            return state.globalTableLayouts || {}; // Ensure it returns an object even if undefined
        },
        updateGlobalTableLayouts: (newLayouts) => updateState({ globalTableLayouts: newLayouts })
    });

    initTargetView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI }, currentTargetDisplay);
    initProxyLogView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initSynackView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI });
    initChecklistView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initChecklistTemplateView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initSettingsView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, applyUiSettingsFunc: applyUiSettings });
    initSitemapView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initModifierView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initPageSitemapView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI });
    initDomainDetailView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI });
    initVisualizerView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI });
    initDomainsView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });
    initSynackMissionsView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });

    await fetchAndSetInitialCurrentTarget();

    // After fetching initial settings (which includes UI config), apply the theme
    // Determine initial theme:
    // 1. Check localStorage (user preference)
    // 2. Check backend config defaultTheme
    // 3. Fallback to 'light'
    const appSettings = getState().appSettings || {}; // Get settings fetched by fetchAndSetInitialCurrentTarget
    const configDefaultTheme = appSettings.ui?.DefaultTheme || 'light'; // Use config default
    const initialTheme = localStorage.getItem(THEME_KEY) || configDefaultTheme;
    console.log(`[App.js] Initial theme determined: ${initialTheme} (localStorage: ${localStorage.getItem(THEME_KEY)}, configDefault: ${configDefaultTheme})`);
    applyTheme(initialTheme);

    if (themeToggleBtn) {
        themeToggleBtn.addEventListener('click', () => {
            const currentTheme = localStorage.getItem(THEME_KEY) || 'light'; // Always base toggle on current state
            const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
            applyTheme(newTheme);
        });
    }

    initRouter({
        viewContentContainer,
        sidebarItems,
        updateBreadcrumbs,
        showModalConfirm,
        showModalMessage,
        API_BASE_URL: API_BASE_URL_CONST,
        getState,
        setState: updateState,
        viewLoaders: {
            loadPlatformsViewModule,
            handleAddPlatformModule,
            fetchAndDisplayPlatformsModule,
            loadTargetsView: (platformId) => loadTargetsView(viewContentContainer, platformId), // This is in targetView.js
            loadCurrentTargetView: (targetId, tab) => loadCurrentTargetView(targetId, tab), // Pass tab here
            loadSynackTargetsView: () => loadSynackTargetsView(viewContentContainer),
            loadSynackAnalyticsView: () => loadSynackAnalyticsView(viewContentContainer), // This is in synackView.js
            loadProxyLogView: async (proxyLogParams) => { // This is in proxyLogView.js
                await loadProxyLogView(viewContentContainer, proxyLogParams); // Load static HTML
                await fetchAndDisplayProxyLogs(proxyLogParams); // Then fetch and display data
            },
            loadProxyLogDetailView: (logId) => loadProxyLogDetailView(viewContentContainer, logId),
            loadChecklistTemplatesView: () => loadChecklistTemplatesView(viewContentContainer),
            loadSettingsView: () => {
                console.log('[App.js] Router attempting to call loadSettingsView.');
                loadSettingsView(viewContentContainer);
            },
            loadSitemapView: () => loadSitemapView(viewContentContainer),
            loadModifierView: (params) => loadModifierView(viewContentContainer, params),
            loadPageSitemapView: () => loadPageSitemapView(viewContentContainer),
            loadVisualizerView: () => loadVisualizerView(viewContentContainer),
            loadDomainDetailView: (container, domainId) => loadDomainDetailView(container, domainId),
            loadDomainsView: () => loadDomainsView(viewContentContainer),
            loadSynackMissionsView: () => loadSynackMissionsView(viewContentContainer)
        },
        getPlatformDetailsFunc: apiService.getPlatformDetails,
        cancelTargetEditFunc: cancelActiveTargetEdit,
        cancelChecklistItemEditFunc: cancelActiveChecklistItemEdit
    });

    function localFormatHeadersForFinding(headersObj) { // Helper for formatting headers in finding description
        if (!headersObj || Object.keys(headersObj).length === 0) return '(No Headers)';
        return Object.entries(headersObj)
            .map(([key, value]) => `${escapeHtml(key)}: ${escapeHtml(Array.isArray(value) ? value.join(', ') : value)}`)
            .join('\n');
    }

    async function loadCurrentTargetView(targetIdFromParam = null, tabToMakeActive = 'checklistTab') {
        const appState = getState();
        const targetIdToLoad = targetIdFromParam !== null ? parseInt(targetIdFromParam, 10) : appState.currentTargetId;

        if (targetIdToLoad) {
             try {
                const target = await apiService.getTargetDetails(targetIdToLoad);
                let platformNameForBreadcrumb = `Platform ${target.platform_id}`;
                const platformDetails = await apiService.getPlatformDetails(target.platform_id);
                if (platformDetails) platformNameForBreadcrumb = platformDetails.name;

                updateBreadcrumbs([
                    { name: "Platforms", hash: "#platforms" },
                    { name: escapeHtml(platformNameForBreadcrumb), hash: `#targets?platform_id=${target.platform_id}` },
                    { name: `Target: ${escapeHtml(target.codename)}` }
                ]);
                document.title = `Target: ${escapeHtml(target.codename)} - Toolkit`;
                if (!viewContentContainer) { console.error("viewContentContainer not found!"); return; }
                viewContentContainer.innerHTML = `
                    <h1>Target: ${escapeHtml(target.codename)}</h1>
                    <div style="display: flex; align-items: center; gap: 15px; margin-top: 10px; margin-bottom: 10px; flex-wrap: wrap;">
                        <div style="display: flex; align-items: center; gap: 5px;">
                            ${target.id === appState.currentTargetId
                                ? '<button id="clearCurrentTargetBtn" class="action-button" title="Clear This as Current Target" style="font-size: 1.2em; padding: 2px 5px; color: #e74c3c;">❌</button>'
                                : `<button class="action-button set-current-target primary" data-id="${target.id}" data-name="${escapeHtml(target.codename)}" title="Set as Current Target" style="font-size:1.2em; padding: 2px 5px;">📍</button>`}
                            <span style="font-weight: bold;">${escapeHtml(target.codename)}</span>
                        </div>
                        <span class="target-detail-item"><strong>ID:</strong> ${target.id}</span>
                        <span class="target-detail-item"><strong>Slug:</strong> ${escapeHtml(target.slug)}</span>
                        <span class="target-detail-item"><strong>Platform:</strong> ${escapeHtml(platformNameForBreadcrumb)} (ID: ${target.platform_id})</span>
                    </div>
                    <p><strong>Link:</strong> <a href="${escapeHtml(target.link)}" target="_blank">${escapeHtml(target.link)}</a></p>

                    <div class="tabs" style="margin-top: 20px;">
                        <button class="tab-button" data-tab="checklistTab">Checklist</button>
                        <button class="tab-button" data-tab="notesTab">Notes</button>
                        <button class="tab-button" data-tab="findingsTab">Findings</button>
                        <button class="tab-button" data-tab="scopeRulesTab">Scope Rules</button>
                    </div>

                    <div id="checklistTab" class="tab-content">
                        <h3>Target Checklist</h3>
                        <div id="targetChecklistContent">Loading checklist...</div>
                    </div>

                    <div id="notesTab" class="tab-content">
                        <h3>Target Notes</h3>
                        <div id="targetNotesSection" data-target-id="${target.id}" data-current-link="${escapeHtml(target.link)}">
                            <div class="notes-display-mode">
                                <p><strong>Notes:</strong> <button id="editTargetNotesBtn" class="action-button inline-edit-button" title="Edit Notes">✏️</button></p>
                                <div id="targetNotesContent" class="markdown-rendered-notes" data-raw-notes="${escapeHtmlAttribute(target.notes || '')}">
                                    ${(() => {
                                        // Ensure Showdown is loaded
                                        if (typeof showdown !== 'undefined') {
                                            const converter = new showdown.Converter({ tables: true, simpleLineBreaks: true, ghCompatibleHeaderId: true });
                                            return target.notes ? converter.makeHtml(target.notes) : '<p>(None)</p>';
                                        }
                                        return `<p>${escapeHtml(target.notes || '(None)')}</p>`; // Fallback if Showdown not loaded
                                    })()}
                                </div>
                            </div>
                            <div class="notes-edit-mode" style="display:none;">
                                <p><strong>Edit Notes:</strong></p>
                                <textarea id="targetNotesTextarea" rows="20" style="width: 100%; margin-bottom: 10px;"></textarea>
                                <button id="saveTargetNotesBtn" class="primary small-button">Save Notes</button>
                                <button id="cancelTargetNotesBtn" class="secondary small-button">Cancel</button>
                                <div id="saveTargetNotesMessage" class="message-area" style="margin-top: 5px;"></div>
                            </div>
                        </div>
                    </div>

                    <div id="findingsTab" class="tab-content">
                        <h3>Target Findings</h3>
                        <div id="targetFindingsContent">Loading findings...</div>
                    </div>

                    <div id="scopeRulesTab" class="tab-content">
                        <h3>Scope Rules</h3>
                        <button id="importScopeFromFileBtn" class="secondary small-button" style="margin-bottom: 10px; margin-right: 5px;">Import From File</button>
                        <input type="file" id="scopeFileImportInput" style="display: none;" accept=".json">
                        <div id="addScopeRuleMessage" class="message-area" style="margin-bottom: 15px;"></div>
                        <div class="scope-forms-container" style="display: flex; gap: 20px; margin-bottom:20px;">
                            <form id="addInScopeRuleForm" class="scope-rule-form" data-in-scope="true" style="flex:1; padding:15px; border:1px solid #ddd; border-radius:4px;">
                                <h4>Add In-Scope Item</h4>
                                <input type="hidden" name="target_id" value="${target.id}">
                                <div class="form-group"><label for="inScopePattern">Pattern:</label><input type="text" id="inScopePattern" name="pattern" placeholder="e.g., *.example.com" required></div>
                                <div class="form-group"><label for="inScopeItemType">Type:</label><select id="inScopeItemType" name="item_type">
                                    <option value="domain">Domain</option><option value="subdomain">Subdomain</option><option value="ip_address">IP Address</option><option value="cidr">CIDR</option><option value="url_path">URL Path</option>
                                </select></div>
                                <div class="form-group"><label for="inScopeDescription">Description:</label><input type="text" id="inScopeDescription" name="description" placeholder="Optional description"></div>
                                <button type="submit" class="primary">Add In-Scope</button>
                            </form>
                            <form id="addOutOfScopeRuleForm" class="scope-rule-form" data-in-scope="false" style="flex:1; padding:15px; border:1px solid #ddd; border-radius:4px;">
                                <h4>Add Out-of-Scope Item</h4>
                                <input type="hidden" name="target_id" value="${target.id}">
                                <div class="form-group"><label for="outOfScopePattern">Pattern:</label><input type="text" id="outOfScopePattern" name="pattern" placeholder="e.g., cdn.example.com" required></div>
                                <div class="form-group"><label for="outOfScopeItemType">Type:</label><select id="outOfScopeItemType" name="item_type">
                                    <option value="domain">Domain</option><option value="subdomain">Subdomain</option><option value="ip_address">IP Address</option><option value="cidr">CIDR</option><option value="url_path">URL Path</option>
                                </select></div>
                                <div class="form-group"><label for="outOfScopeDescription">Description:</label><input type="text" id="outOfScopeDescription" name="description" placeholder="Optional description"></div>
                                <button type="submit" class="secondary">Add Out-of-Scope</button>
                            </form>
                        </div>
                        <div id="current-target-scope"></div>
                    </div>
                `;
                // Add import button to the scope rules tab *within* the innerHTML
                const scopeTabContent = viewContentContainer.querySelector('#scopeRulesTab');
                if (scopeTabContent) {
                    const importButton = `<button id="importScopeFromClipboardBtn" class="secondary small-button" style="margin-bottom: 10px;">Import Scope from Clipboard</button>`;
                    scopeTabContent.insertAdjacentHTML('afterbegin', importButton);
                }
                renderScopeRulesTable(document.getElementById('current-target-scope'), target.scope_rules || []);

                const clearBtn = document.getElementById('clearCurrentTargetBtn');
                if(clearBtn) clearBtn.addEventListener('click', handleClearCurrentTarget);

                const setBtn = viewContentContainer.querySelector('.set-current-target');
                if(setBtn) setBtn.addEventListener('click', handleSetCurrentTargetFromDetails);

                document.getElementById('addInScopeRuleForm')?.addEventListener('submit', handleAddScopeRule);
                document.getElementById('addOutOfScopeRuleForm')?.addEventListener('submit', handleAddScopeRule);
                document.getElementById('importScopeFromFileBtn')?.addEventListener('click', () => document.getElementById('scopeFileImportInput').click());
                document.getElementById('scopeFileImportInput')?.addEventListener('change', (event) => handleScopeFileSelected(event, target.id));
                document.getElementById('importScopeFromClipboardBtn')?.addEventListener('click', () => importScopeRulesFromClipboard(target.id));
                
                
                
                document.getElementById('editTargetNotesBtn')?.addEventListener('click', handleEditTargetNotes);
                document.getElementById('saveTargetNotesBtn')?.addEventListener('click', handleSaveTargetNotes);
                document.getElementById('cancelTargetNotesBtn')?.addEventListener('click', cancelTargetNotesEdit);
                
                // Determine the actual tab ID to activate, normalizing the input
                let tabIdForActivation = 'checklistTab'; // Default tab
                const validTabHtmlIds = ['checklistTab', 'notesTab', 'findingsTab', 'scopeRulesTab'];

                if (tabToMakeActive && typeof tabToMakeActive === 'string' && tabToMakeActive.trim() !== '') {
                    let matchedHtmlId = null;
                    // First, check for an exact case-sensitive match
                    if (validTabHtmlIds.includes(tabToMakeActive)) {
                        matchedHtmlId = tabToMakeActive;
                    } else {
                        // If no exact match, try a case-insensitive match, also removing "Tab" suffix for flexibility
                        const normalizedInput = tabToMakeActive.toLowerCase().replace(/tab$/, '');
                        matchedHtmlId = validTabHtmlIds.find(validId =>
                            validId.toLowerCase().replace(/tab$/, '') === normalizedInput
                        );
                    }
                    if (matchedHtmlId) {
                        tabIdForActivation = matchedHtmlId;
                    }
                }
                setActiveTab(tabIdForActivation);
                document.querySelectorAll('.tabs .tab-button').forEach(button => button.addEventListener('click', handleTabSwitch));
                
                // After other content is loaded and tabs are set up:
                fetchAndDisplayChecklistItems(target.id);
                fetchAndDisplayTargetFindings(target.id); // New function call

                // Check for action to pre-fill "Add Finding" form
                const hashParams = new URLSearchParams(window.location.hash.split('?')[1] || '');
                const action = hashParams.get('action');
                const fromLogId = hashParams.get('from_log_id');

                if (action === 'addFinding' && fromLogId) {
                    // Ensure findings tab is active if we are adding a finding
                    // This might override tabToMakeActive if it was different, which is intended.
                    setActiveTab('findingsTab'); 
                    await displayAddFindingForm(target.id, { http_traffic_log_id: fromLogId });
                }

             } catch(error) {
                 console.error("Error fetching target details:", error);
                 if (viewContentContainer) viewContentContainer.innerHTML = `<h1>Target Details</h1><p class="error-message">Error loading details for target ID ${targetIdToLoad}: ${error.message}</p>`;
                 updateBreadcrumbs([{ name: "Target Details" }, { name: "Error" }]);
             }
        } else {
             if (viewContentContainer) {
                viewContentContainer.innerHTML = `<h1>Current Target</h1><p>No target is currently set, or no ID provided to view.</p><p>Use the 📍 button in the Targets list to set one, or select a target to view its details.</p>`;
             }
             updateBreadcrumbs([{ name: "Current Target" }]);
        }
    }

    function setActiveTab(tabIdToActivate) {
        document.querySelectorAll('.tabs .tab-button').forEach(btn => {
            btn.classList.toggle('active', btn.getAttribute('data-tab') === tabIdToActivate);
        });
        document.querySelectorAll('.tab-content').forEach(content => {
            content.classList.toggle('active', content.id === tabIdToActivate);
        });
    }

    function handleTabSwitch(event) {
        const button = event.target;
        const tabId = button.getAttribute('data-tab');
        if (tabId) setActiveTab(tabId);
    }

    async function handleClearCurrentTarget() {
        showModalConfirm("Clear Current Target", "Are you sure you want to clear the current target?", async () => {
            try {
                await apiService.setCurrentTargetSetting(null);
                updateState({ currentTargetId: null, currentTargetName: 'None' });
                if (currentTargetDisplay) {
                    currentTargetDisplay.textContent = `Target: None`;
                    currentTargetDisplay.title = `No current target set`;
                }
                showModalMessage('Success', 'Current target has been cleared.');
                const appState = getState();
                if (appState.currentView === 'current-target') loadCurrentTargetView();
            } catch (error) {
                showModalMessage("Error", `Failed to clear current target: ${error.message}`);
            }
        });
    }

    async function handleSetCurrentTargetFromDetails(event) {
        const button = event.target.closest('button');
        const targetId = button.getAttribute('data-id');
        const targetName = button.getAttribute('data-name');
        const targetIdNum = parseInt(targetId, 10);

        if (isNaN(targetIdNum)) {
            showModalMessage("Error", "Invalid target ID encountered.");
            return;
        }
        try {
            await apiService.setCurrentTargetSetting(targetIdNum);
            updateState({ currentTargetId: targetIdNum, currentTargetName: targetName });
            if (currentTargetDisplay) {
                currentTargetDisplay.textContent = `Target: ${escapeHtml(targetName)} (ID: ${targetIdNum})`;
                currentTargetDisplay.title = `Current Target: ${escapeHtml(targetName)} (ID: ${targetIdNum})`;
            }
            showModalMessage('Current Target Set', `Set current target to "${escapeHtml(targetName)}" (ID: ${targetIdNum}).`);
            const appState = getState();
            if (appState.currentView === 'current-target') loadCurrentTargetView(targetIdNum);
        } catch (error) {
            showModalMessage("Error", `Failed to set current target: ${error.message}`);
        }
    }

    function renderScopeRulesTable(containerElement, scopeRules) {
        if (!containerElement) return;
        if (!scopeRules || scopeRules.length === 0) {
            containerElement.innerHTML = "<p>No scope rules defined for this target.</p>";
            return;
        }
        let tableHTML = `
            <table><thead><tr><th>ID</th><th>Type</th><th>Scope</th><th>Pattern</th><th>Description</th><th>Actions</th></tr></thead><tbody>`;
        scopeRules.forEach(rule => {
            const scopeLabel = rule.is_in_scope ? 'IN' : 'OUT';
            const scopeClass = rule.is_in_scope ? 'scope-in' : 'scope-out';
            tableHTML += `<tr data-rule-id="${rule.id}"><td>${rule.id}</td><td>${escapeHtml(rule.item_type)}</td><td class="${scopeClass}">${scopeLabel}</td><td>${escapeHtml(rule.pattern)}</td><td>${escapeHtml(rule.description)}</td><td><button class="action-button delete-scope-rule" data-rule-id="${rule.id}" title="Delete Rule">🗑️</button></td></tr>`;
        });
        tableHTML += `</tbody></table>`;
        containerElement.innerHTML = tableHTML;
        document.querySelectorAll('.delete-scope-rule').forEach(button => button.addEventListener('click', handleDeleteScopeRule));
    }

    async function handleAddScopeRule(event) {
        event.preventDefault();
        const form = event.target;
        const targetId = form.querySelector('input[name="target_id"]').value;
        const pattern = form.querySelector('input[name="pattern"]').value.trim();
        const itemType = form.querySelector('select[name="item_type"]').value;
        const description = form.querySelector('input[name="description"]').value.trim();
        const isInScope = form.getAttribute('data-in-scope') === 'true';
        const messageArea = document.getElementById('addScopeRuleMessage');        

        if (messageArea) messageArea.textContent = '';
        if (messageArea) messageArea.className = 'message-area';
        if (!pattern) {
            messageArea.textContent = 'Pattern cannot be empty.';
            return;
        }
        const payload = { target_id: parseInt(targetId, 10), pattern, item_type: itemType, is_in_scope: isInScope, description };
        try {
            await apiService.addScopeRule(payload);
            messageArea.textContent = 'Scope rule added successfully!';
            messageArea.classList.add('success-message');
            form.reset();
            loadCurrentTargetView(targetId, 'scopeRulesTab'); // Keep scopeRulesTab active
        } catch (error) {
            messageArea.textContent = `Error adding scope rule: ${error.message}`;
            messageArea.classList.add('error-message');
        }
    }

    // Module-level variable to store the target notes keydown listener
    let targetNotesKeydownListener = null;

    function handleEditTargetNotes() {
        const notesSection = document.getElementById('targetNotesSection');
        const displayMode = notesSection.querySelector('.notes-display-mode');
        const editMode = notesSection.querySelector('.notes-edit-mode');
        const notesContentDiv = document.getElementById('targetNotesContent');
        const notesTextarea = document.getElementById('targetNotesTextarea');

        // Get raw Markdown from the data attribute
        const rawNotes = notesContentDiv.getAttribute('data-raw-notes');
        notesTextarea.value = rawNotes || '';

        displayMode.style.display = 'none';
        editMode.style.display = 'block';
        notesTextarea.focus();

        // Remove previous listener if any, then add new one
        if (targetNotesKeydownListener) {
            notesTextarea.removeEventListener('keydown', targetNotesKeydownListener);
        }

        targetNotesKeydownListener = function(event) {
            if (event.key === 'Escape') {
                event.preventDefault(); // Prevent any other escape behavior
                cancelTargetNotesEdit();
                return;
            }

            if (event.key === 'Enter') {
                if (event.ctrlKey || event.metaKey) { // Ctrl+Enter or Cmd+Enter to save
                    event.preventDefault(); // Prevent newline
                    document.getElementById('saveTargetNotesBtn').click(); // Trigger save
                } else if (event.shiftKey) {
                    // Shift+Enter: Allow default behavior (newline).
                } else {
                    // Just Enter: Allow default behavior (newline).
                    // event.stopPropagation(); // Uncomment if plain Enter still causes unexpected behavior
                }
            }
        };
        notesTextarea.addEventListener('keydown', targetNotesKeydownListener);
    }

    async function handleSaveTargetNotes() {
        const notesSection = document.getElementById('targetNotesSection');
        const targetId = notesSection.getAttribute('data-target-id');
        const currentLink = notesSection.getAttribute('data-current-link');
        const notesTextarea = document.getElementById('targetNotesTextarea');
        const newNotes = notesTextarea.value.trim();
        const messageArea = document.getElementById('saveTargetNotesMessage');
        messageArea.textContent = '';
        messageArea.className = 'message-area';
        const payload = { link: currentLink, notes: newNotes };
        try {
            const updatedTarget = await apiService.updateTarget(targetId, payload);
            const notesDisplayDiv = document.getElementById('targetNotesContent');
            const rawNotes = updatedTarget.notes || '';
            if (typeof showdown !== 'undefined') {
                const converter = new showdown.Converter({ tables: true, simpleLineBreaks: true, ghCompatibleHeaderId: true });
                notesDisplayDiv.innerHTML = rawNotes ? converter.makeHtml(rawNotes) : '<p>(None)</p>';
            } else {
                notesDisplayDiv.innerHTML = `<p>${escapeHtml(rawNotes || '(None)')}</p>`; // Fallback
            }
            notesDisplayDiv.setAttribute('data-raw-notes', escapeHtmlAttribute(rawNotes));

            notesSection.setAttribute('data-current-link', escapeHtml(updatedTarget.link));
            messageArea.textContent = 'Notes updated successfully!';
            messageArea.classList.add('success-message');
            cancelTargetNotesEdit();
        } catch (error) {
            messageArea.textContent = `Error updating notes: ${error.message}`;
            messageArea.classList.add('error-message');
        }
    }

    function cancelTargetNotesEdit() {
        const notesSection = document.getElementById('targetNotesSection');
        const displayMode = notesSection.querySelector('.notes-display-mode');
        const editMode = notesSection.querySelector('.notes-edit-mode');
        const messageArea = document.getElementById('saveTargetNotesMessage');
        const notesTextarea = document.getElementById('targetNotesTextarea');

        if (messageArea) {
            messageArea.textContent = '';
            messageArea.className = 'message-area'; // Reset classes
        }
        displayMode.style.display = 'block';
        editMode.style.display = 'none';

        if (notesTextarea && targetNotesKeydownListener) {
            notesTextarea.removeEventListener('keydown', targetNotesKeydownListener);
            targetNotesKeydownListener = null;
        }
    }

    async function handleDeleteScopeRule(event) {
        const button = event.target.closest('button');
        const ruleId = button.getAttribute('data-rule-id');
        const appState = getState();
        const targetId = appState.currentTargetId;
        showModalConfirm('Confirm Delete Scope Rule', `Are you sure you want to delete scope rule ID ${ruleId}?`, async () => {
            try {
                await apiService.deleteScopeRule(ruleId);
                showModalMessage('Success', `Scope rule ID ${ruleId} deleted successfully.`);
                if (targetId) loadCurrentTargetView(targetId.toString(), 'scopeRulesTab'); // Keep scopeRulesTab active
            } catch (error) {
                showModalMessage('Error', `Error deleting scope rule: ${error.message}`);
            }
        });
    }

    async function handleScopeFileSelected(event, targetId) {
        const file = event.target.files[0];
        if (!file) {
            showModalMessage("File Error", "No file selected.");
            return;
        }
        if (file.type !== "application/json") {
            showModalMessage("File Error", "Please select a valid JSON file (.json).");
            return;
        }

        // We'll need a function in targetView.js to handle the file reading and API call
        // For now, let's assume it exists and is named importScopeRulesFromFile
        if (typeof importScopeRulesFromFile === 'function') { // Check if the function exists
            importScopeRulesFromFile(targetId, file);
        } else {
            showModalMessage("Error", "File import functionality is not fully implemented yet (targetView.js).");
        }
        event.target.value = null; // Reset file input
    }

    function cleanupEasyMDEInstances() {
        for (const key in easyMDEInstances) {
            if (easyMDEInstances[key] && typeof easyMDEInstances[key].toTextArea === 'function') {
                try {
                    easyMDEInstances[key].toTextArea(); // This removes the editor and restores the original textarea
                } catch (e) {
                    console.warn("Error cleaning up EasyMDE instance for", key, e);
                }
            }
        }
        easyMDEInstances = {}; // Reset the store
    }

    function initializeEasyMDE(elementId, initialValue = '', customOptions = {}) {
        if (typeof EasyMDE === 'undefined') {
            console.warn("EasyMDE library not loaded. Markdown editor will not be available.");
            return null;
        }
        const element = document.getElementById(elementId);
        if (!element) return null;

        const editor = new EasyMDE({
            element: element,
            initialValue: initialValue,
            spellChecker: false,
            minHeight: '100px',
            maxHeight: '250px',
            status: ["lines", "words"],
            toolbar: ["bold", "italic", "heading", "|", "quote", "unordered-list", "ordered-list", "|", "link", "image", "|", "preview", "side-by-side", "fullscreen", "|", "guide"],
            ...customOptions
        });
        easyMDEInstances[elementId] = editor;
        return editor;
    }

    async function fetchAndDisplayTargetFindings(targetId) {
        const findingsContentDiv = document.getElementById('targetFindingsContent');
        if (!findingsContentDiv) {
            console.error("targetFindingsContent div not found!");
            return;
        }
        findingsContentDiv.innerHTML = '<p>Loading findings...</p>';
        cleanupEasyMDEInstances(); // Clean up before re-rendering

        try {
            const findings = await apiService.getTargetFindings(targetId);
            if (findings && findings.length > 0) {
                let findingsHTML = `
                    <button id="addNewFindingBtn" class="primary small-button" style="margin-bottom:15px;">Add New Finding</button>
                    <table>
                        <thead>
                            <tr>
                                <th>ID</th>
                                <th>Title</th>
                                <th>Severity</th>
                                <th>Status</th>
                                <th>Discovered</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>`;
                findings.forEach(finding => {
                    findingsHTML += `
                        <tr data-finding-id="${finding.id}">
                            <td>${finding.id}</td>
                            <td>${escapeHtml(finding.title)}</td>
                            <td>${escapeHtml(finding.severity.String || 'N/A')}</td>
                            <td>${escapeHtml(finding.status)}</td>
                            <td>${new Date(finding.discovered_at || Date.now()).toLocaleDateString()}</td>
                            <td><button class="action-button view-finding-detail" data-finding-id="${finding.id}" title="View Finding">👁️</button> <button class="action-button edit-finding" data-finding-id="${finding.id}" data-target-id="${targetId}" title="Edit Finding">✏️</button> <button class="action-button delete-finding" data-finding-id="${finding.id}" data-target-id="${targetId}" title="Delete Finding">🗑️</button></td>
                        </tr>`;
                });
                findingsHTML += `</tbody></table>`;
                findingsContentDiv.innerHTML = findingsHTML;
            } else {
                findingsContentDiv.innerHTML = '<p>No findings recorded for this target yet.</p> <button id="addNewFindingBtn" class="primary small-button">Add New Finding</button>';
            }
            const addNewFindingBtn = document.getElementById('addNewFindingBtn');
            if (addNewFindingBtn) {
                addNewFindingBtn.addEventListener('click', () => displayAddFindingForm(targetId));
            }
            findingsContentDiv.querySelectorAll('.view-finding-detail').forEach(btn => btn.addEventListener('click', (e) => handleViewFindingDetail(e.currentTarget.dataset.findingId)));
            findingsContentDiv.querySelectorAll('.edit-finding').forEach(btn => btn.addEventListener('click', (e) => displayEditFindingForm(e.currentTarget.dataset.findingId, e.currentTarget.dataset.targetId)));
            findingsContentDiv.querySelectorAll('.delete-finding').forEach(btn => btn.addEventListener('click', (e) => handleDeleteFinding(e.currentTarget.dataset.findingId, e.currentTarget.dataset.targetId)));
        } catch (fetchFindingsError) {
            console.error("Error fetching target findings:", fetchFindingsError);
            if (findingsContentDiv) {
                findingsContentDiv.innerHTML = `<p class="error-message">Error loading findings: ${escapeHtml(fetchFindingsError.message)}</p>`;
            }
        }
    }

    function applyUiSettings(uiSettings) {
        const showSynack = uiSettings && uiSettings.ShowSynackSection === true;
        const synackSection = document.getElementById('synack-sidebar-section');
        if (synackSection) {
            synackSection.classList.toggle('hidden', !showSynack);
        } else {
            console.warn('[App.js] synack-sidebar-section not found');
        }
    }

    async function displayAddFindingForm(targetId, prefillData = null) {
        const findingsContentDiv = document.getElementById('targetFindingsContent');
        if (!findingsContentDiv) {
            showModalMessage("Error", "UI element missing, cannot display form.");
            return;
        }
        cleanupEasyMDEInstances(); // Clean up before rendering new form

        let initialTitle = '';
        let initialDescription = '';
        let initialPayload = '';
        let initialHttpLogId = prefillData?.http_traffic_log_id || '';
        let initialSeverity = 'Medium';
        let initialStatus = 'Open';

        if (prefillData && prefillData.http_traffic_log_id) {
            showModalMessage("Loading...", "Fetching log details for pre-fill...", true, 1500);
            try {
                const logEntry = await apiService.getProxyLogDetail(prefillData.http_traffic_log_id);
                let pathForTitle = 'N/A';
                try { pathForTitle = new URL(logEntry.request_url?.String).pathname; } catch (e) { pathForTitle = logEntry.request_url?.String || 'N/A'; }
                initialTitle = `Finding from Log ${logEntry.id}: ${logEntry.request_method?.String || 'N/A'} ${pathForTitle}`;
                
                let reqHeadersObj = {};
                if (logEntry.request_headers && logEntry.request_headers.Valid && logEntry.request_headers.String) {
                    try { reqHeadersObj = JSON.parse(logEntry.request_headers.String); } catch (e) { /* ignore */ }
                }
                let requestContentTypeForBody = '';
                for (const key in reqHeadersObj) {
                    if (key.toLowerCase() === 'content-type') {
                        requestContentTypeForBody = Array.isArray(reqHeadersObj[key]) ? reqHeadersObj[key][0] : reqHeadersObj[key];
                        break;
                    }
                }
                const formatBodyForDesc = (base64Body, contentType) => {
                    if (!base64Body) return '';
                    try { const text = atob(base64Body); return text.length > 500 ? text.substring(0, 500) + '...' : text; } catch (e) { return ''; }
                };
                let resHeadersObj = {};
                if (logEntry.response_headers && logEntry.response_headers.Valid && logEntry.response_headers.String) {
                    try { resHeadersObj = JSON.parse(logEntry.response_headers.String); } catch (e) { /* ignore */ }
                }
                initialDescription = `Source Log ID: ${logEntry.id}\nURL: ${logEntry.request_url?.String || 'N/A'}\nMethod: ${logEntry.request_method?.String || 'N/A'}\nStatus: ${logEntry.response_status_code || 'N/A'}\n\n--- Request ---\nHeaders:\n${localFormatHeadersForFinding(reqHeadersObj)}\nBody:\n${formatBodyForDesc(logEntry.request_body, requestContentTypeForBody)}\n\n--- Response ---\nHeaders:\n${localFormatHeadersForFinding(resHeadersObj)}\nBody:\n${formatBodyForDesc(logEntry.response_body, logEntry.response_content_type?.String)}`;
                if (requestContentTypeForBody && (requestContentTypeForBody.includes('json') || requestContentTypeForBody.includes('xml') || requestContentTypeForBody.includes('text') || requestContentTypeForBody.includes('form'))) {
                    try { initialPayload = atob(logEntry.request_body); } catch(e) { initialPayload = '(Error decoding request body)';}
                } else if (logEntry.request_body) { initialPayload = '(Request body is potentially binary or has no Content-Type)'; }

            } catch (err) {
                console.error("Error fetching log details for prefill:", err);
                showModalMessage("Error", "Could not fetch log details for pre-filling finding.");
            }
        }
        
        let vulnerabilityTypesOptions = '<option value="">-- Select Type --</option>';
        try {
            const vulnTypes = await apiService.getAllVulnerabilityTypes();
            vulnTypes.forEach(vt => vulnerabilityTypesOptions += `<option value="${vt.id}">${escapeHtml(vt.name)}</option>`);
        } catch (err) {
            console.error("Error fetching vulnerability types:", err);
            vulnerabilityTypesOptions = '<option value="">Error loading types</option>';
        }

        findingsContentDiv.innerHTML = `
            <h3>Add New Finding</h3>
            <form id="addFindingForm">
                <input type="hidden" name="target_id" value="${targetId}">
                <div class="form-group">
                    <label for="findingTitle">Title:</label>
                    <input type="text" id="findingTitle" name="title" value="${escapeHtmlAttribute(initialTitle)}" required>
                </div>
                <div class="form-group">
                    <label for="findingVulnerabilityType">Vulnerability Type:</label>
                    <select id="findingVulnerabilityType" name="vulnerability_type_id">
                        ${vulnerabilityTypesOptions}
                    </select>
                </div>
                <div class="form-group">
                    <label for="findingSummary">Summary (Markdown supported):</label>
                    <textarea id="findingSummary" name="summary" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label for="findingDescription">Description (Markdown supported):</label>
                    <textarea id="findingDescription" name="description" rows="5">${escapeHtml(initialDescription)}</textarea>
                </div>
                <div class="form-group">
                    <label for="findingStepsToReproduce">Steps to Reproduce (Markdown supported):</label>
                    <textarea id="findingStepsToReproduce" name="steps_to_reproduce" rows="5"></textarea>
                </div>
                <div class="form-group">
                    <label for="findingPayload">Payload:</label>
                    <textarea id="findingPayload" name="payload" rows="3">${escapeHtml(initialPayload)}</textarea>
                </div>
                <div class="form-group">
                    <label for="findingImpact">Impact:</label>
                    <textarea id="findingImpact" name="impact" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label for="findingSeverity">Severity:</label>
                    <select id="findingSeverity" name="severity">
                        <option value="Informational" ${initialSeverity === 'Informational' ? 'selected' : ''}>Informational</option>
                        <option value="Low" ${initialSeverity === 'Low' ? 'selected' : ''}>Low</option>
                        <option value="Medium" ${initialSeverity === 'Medium' ? 'selected' : ''}>Medium</option>
                        <option value="High" ${initialSeverity === 'High' ? 'selected' : ''}>High</option>
                        <option value="Critical" ${initialSeverity === 'Critical' ? 'selected' : ''}>Critical</option>
                    </select>
                </div>
                <div class="form-group">
                    <label for="findingStatus">Status:</label>
                    <select id="findingStatus" name="status">
                        <option value="Open" ${initialStatus === 'Open' ? 'selected' : ''}>Open</option>
                        <option value="Closed" ${initialStatus === 'Closed' ? 'selected' : ''}>Closed</option>
                        <option value="Remediated" ${initialStatus === 'Remediated' ? 'selected' : ''}>Remediated</option>
                        <option value="Accepted Risk" ${initialStatus === 'Accepted Risk' ? 'selected' : ''}>Accepted Risk</option>
                    </select>
                </div>
                <div class="form-group">
                    <label for="findingCvssScore">CVSS Score (e.g., 7.5):</label>
                    <input type="number" id="findingCvssScore" name="cvss_score" step="0.1" min="0" max="10">
                </div>
                <div class="form-group">
                    <label for="findingCweId">CWE ID (e.g., 79):</label>
                    <input type="number" id="findingCweId" name="cwe_id">
                </div>
                <div class="form-group">
                    <label for="findingReferences">References (URLs, one per line, or JSON):</label>
                    <textarea id="findingReferences" name="finding_references" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label for="findingRecommendations">Recommendations:</label>
                    <textarea id="findingRecommendations" name="recommendations" rows="3"></textarea>
                </div>
                <div class="form-group">
                    <label for="findingHttpLogId">Associated HTTP Log ID (Optional):</label>
                    <input type="number" id="findingHttpLogId" name="http_traffic_log_id" value="${initialHttpLogId}">
                </div>
                <div class="form-actions">
                    <button type="submit" class="primary">Save Finding</button>
                    <button type="button" id="cancelAddFindingBtn" class="secondary">Cancel</button>
                </div>
            </form>
            <div id="addFindingMessage" class="message-area" style="margin-top:10px;"></div>
        `;

        document.getElementById('addFindingForm').addEventListener('submit', (event) => handleSaveNewFinding(event, targetId));
        document.getElementById('cancelAddFindingBtn').addEventListener('click', () => fetchAndDisplayTargetFindings(targetId));

        // Initialize EasyMDE for markdown fields
        initializeEasyMDE('findingSummary', '');
        initializeEasyMDE('findingDescription', initialDescription); // Use pre-filled value
        initializeEasyMDE('findingStepsToReproduce', '');
        initializeEasyMDE('findingImpact', '');
        initializeEasyMDE('findingRecommendations', '');
    }

    async function handleSaveNewFinding(event, targetId) {
        event.preventDefault();
        const form = event.target;
        const messageArea = document.getElementById('addFindingMessage');
        messageArea.textContent = '';
        messageArea.className = 'message-area';

        const getEditorValueOrForm = (editorId, formKey) => {
            if (easyMDEInstances[editorId] && typeof easyMDEInstances[editorId].value === 'function') {
                return easyMDEInstances[editorId].value();
            }
            return formData.get(formKey);
        };

        const formData = new FormData(form);
        const formatNullableString = (value) => {
            const trimmedValue = String(value || '').trim();
            return trimmedValue ? { String: trimmedValue, Valid: true } : null;
        };
        const formatNullableNumber = (value, isFloat = false) => {
            const strValue = String(value || '').trim();
            if (strValue === '') return null;
            const numValue = isFloat ? parseFloat(strValue) : parseInt(strValue, 10);
            return isNaN(numValue) ? null : (isFloat ? { Float64: numValue, Valid: true } : { Int64: numValue, Valid: true });
        };

        const findingData = {
            target_id: parseInt(formData.get('target_id'), 10),
            title: formData.get('title').trim(),
            summary: formatNullableString(getEditorValueOrForm('findingSummary', 'summary')),
            description: formatNullableString(getEditorValueOrForm('findingDescription', 'description')),
            steps_to_reproduce: formatNullableString(getEditorValueOrForm('findingStepsToReproduce', 'steps_to_reproduce')),
            impact: formatNullableString(getEditorValueOrForm('findingImpact', 'impact')),
            recommendations: formatNullableString(getEditorValueOrForm('findingRecommendations', 'recommendations')),
            payload: formatNullableString(formData.get('payload')),
            severity: formatNullableString(formData.get('severity')),
            status: formData.get('status'),
            cvss_score: formatNullableNumber(formData.get('cvss_score'), true),
            cwe_id: formatNullableNumber(formData.get('cwe_id')),
            finding_references: formatNullableString(formData.get('finding_references')),
            http_traffic_log_id: formatNullableNumber(formData.get('http_traffic_log_id')),
            vulnerability_type_id: formatNullableNumber(formData.get('vulnerability_type_id')),
        };

        if (!findingData.title) {
            messageArea.textContent = "Title is required.";
            messageArea.classList.add('error-message');
            return;
        }

        try {
            await apiService.createTargetFinding(findingData);
            showModalMessage("Success", "New finding added successfully!");
            fetchAndDisplayTargetFindings(targetId);
        } catch (error) {
            messageArea.textContent = `Error saving finding: ${escapeHtml(error.message)}`;
            messageArea.classList.add('error-message');
            console.error("Error saving finding:", error);
        }
    }

    async function displayEditFindingForm(findingId, targetId) {
        const findingsContentDiv = document.getElementById('targetFindingsContent');
        if (!findingsContentDiv) {
            showModalMessage("Error", "UI element missing, cannot display form.");
            return;
        }
        cleanupEasyMDEInstances(); // Clean up before rendering new form

        try {
            // Fetch the finding details
            // This is done here to ensure the form is populated with the latest data
            // before rendering.
            // This also ensures that if the user switches from view to edit, they get the latest.
            const finding = await apiService.getFindingDetails(findingId);
            let vulnerabilityTypesOptions = '<option value="">-- Select Type --</option>';
            try {
                const vulnTypes = await apiService.getAllVulnerabilityTypes();
                vulnTypes.forEach(vt => vulnerabilityTypesOptions += `<option value="${vt.id}" ${finding.vulnerability_type_id && finding.vulnerability_type_id.Int64 === vt.id ? 'selected' : ''}>${escapeHtml(vt.name)}</option>`);
            } catch (err) {
                console.error("Error fetching vulnerability types for edit form:", err);
                vulnerabilityTypesOptions = '<option value="">Error loading types</option>';
            }

            findingsContentDiv.innerHTML = `
                <h3>Edit Finding (ID: ${finding.id})</h3>
                <div class="form-actions" style="margin-bottom: 15px;">
                    <button type="submit" form="editFindingForm" class="primary">Save Changes</button>
                    <button type="button" id="cancelEditFindingBtnTop" class="secondary">Cancel</button>
                </div>

                <form id="editFindingForm">
                    <input type="hidden" name="target_id" value="${targetId}">
                    <input type="hidden" name="finding_id" value="${finding.id}">
                    <div class="form-group">
                        <label for="findingTitleEdit">Title:</label>
                        <input type="text" id="findingTitleEdit" name="title" value="${escapeHtmlAttribute(finding.title)}" required>
                    </div>
                    <div class="form-group">
                        <label for="findingVulnerabilityTypeEdit">Vulnerability Type:</label>
                        <select id="findingVulnerabilityTypeEdit" name="vulnerability_type_id">
                            ${vulnerabilityTypesOptions}
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingSummaryEdit">Summary:</label>
                        <textarea id="findingSummaryEdit" name="summary" rows="3">${escapeHtml(finding.summary.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingDescriptionEdit">Description:</label>
                        <textarea id="findingDescriptionEdit" name="description" rows="5">${escapeHtml(finding.description.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingStepsToReproduceEdit">Steps to Reproduce:</label>
                        <textarea id="findingStepsToReproduceEdit" name="steps_to_reproduce" rows="5">${escapeHtml(finding.steps_to_reproduce.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingPayloadEdit">Payload:</label>
                        <textarea id="findingPayloadEdit" name="payload" rows="3">${escapeHtml(finding.payload.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingImpactEdit">Impact:</label>
                        <textarea id="findingImpactEdit" name="impact" rows="3">${escapeHtml(finding.impact.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingSeverityEdit">Severity:</label>
                        <select id="findingSeverityEdit" name="severity">
                            <option value="Informational" ${finding.severity.String === 'Informational' ? 'selected' : ''}>Informational</option>
                            <option value="Low" ${finding.severity.String === 'Low' ? 'selected' : ''}>Low</option>
                            <option value="Medium" ${finding.severity.String === 'Medium' ? 'selected' : ''}>Medium</option>
                            <option value="High" ${finding.severity.String === 'High' ? 'selected' : ''}>High</option>
                            <option value="Critical" ${finding.severity.String === 'Critical' ? 'selected' : ''}>Critical</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingStatusEdit">Status:</label>
                        <select id="findingStatusEdit" name="status">
                            <option value="Open" ${finding.status === 'Open' ? 'selected' : ''}>Open</option>
                            <option value="Closed" ${finding.status === 'Closed' ? 'selected' : ''}>Closed</option>
                            <option value="Remediated" ${finding.status === 'Remediated' ? 'selected' : ''}>Remediated</option>
                            <option value="Accepted Risk" ${finding.status === 'Accepted Risk' ? 'selected' : ''}>Accepted Risk</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingCvssScoreEdit">CVSS Score:</label>
                        <input type="number" id="findingCvssScoreEdit" name="cvss_score" step="0.1" min="0" max="10" value="${finding.cvss_score.Valid ? finding.cvss_score.Float64 : ''}">
                    </div>
                    <div class="form-group">
                        <label for="findingCweIdEdit">CWE ID:</label>
                        <input type="number" id="findingCweIdEdit" name="cwe_id" value="${finding.cwe_id.Valid ? finding.cwe_id.Int64 : ''}">
                    </div>
                    <div class="form-group">
                        <label for="findingReferencesEdit">References:</label>
                        <textarea id="findingReferencesEdit" name="finding_references" rows="3">${escapeHtml(finding.finding_references.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingRecommendationsEdit">Recommendations:</label>
                        <textarea id="findingRecommendationsEdit" name="recommendations" rows="3">${escapeHtml(finding.recommendations.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingHttpLogIdEdit">Associated HTTP Log ID:</label>
                        <input type="number" id="findingHttpLogIdEdit" name="http_traffic_log_id" value="${finding.http_traffic_log_id.Valid ? finding.http_traffic_log_id.Int64 : ''}">
                    </div>
                    <div class="form-actions">
                        <button type="submit" class="primary">Save Changes</button>
                        <button type="button" id="cancelEditFindingBtn" class="secondary">Cancel</button>
                    </div>
                </form>
                <div id="editFindingMessage" class="message-area" style="margin-top:10px;"></div>
            `;
            document.getElementById('editFindingForm').addEventListener('submit', (event) => handleSaveUpdatedFinding(event, findingId, targetId));
            document.getElementById('cancelEditFindingBtnTop')?.addEventListener('click', () => displayViewFindingFormReadOnly(findingId));
            document.getElementById('cancelEditFindingBtn').addEventListener('click', () => displayViewFindingFormReadOnly(findingId));

            // Initialize EasyMDE for markdown fields in edit form
            initializeEasyMDE('findingSummaryEdit', finding.summary.String || '');
            initializeEasyMDE('findingDescriptionEdit', finding.description.String || '');
            initializeEasyMDE('findingStepsToReproduceEdit', finding.steps_to_reproduce.String || '');
            initializeEasyMDE('findingImpactEdit', finding.impact.String || '');
            initializeEasyMDE('findingRecommendationsEdit', finding.recommendations.String || '');
        } catch (error) {
            console.error("Error fetching finding for edit:", error);
            findingsContentDiv.innerHTML = `<p class="error-message">Error loading finding for editing: ${error.message}</p>`;
        }
    }

    async function handleViewFindingDetail(findingId) {
        await displayViewFindingFormReadOnly(findingId);
    }

    async function displayViewFindingFormReadOnly(findingId) {
        const findingsContentDiv = document.getElementById('targetFindingsContent');
        if (!findingsContentDiv) {
            showModalMessage("Error", "UI element missing, cannot display form.");
            return;
        }
        cleanupEasyMDEInstances(); // Clean up before rendering new form

        try {
            const finding = await apiService.getFindingDetails(findingId);
            let vulnerabilityTypesOptions = '<option value="">-- Select Type --</option>';
            try {
                const vulnTypes = await apiService.getAllVulnerabilityTypes();
                vulnTypes.forEach(vt => vulnerabilityTypesOptions += `<option value="${vt.id}" ${finding.vulnerability_type_id && finding.vulnerability_type_id.Int64 === vt.id ? 'selected' : ''}>${escapeHtml(vt.name)}</option>`);
            } catch (err) {
                console.error("Error fetching vulnerability types for view form:", err);
                vulnerabilityTypesOptions = '<option value="">Error loading types</option>';
            }

            findingsContentDiv.innerHTML = `
                <h3>View Finding (ID: ${finding.id}) <button id="switchToEditFindingBtn" class="action-button inline-edit-button" data-finding-id="${finding.id}" data-target-id="${finding.target_id}" title="Edit Finding">✏️</button></h3>
                <form id="viewFindingForm">
                    <input type="hidden" name="finding_id" value="${finding.id}">
                    <div class="form-group">
                        <label for="findingTitleView">Title:</label>
                        <input type="text" id="findingTitleView" name="title" value="${escapeHtmlAttribute(finding.title)}" disabled>
                    </div>
                    <div class="form-group">
                        <label for="findingVulnerabilityTypeView">Vulnerability Type:</label>
                        <select id="findingVulnerabilityTypeView" name="vulnerability_type_id" disabled>
                            ${vulnerabilityTypesOptions}
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingSummaryView">Summary:</label>
                        <textarea id="findingSummaryView" name="summary" rows="3" disabled>${escapeHtml(finding.summary.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingDescriptionView">Description:</label>
                        <textarea id="findingDescriptionView" name="description" rows="5" disabled>${escapeHtml(finding.description.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingStepsToReproduceView">Steps to Reproduce:</label>
                        <textarea id="findingStepsToReproduceView" name="steps_to_reproduce" rows="5" disabled>${escapeHtml(finding.steps_to_reproduce.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingPayloadView">Payload:</label>
                        <textarea id="findingPayloadView" name="payload" rows="3" disabled>${escapeHtml(finding.payload.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingImpactView">Impact:</label>
                        <textarea id="findingImpactView" name="impact" rows="3" disabled>${escapeHtml(finding.impact.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingSeverityView">Severity:</label>
                        <select id="findingSeverityView" name="severity" disabled>
                            <option value="Informational" ${finding.severity.String === 'Informational' ? 'selected' : ''}>Informational</option>
                            <option value="Low" ${finding.severity.String === 'Low' ? 'selected' : ''}>Low</option>
                            <option value="Medium" ${finding.severity.String === 'Medium' ? 'selected' : ''}>Medium</option>
                            <option value="High" ${finding.severity.String === 'High' ? 'selected' : ''}>High</option>
                            <option value="Critical" ${finding.severity.String === 'Critical' ? 'selected' : ''}>Critical</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingStatusView">Status:</label>
                        <select id="findingStatusView" name="status" disabled>
                            <option value="Open" ${finding.status === 'Open' ? 'selected' : ''}>Open</option>
                            <option value="Closed" ${finding.status === 'Closed' ? 'selected' : ''}>Closed</option>
                            <option value="Remediated" ${finding.status === 'Remediated' ? 'selected' : ''}>Remediated</option>
                            <option value="Accepted Risk" ${finding.status === 'Accepted Risk' ? 'selected' : ''}>Accepted Risk</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="findingCvssScoreView">CVSS Score:</label>
                        <input type="number" id="findingCvssScoreView" name="cvss_score" step="0.1" min="0" max="10" value="${finding.cvss_score.Valid ? finding.cvss_score.Float64 : ''}" disabled>
                    </div>
                    <div class="form-group">
                        <label for="findingCweIdView">CWE ID:</label>
                        <input type="number" id="findingCweIdView" name="cwe_id" value="${finding.cwe_id.Valid ? finding.cwe_id.Int64 : ''}" disabled>
                    </div>
                    <div class="form-group">
                        <label for="findingReferencesView">References:</label>
                        <textarea id="findingReferencesView" name="finding_references" rows="3" disabled>${escapeHtml(finding.finding_references.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingRecommendationsView">Recommendations:</label>
                        <textarea id="findingRecommendationsView" name="recommendations" rows="3" disabled>${escapeHtml(finding.recommendations.String || '')}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="findingHttpLogIdView">Associated HTTP Log ID:</label>
                        <input type="number" id="findingHttpLogIdView" name="http_traffic_log_id" value="${finding.http_traffic_log_id.Valid ? finding.http_traffic_log_id.Int64 : ''}" disabled>
                    </div>
                    <div class="form-actions">
                        <button type="button" id="backToFindingsListBtn" class="secondary">Back to List</button>
                        <button type="button" id="switchToEditFindingBtnBottom" class="action-button" data-finding-id="${finding.id}" data-target-id="${finding.target_id}" title="Edit Finding" style="margin-left: 10px;">✏️</button>
                    </div>
                </form>
            `;
            document.getElementById('backToFindingsListBtn')?.addEventListener('click', () => fetchAndDisplayTargetFindings(finding.target_id));

            const switchToEdit = (e) => {
                const button = e.currentTarget;
                const findingIdToEdit = button.dataset.findingId;
                const targetIdForEdit = button.dataset.targetId;
                displayEditFindingForm(findingIdToEdit, targetIdForEdit);
            };

            document.getElementById('switchToEditFindingBtn')?.addEventListener('click', switchToEdit);
            document.getElementById('switchToEditFindingBtnBottom')?.addEventListener('click', switchToEdit);

            // Initialize EasyMDE for markdown fields in a read-only state
            const mdeFields = [
                { id: 'findingSummaryView', value: finding.summary.String || '' },
                { id: 'findingDescriptionView', value: finding.description.String || '' },
                { id: 'findingStepsToReproduceView', value: finding.steps_to_reproduce.String || '' },
                { id: 'findingImpactView', value: finding.impact.String || '' },
                { id: 'findingRecommendationsView', value: finding.recommendations.String || '' }
            ];

            mdeFields.forEach(field => {
                const editor = initializeEasyMDE(field.id, field.value, { toolbar: false, status: false });
                if (editor) {
                    editor.codemirror.setOption("readOnly", "nocursor");
                }
            });
        } catch (error) {
            console.error("Error fetching finding for view:", error);
            findingsContentDiv.innerHTML = `<p class="error-message">Error loading finding for viewing: ${error.message}</p>`;
        }
    }
    async function handleSaveUpdatedFinding(event, findingId, targetId) {
        event.preventDefault();
        const form = event.target;
        const messageArea = document.getElementById('editFindingMessage');
        messageArea.textContent = '';
        messageArea.className = 'message-area';

        const getEditorValueOrForm = (editorId, formKey) => {
            if (easyMDEInstances[editorId] && typeof easyMDEInstances[editorId].value === 'function') {
                return easyMDEInstances[editorId].value();
            }
            return formData.get(formKey);
        };

        const formData = new FormData(form);
        const formatNullableString = (value) => (String(value || '').trim() ? { String: String(value).trim(), Valid: true } : null);
        const formatNullableNumber = (value, isFloat = false) => {
            const strValue = String(value || '').trim();
            if (strValue === '') return null;
            const numValue = isFloat ? parseFloat(strValue) : parseInt(strValue, 10);
            return isNaN(numValue) ? null : (isFloat ? { Float64: numValue, Valid: true } : { Int64: numValue, Valid: true });
        };

        const findingData = {
            title: formData.get('title').trim(),
            summary: formatNullableString(getEditorValueOrForm('findingSummaryEdit', 'summary')),
            description: formatNullableString(getEditorValueOrForm('findingDescriptionEdit', 'description')),
            steps_to_reproduce: formatNullableString(getEditorValueOrForm('findingStepsToReproduceEdit', 'steps_to_reproduce')),
            impact: formatNullableString(getEditorValueOrForm('findingImpactEdit', 'impact')),
            recommendations: formatNullableString(getEditorValueOrForm('findingRecommendationsEdit', 'recommendations')),
            payload: formatNullableString(formData.get('payload')),
            severity: formatNullableString(formData.get('severity')),
            status: formData.get('status'),
            cvss_score: formatNullableNumber(formData.get('cvss_score'), true),
            cwe_id: formatNullableNumber(formData.get('cwe_id')),
            finding_references: formatNullableString(formData.get('finding_references')),
            http_traffic_log_id: formatNullableNumber(formData.get('http_traffic_log_id')),
            vulnerability_type_id: formatNullableNumber(formData.get('vulnerability_type_id')),
        };

        if (!findingData.title) {
            messageArea.textContent = "Title is required.";
            messageArea.classList.add('error-message');
            return;
        }

        try {
            await apiService.updateTargetFinding(findingId, findingData);
            showModalMessage("Success", "Finding updated successfully.", true, 1500);
            await displayViewFindingFormReadOnly(findingId); // Go back to read-only view
        } catch (error) {
            messageArea.textContent = `Error updating finding: ${escapeHtml(error.message)}`;
            messageArea.classList.add('error-message');
            console.error("Error updating finding:", error);
        }
    }

    async function handleDeleteFinding(findingId, targetId) {
        showModalConfirm("Confirm Delete", `Are you sure you want to delete finding ID ${findingId}? This action cannot be undone.`, async () => {
            try {
                await apiService.deleteTargetFinding(findingId);
                showModalMessage("Success", `Finding ID ${findingId} deleted successfully.`);
                fetchAndDisplayTargetFindings(targetId);
            } catch (error) {
                console.error("Error deleting finding:", error);
                showModalMessage("Error", `Failed to delete finding ID ${findingId}: ${error.message}`);
            }
        });
    }

    async function fetchAndSetInitialCurrentTarget() {
        const currentTargetDisplay = document.getElementById('currentPlatformTarget');
        let fetchedCurrentTargetId = null;
        let fetchedCurrentTargetName = 'None';
        let fetchedGlobalTableLayouts = {};
        let fetchedAppSettings = {
            ui: { ShowSynackSection: false, DefaultTheme: 'light' },
            missions: { enabled: false }
        };

        try {
            const currentTargetSetting = await apiService.getCurrentTargetSetting();
            if (currentTargetSetting && typeof currentTargetSetting.target_id === 'number' && currentTargetSetting.target_id !== 0) {
                fetchedCurrentTargetId = currentTargetSetting.target_id;
                if (fetchedCurrentTargetId !== 0) { 
                    try {
                        const targetDetails = await apiService.getTargetDetails(fetchedCurrentTargetId);
                        fetchedCurrentTargetName = targetDetails.codename || 'Unknown';
                    } catch (targetDetailsError) {
                        console.error(`Error fetching details for target ID ${fetchedCurrentTargetId}:`, targetDetailsError);
                        fetchedCurrentTargetName = 'Unknown (Error)';
                    }
                }
            } 
            fetchedGlobalTableLayouts = await apiService.getTableLayouts();
            const appSettingsFromApi = await apiService.getAppSettings();
            if (appSettingsFromApi && appSettingsFromApi.ui !== undefined && appSettingsFromApi.missions !== undefined) {
                fetchedAppSettings = appSettingsFromApi;
            } else {
                console.warn("[App.js] getAppSettings did not return the expected structure. Using defaults.");
            }
        } catch (error) {
            console.error("Error fetching initial settings:", error);
        } finally {
            initState({
                currentTargetId: fetchedCurrentTargetId,
                currentTargetName: fetchedCurrentTargetName,
                globalTableLayouts: fetchedGlobalTableLayouts,
                appSettings: fetchedAppSettings
            });
            applyUiSettings(fetchedAppSettings.ui);
            if (currentTargetDisplay) {
                const appState = getState(); 
                currentTargetDisplay.textContent = `Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId || 'None'})`;
                currentTargetDisplay.title = `Current Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId || 'None'})`;
            }
        }
    }

    async function displayAppVersion() {
        try {
            const versionData = await apiService.getVersion();
            const versionDisplay = document.getElementById('appVersionDisplay');
            if (versionDisplay && versionData && versionData.version) {
                versionDisplay.textContent = `v${versionData.version}`;
            }
        } catch (error) {
            console.error("Failed to fetch app version:", error);
            const versionDisplay = document.getElementById('appVersionDisplay');
            if (versionDisplay) {
                versionDisplay.textContent = 'v?.?.?';
            }
        }
    }
    displayAppVersion();
});
