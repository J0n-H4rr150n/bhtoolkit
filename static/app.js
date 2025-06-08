// static/app.js
import { initState, getState, updateState } from './stateService.js';
import { initRouter } from './router.js';
import { initUIService, showModalMessage, showModalConfirm, updateBreadcrumbs } from './uiService.js';
import * as apiService from './apiService.js'; // Keep as is
import { initTableService, saveCurrentTableLayout, makeTableColumnsResizable, getIsResizing } from './tableService.js'; // Import getIsResizing
import { escapeHtml, escapeHtmlAttribute, debounce, copyToClipboard, downloadCSV } from './utils.js';

// View Module Imports
import {
    loadPlatformsView as loadPlatformsViewModule,
    handleAddPlatform as handleAddPlatformModule,
    fetchAndDisplayPlatforms as fetchAndDisplayPlatformsModule
} from './views/platformView.js';
import { initTargetView, loadTargetsView, cancelActiveTargetEdit } from './views/targetView.js';
import { initProxyLogView, loadProxyLogView, loadProxyLogDetailView } from './views/proxyLogView.js';
import { initSynackView, loadSynackTargetsView, loadSynackAnalyticsView } from './views/synackView.js';
import { initChecklistView, fetchAndDisplayChecklistItems, cancelActiveChecklistItemEdit } from './views/checklistView.js';
import { initChecklistTemplateView, loadChecklistTemplatesView } from './views/checklistTemplateView.js';
import { initSettingsView, loadSettingsView } from './views/settingsView.js';
import { initSitemapView, loadSitemapView } from './views/sitemapView.js';


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

    const stateServiceAPI = {
        getState,
        updateState
    };

    const uiServiceAPI = {
        showModalMessage,
        showModalConfirm,
        updateBreadcrumbs
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
    initSettingsView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI });
    initSitemapView({ apiService, uiService: uiServiceAPI, stateService: stateServiceAPI, tableService: tableServiceAPI });

    await fetchAndSetInitialCurrentTarget();

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
            loadTargetsView: (platformId) => loadTargetsView(viewContentContainer, platformId),
            loadCurrentTargetView: (targetId) => loadCurrentTargetView(targetId),
            loadSynackTargetsView: () => loadSynackTargetsView(viewContentContainer),
            loadSynackAnalyticsView: () => loadSynackAnalyticsView(viewContentContainer),
            loadProxyLogView: (proxyLogParams) => loadProxyLogView(viewContentContainer, proxyLogParams),
            loadProxyLogDetailView: (logId) => loadProxyLogDetailView(viewContentContainer, logId),
            loadChecklistTemplatesView: () => loadChecklistTemplatesView(viewContentContainer),
            loadSettingsView: () => {
                console.log('[App.js] Router attempting to call loadSettingsView.');
                loadSettingsView(viewContentContainer);
            },
            loadSitemapView: () => loadSitemapView(viewContentContainer)
        },
        getPlatformDetailsFunc: apiService.getPlatformDetails,
        cancelTargetEditFunc: cancelActiveTargetEdit,
        cancelChecklistItemEditFunc: cancelActiveChecklistItemEdit
    });

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
                    <div style="margin-top: 20px;">
                        ${target.id === appState.currentTargetId
                            ? '<button id="clearCurrentTargetBtn" class="secondary">Clear This as Current Target</button>'
                            : `<button class="action-button set-current-target primary" data-id="${target.id}" data-name="${escapeHtml(target.codename)}">Set as Current Target</button>`}
                    </div>
                    <p><strong>ID:</strong> ${target.id}</p>
                    <p><strong>Slug:</strong> ${escapeHtml(target.slug)}</p>
                    <p><strong>Link:</strong> <a href="${escapeHtml(target.link)}" target="_blank">${escapeHtml(target.link)}</a></p>
                    <p><strong>Platform ID:</strong> ${target.platform_id} (${escapeHtml(platformNameForBreadcrumb)})</p>

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
                                <pre id="targetNotesContent" style="white-space: pre-wrap; word-wrap: break-word; padding: 10px; background-color: #f8f9fa; border: 1px solid #e9ecef; border-radius: 4px; min-height: 100px;">${escapeHtml(target.notes || '(None)')}</pre>
                            </div>
                            <div class="notes-edit-mode" style="display:none;">
                                <p><strong>Edit Notes:</strong></p>
                                <textarea id="targetNotesTextarea" rows="10" style="width: 100%; margin-bottom: 10px;"></textarea>
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
                        <div id="addScopeRuleMessage" class="message-area" style="margin-bottom: 15px;"></div>
                        <div class="scope-forms-container" style="display: flex; gap: 20px; margin-bottom:20px;">
                            <form id="addInScopeRuleForm" class="scope-rule-form" data-in-scope="true" style="flex:1; padding:15px; border:1px solid #ddd; border-radius:4px; background-color:#f9f9f9;">
                                <h4>Add In-Scope Item</h4>
                                <input type="hidden" name="target_id" value="${target.id}">
                                <div class="form-group"><label for="inScopePattern">Pattern:</label><input type="text" id="inScopePattern" name="pattern" placeholder="e.g., *.example.com" required></div>
                                <div class="form-group"><label for="inScopeItemType">Type:</label><select id="inScopeItemType" name="item_type">
                                    <option value="domain">Domain</option><option value="subdomain">Subdomain</option><option value="ip_address">IP Address</option><option value="cidr">CIDR</option><option value="url_path">URL Path</option>
                                </select></div>
                                <div class="form-group"><label for="inScopeDescription">Description:</label><input type="text" id="inScopeDescription" name="description" placeholder="Optional description"></div>
                                <button type="submit" class="primary">Add In-Scope</button>
                            </form>
                            <form id="addOutOfScopeRuleForm" class="scope-rule-form" data-in-scope="false" style="flex:1; padding:15px; border:1px solid #ddd; border-radius:4px; background-color:#f9f9f9;">
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
                renderScopeRulesTable(document.getElementById('current-target-scope'), target.scope_rules || []);

                const clearBtn = document.getElementById('clearCurrentTargetBtn');
                if(clearBtn) clearBtn.addEventListener('click', handleClearCurrentTarget);

                const setBtn = viewContentContainer.querySelector('.set-current-target');
                if(setBtn) setBtn.addEventListener('click', handleSetCurrentTargetFromDetails);

                document.getElementById('addInScopeRuleForm')?.addEventListener('submit', handleAddScopeRule);
                document.getElementById('addOutOfScopeRuleForm')?.addEventListener('submit', handleAddScopeRule);

                document.getElementById('editTargetNotesBtn')?.addEventListener('click', handleEditTargetNotes);
                document.getElementById('saveTargetNotesBtn')?.addEventListener('click', handleSaveTargetNotes);
                document.getElementById('cancelTargetNotesBtn')?.addEventListener('click', cancelTargetNotesEdit);

                setActiveTab(tabToMakeActive || 'checklistTab'); // Use passed tab or default
                document.querySelectorAll('.tabs .tab-button').forEach(button => button.addEventListener('click', handleTabSwitch));
                fetchAndDisplayChecklistItems(target.id);
                fetchAndDisplayTargetFindings(target.id); // New function call

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

    function handleEditTargetNotes() {
        const notesSection = document.getElementById('targetNotesSection');
        const displayMode = notesSection.querySelector('.notes-display-mode');
        const editMode = notesSection.querySelector('.notes-edit-mode');
        const notesContent = document.getElementById('targetNotesContent');
        const notesTextarea = document.getElementById('targetNotesTextarea');
        notesTextarea.value = notesContent.textContent === '(None)' ? '' : notesContent.textContent;
        displayMode.style.display = 'none';
        editMode.style.display = 'block';
        notesTextarea.focus();
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
            document.getElementById('targetNotesContent').textContent = updatedTarget.notes || '(None)';
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
        document.getElementById('saveTargetNotesMessage').textContent = '';
        displayMode.style.display = 'block';
        editMode.style.display = 'none';
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

    async function fetchAndDisplayTargetFindings(targetId) {
        const findingsContentDiv = document.getElementById('targetFindingsContent');
        if (!findingsContentDiv) {
            console.error("targetFindingsContent div not found!");
            return;
        }
        findingsContentDiv.innerHTML = '<p>Loading findings...</p>';

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
                            <td>${new Date(finding.discovered_at).toLocaleDateString()}</td>
                            <td><button class="action-button view-finding-detail" data-finding-id="${finding.id}">👁️</button> <button class="action-button edit-finding" data-finding-id="${finding.id}">✏️</button> <button class="action-button delete-finding danger" data-finding-id="${finding.id}">🗑️</button></td>
                        </tr>`;
                });
                findingsHTML += `</tbody></table>`;
                findingsContentDiv.innerHTML = findingsHTML;
                // Add event listeners for view/edit/delete finding buttons later
            } else {
                findingsContentDiv.innerHTML = '<p>No findings recorded for this target yet.</p> <button id="addNewFindingBtn" class="primary small-button">Add New Finding</button>';
            }
            // Add event listener for "Add New Finding" button later
        } catch (error) {
            console.error("Error fetching target findings:", error);
            findingsContentDiv.innerHTML = `<p class="error-message">Error loading findings: ${error.message}</p>`;
        }
    }

    function applyUiSettings(settings) {
        const showSynack = settings && settings.showSynackSection === true;
        const synackHeader = document.getElementById('synack-sidebar-header');
        const synackTargetsItem = document.querySelector('.sidebar-item[data-view="synack-targets"]');
        const synackAnalyticsItem = document.querySelector('.sidebar-item[data-view="synack-analytics"]');
        if (synackHeader) synackHeader.classList.toggle('hidden', !showSynack);
        if (synackTargetsItem) synackTargetsItem.classList.toggle('hidden', !showSynack);
        if (synackAnalyticsItem) synackAnalyticsItem.classList.toggle('hidden', !showSynack);
    }

    async function fetchAndSetInitialCurrentTarget() {
        const currentTargetDisplay = document.getElementById('currentPlatformTarget');
        let fetchedCurrentTargetId = null;
        let fetchedCurrentTargetName = 'None';
        let fetchedGlobalTableLayouts = {};
        let fetchedUiSettings = { showSynackSection: false }; 

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
            fetchedUiSettings = await apiService.getUISettings();

        } catch (error) {
            console.error("Error fetching initial settings:", error);
        } finally {
            initState({
                currentTargetId: fetchedCurrentTargetId,
                currentTargetName: fetchedCurrentTargetName,
                globalTableLayouts: fetchedGlobalTableLayouts
            });
            applyUiSettings(fetchedUiSettings);

            if (currentTargetDisplay) {
                const appState = getState(); 
                currentTargetDisplay.textContent = `Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId || 'None'})`;
                currentTargetDisplay.title = `Current Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId || 'None'})`;
            }
            console.log('[App.js] fetchAndSetInitialCurrentTarget completed. State initialized with targetId:', getState().currentTargetId);
        }
    }
});
