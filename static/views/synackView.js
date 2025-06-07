import { escapeHtml, escapeHtmlAttribute, downloadCSV } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
// tableService is not directly used by Synack views for resizable columns in the provided app.js code

// DOM element references (will be queried within functions or passed)
let viewContentContainer; // Main container, passed to load functions

/**
 * Initializes the Synack View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService.
 */
export function initSynackView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    console.log("[SynackView] Initialized.");
}

// --- Synack Targets View ---

function handleSynackTargetsSort(event) {
    const newSortBy = event.target.closest('th').getAttribute('data-sort-key');
    if (!newSortBy) return;

    const appState = stateService.getState();
    const currentSynackTargetsState = appState.paginationState.synackTargets;
    let newSortOrder = (currentSynackTargetsState.sortBy === newSortBy && currentSynackTargetsState.sortOrder === 'ASC') ? 'DESC' : 'ASC';

    window.location.hash = `#synack-targets?page=1&sort_by=${newSortBy}&sort_order=${newSortOrder}`;
}

function renderSynackPaginationControls(container) {
    if (!container) return;
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords, sortBy, sortOrder } = appState.paginationState.synackTargets;

    let paginationHTML = '';

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total target(s) found.</p>` : '';
        return;
    }

    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total targets)</p>`;
    const buildHash = (page) => `#synack-targets?page=${page}&sort_by=${sortBy}&sort_order=${sortOrder}`;

    const prevButton = document.createElement('button');
    prevButton.className = 'secondary';
    prevButton.style.marginRight = '5px';
    prevButton.innerHTML = '&laquo; Previous';
    if (currentPage <= 1) prevButton.disabled = true;
    prevButton.addEventListener('click', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); });

    const nextButton = document.createElement('button');
    nextButton.className = 'secondary';
    nextButton.innerHTML = 'Next &raquo;';
    if (currentPage >= totalPages) nextButton.disabled = true;
    nextButton.addEventListener('click', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    if (currentPage > 1) container.appendChild(prevButton);
    if (currentPage < totalPages) container.appendChild(nextButton);
}

async function handlePromoteSelectedSynackTargets() {
    const selectedCheckboxes = document.querySelectorAll('.synack-target-checkbox:checked');
    const platformIdInput = document.getElementById('promoteToPlatformId');
    const messageArea = document.getElementById('synackTargetListMessage');

    if (!messageArea || !platformIdInput) {
        console.error("Required elements for promotion not found.");
        uiService.showModalMessage("UI Error", "Could not find necessary elements on the page for promotion.");
        return;
    }
    messageArea.textContent = '';
    messageArea.className = 'message-area';

    if (selectedCheckboxes.length === 0) {
        uiService.showModalMessage('Selection Needed', 'Please select at least one Synack target to promote.');
        return;
    }
    if (!platformIdInput.value) {
        uiService.showModalMessage('Platform ID Needed', 'Please enter a Platform ID to promote these targets to.');
        platformIdInput.focus();
        return;
    }
    const platformId = parseInt(platformIdInput.value, 10);
    if (isNaN(platformId) || platformId <= 0) {
        uiService.showModalMessage('Invalid Platform ID', 'Platform ID must be a positive number.');
        platformIdInput.focus();
        return;
    }

    let successCount = 0;
    let errorCount = 0;
    let errorMessages = [];

    uiService.showModalMessage('Processing...', `Promoting ${selectedCheckboxes.length} target(s)... Please wait.`);

    for (const checkbox of selectedCheckboxes) {
        const synackTargetIdStr = checkbox.value;
        const codename = checkbox.getAttribute('data-codename') || checkbox.getAttribute('data-name') || synackTargetIdStr;
        const linkOverride = "#NEEDS_LINK_UPDATE";

        const payload = {
            synack_target_id_str: synackTargetIdStr,
            platform_id: platformId,
            codename_override: codename,
            link_override: linkOverride,
        };

        try {
            await apiService.promoteSynackTarget(payload);
            successCount++;
        } catch (error) {
            errorCount++;
            errorMessages.push(`Failed to promote ${escapeHtml(codename)} (Synack ID: ${escapeHtml(synackTargetIdStr)}): ${escapeHtml(error.message)}`);
        }
    }

    let finalMessage = `${successCount} target(s) promoted successfully.`;
    if (errorCount > 0 || errorMessages.length > 0) {
        finalMessage += `\n${errorCount} target(s) failed to promote:\n` + errorMessages.join('\n');
        messageArea.classList.add('error-message');
    } else {
        messageArea.classList.add('success-message');
    }
    uiService.showModalMessage('Promotion Complete', finalMessage.replace(/\n/g, '<br>'));
    loadSynackTargetsView(viewContentContainer); // Refresh the list
}

/**
 * Loads the Synack targets view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadSynackTargetsView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadSynackTargetsView!");
        return;
    }
    if (!apiService || !uiService || !stateService) {
        console.error("SynackView not initialized. Call initSynackView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SynackView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentPage, limit, sortBy, sortOrder } = appState.paginationState.synackTargets;

    viewContentContainer.innerHTML = `
        <h1>Synack Targets</h1>
        <div class="form-group inline-form" style="margin-bottom:20px;">
            <label for="promoteToPlatformId" style="margin-right: 10px; white-space: nowrap;">Promote selected to Platform ID:</label>
            <input type="number" id="promoteToPlatformId" placeholder="Platform ID" style="width: 100px; margin-right: 10px;">
            <button id="promoteSelectedSynackTargetsBtn" class="primary">Promote Selected</button>
        </div>
        <div id="synackTargetListMessage" class="message-area" style="margin-bottom: 15px;"></div>
        <div id="synackTargetList">Loading active Synack targets (Page ${currentPage}, Sort: ${sortBy} ${sortOrder})...</div>
        <div id="synackPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
    `;
    const listDiv = document.getElementById('synackTargetList');
    const paginationControlsDiv = document.getElementById('synackPaginationControls');

    try {
        const params = { active_only: true, page: currentPage, limit: limit, sort_by: sortBy, sort_order: sortOrder };
        const apiResponse = await apiService.getSynackTargets(params);
        const synackTargets = apiResponse.targets || [];

        stateService.updateState({
            paginationState: {
                synackTargets: {
                    ...appState.paginationState.synackTargets,
                    currentPage: apiResponse.page || 1,
                    totalPages: apiResponse.total_pages || 1,
                    totalRecords: apiResponse.total_records || 0,
                }
            }
        });

        if (!listDiv) return;

        const sortableHeaders = [
            { label: '<input type="checkbox" id="selectAllSynackTargets" title="Select/Deselect All on this page">', sortKey: null, isHtml: true },
            { label: 'Synack ID', sortKey: 'synack_target_id_str' },
            { label: 'Codename (DB ID) <i class="fas fa-info-circle" title="Internal database ID for the target."></i>', sortKey: 'codename', isHtml: true },
            { label: 'Name', sortKey: 'name' },
            { label: 'Findings <i class="fas fa-bug" title="Number of findings associated with this target."></i>', sortKey: 'findings_count', isHtml: true },
            { label: 'Status', sortKey: 'status' },
            { label: 'Last Seen', sortKey: 'last_seen_timestamp' }
        ];

        if (synackTargets.length > 0) {
            let tableHTML = `<table><thead><tr>
                ${sortableHeaders.map(h => {
                let classes = h.sortKey ? 'sortable' : '';
                if (h.sortKey === sortBy) classes += sortOrder === 'ASC' ? ' sorted-asc' : ' sorted-desc';
                const content = h.isHtml ? h.label : escapeHtml(h.label);
                return `<th class="${classes}" ${h.sortKey ? `data-sort-key="${h.sortKey}"` : ''}>${content}</th>`;
            }).join('')}
            </tr></thead><tbody>`;
            synackTargets.forEach(target => {
                const safeCodename = escapeHtml(target.codename);
                const safeName = escapeHtml(target.name);
                const safeStatus = escapeHtml(target.status);
                const lastSeen = target.last_seen_timestamp ? new Date(target.last_seen_timestamp).toLocaleDateString() : 'N/A';
                const findingsCount = target.findings_count !== undefined ? target.findings_count : '-';
                tableHTML += `
                    <tr data-synack-id-str="${target.synack_target_id_str}">
                        <td><input type="checkbox" class="synack-target-checkbox" value="${target.synack_target_id_str}" data-codename="${safeCodename}" data-name="${safeName}"></td>
                        <td><a href="#synack-analytics?target_db_id=${target.db_id}" title="View Analytics for ${safeCodename}">${escapeHtml(target.synack_target_id_str)}</a></td>
                        <td>${safeCodename} (${target.db_id})</td>
                        <td>${safeName}</td>
                        <td>${findingsCount}</td>
                        <td>${safeStatus}</td>
                        <td>${lastSeen}</td>
                    </tr>`;
            });
            tableHTML += `</tbody></table>`;
            listDiv.innerHTML = tableHTML;

            const selectAllCheckbox = document.getElementById('selectAllSynackTargets');
            if (selectAllCheckbox) {
                selectAllCheckbox.addEventListener('change', (event) => {
                    document.querySelectorAll('.synack-target-checkbox').forEach(checkbox => checkbox.checked = event.target.checked);
                });
            }
        } else {
            listDiv.innerHTML = '<p>No active Synack targets found for this page.</p>';
        }
        renderSynackPaginationControls(paginationControlsDiv);

    } catch (error) {
        if (listDiv) listDiv.innerHTML = `<p class="error-message">Error loading Synack targets: ${escapeHtml(error.message)}</p>`;
        if (paginationControlsDiv) paginationControlsDiv.innerHTML = '';
    }

    document.querySelectorAll('#synackTargetList th.sortable').forEach(th => {
        th.removeEventListener('click', handleSynackTargetsSort);
        th.addEventListener('click', handleSynackTargetsSort);
    });

    const promoteBtn = document.getElementById('promoteSelectedSynackTargetsBtn');
    if (promoteBtn) {
        promoteBtn.removeEventListener('click', handlePromoteSelectedSynackTargets);
        promoteBtn.addEventListener('click', handlePromoteSelectedSynackTargets);
    }
}

// --- Synack Analytics View ---

function handleSynackAnalyticsSort(event) {
    const newSortBy = event.target.closest('th').getAttribute('data-sort-key');
    if (!newSortBy) return;

    const appState = stateService.getState();
    const currentAnalyticsState = appState.paginationState.synackAnalytics;
    let newSortOrder = (currentAnalyticsState.sortBy === newSortBy && currentAnalyticsState.sortOrder === 'ASC') ? 'DESC' : 'ASC';

    let hash = `#synack-analytics?page=1&sort_by=${newSortBy}&sort_order=${newSortOrder}`;
    if (currentAnalyticsState.targetDbId) {
        hash += `&target_db_id=${currentAnalyticsState.targetDbId}`;
    }
    window.location.hash = hash;
}

function renderSynackAnalyticsPaginationControls(container) {
    if (!container) return;
    const appState = stateService.getState();
    const { targetDbId, currentPage, totalPages, totalRecords, sortBy, sortOrder } = appState.paginationState.synackAnalytics;
    const itemType = targetDbId ? "finding(s)" : "analytic entry(s)";
    let paginationHTML = '';

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total ${itemType} found.</p>` : '';
        return;
    }
    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total ${itemType})</p>`;
    const buildHash = (page) => {
        let hash = `#synack-analytics?page=${page}&sort_by=${sortBy}&sort_order=${sortOrder}`;
        if (targetDbId) hash += `&target_db_id=${targetDbId}`;
        return hash;
    };

    const prevButton = document.createElement('button');
    prevButton.className = 'secondary';
    prevButton.style.marginRight = '5px';
    prevButton.innerHTML = '&laquo; Previous';
    if (currentPage <= 1) prevButton.disabled = true;
    prevButton.addEventListener('click', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); });

    const nextButton = document.createElement('button');
    nextButton.className = 'secondary';
    nextButton.innerHTML = 'Next &raquo;';
    if (currentPage >= totalPages) nextButton.disabled = true;
    nextButton.addEventListener('click', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    if (currentPage > 1) container.appendChild(prevButton);
    if (currentPage < totalPages) container.appendChild(nextButton);
}

function convertSynackAnalyticsToCSV(jsonData, isGlobalView) {
    const escapeCSVField = (field) => {
        if (field === null || field === undefined) return '';
        let str = String(field);
        if (str.search(/("|,|\n)/g) >= 0) {
            str = '"' + str.replace(/"/g, '""') + '"';
        }
        return str;
    };
    let headers = [];
    let rows = [];

    if (isGlobalView) {
        headers = ["Target Codename", "Category Name", "Findings Count"];
        jsonData.forEach(item => {
            rows.push([
                escapeCSVField(item.target_codename),
                escapeCSVField(item.category_name),
                escapeCSVField(item.count)
            ].join(','));
        });
    } else {
        headers = ["Finding ID", "Title", "Category", "Severity", "Status", "Reported At", "Vulnerability URL"];
        jsonData.forEach(item => {
            const reportedAtDate = item.reported_at ? new Date(item.reported_at).toISOString().split('T')[0] : '';
            rows.push([
                escapeCSVField(item.synack_finding_id),
                escapeCSVField(item.title),
                escapeCSVField(item.category_name),
                escapeCSVField(item.severity),
                escapeCSVField(item.status),
                escapeCSVField(reportedAtDate),
                escapeCSVField(item.vulnerability_url)
            ].join(','));
        });
    }
    return [headers.join(',')].concat(rows).join('\n');
}

async function handleExportSynackAnalyticsToCSV() {
    const appState = stateService.getState();
    const { targetDbId, sortBy, sortOrder } = appState.paginationState.synackAnalytics;
    const isGlobalView = !targetDbId;
    const exportLimit = 1000000;
    let itemsToExport;

    uiService.showModalMessage("Exporting...", "Fetching all data for CSV export. This may take a moment...");

    try {
        if (isGlobalView) {
            const apiResponse = await apiService.getGlobalSynackAnalytics({ page: 1, limit: exportLimit, sort_by: sortBy, sort_order: sortOrder });
            itemsToExport = apiResponse.analytics || [];
        } else {
            const apiResponse = await apiService.getTargetSynackAnalytics(targetDbId, { page: 1, limit: exportLimit, sort_by: sortBy, sort_order: sortOrder });
            itemsToExport = apiResponse.findings || [];
        }

        if (itemsToExport.length === 0) {
            uiService.showModalMessage("Export Complete", "No data available to export.");
            return;
        }

        const csvString = convertSynackAnalyticsToCSV(itemsToExport, isGlobalView);
        const filename = isGlobalView ? 'synack_global_analytics.csv' : `synack_target_${targetDbId}_findings.csv`;
        downloadCSV(csvString, filename);
        uiService.hideModal();

    } catch (error) {
        uiService.showModalMessage("Export Error", `Failed to fetch data for CSV: ${escapeHtml(error.message)}`);
    }
}

async function handleRefreshSynackFindings() {
    const appState = stateService.getState();
    const { targetDbId } = appState.paginationState.synackAnalytics;
    if (!targetDbId) return;

    uiService.showModalMessage("Refreshing Findings", "Requesting a refresh of findings data... Please wait.");
    try {
        await apiService.refreshSynackFindings(targetDbId);
        uiService.showModalMessage(
            "Refresh Initiated",
            "Findings refresh has been queued for this target.<br><br>" +
            "To complete the refresh, please ensure the proxy intercepts a request to the Synack target list (e.g., refresh the Synack platform page where targets are listed).<br><br>" +
            "The data will update after the next successful processing of the target list by the proxy."
        );
    } catch (error) {
        uiService.showModalMessage("Refresh Error", `Failed to request a findings refresh: ${escapeHtml(error.message)}`);
    }
}

async function fetchAndDisplaySynackAnalytics() {
    const listDiv = document.getElementById('synackAnalyticsList');
    const paginationControlsDiv = document.getElementById('synackAnalyticsPaginationControls');

    if (!listDiv || !paginationControlsDiv) {
        console.error("Synack analytics list or pagination container not found.");
        return;
    }

    const appState = stateService.getState();
    const { targetDbId, currentPage, limit, sortBy, sortOrder } = appState.paginationState.synackAnalytics;
    const isGlobalView = !targetDbId;

    listDiv.innerHTML = `<p>Fetching ${isGlobalView ? 'all Synack analytics' : `analytics for target DB ID ${targetDbId}`}, page ${currentPage}, sort by ${sortBy} ${sortOrder}...</p>`;

    try {
        let apiResponse;
        if (isGlobalView) {
            apiResponse = await apiService.getGlobalSynackAnalytics({ page: currentPage, limit: limit, sort_by: sortBy, sort_order: sortOrder });
        } else {
            apiResponse = await apiService.getTargetSynackAnalytics(targetDbId, { page: currentPage, limit: limit, sort_by: sortBy, sort_order: sortOrder });
        }
        const itemsToDisplay = isGlobalView ? (apiResponse.analytics || []) : (apiResponse.findings || []);

        stateService.updateState({
            paginationState: {
                synackAnalytics: {
                    ...appState.paginationState.synackAnalytics,
                    currentPage: apiResponse.page || 1,
                    totalPages: apiResponse.total_pages || 1,
                    totalRecords: apiResponse.total_records || 0,
                }
            }
        });

        let sortableHeaders = [];
        if (isGlobalView) {
            sortableHeaders.push({ label: 'Target', sortKey: 'target_codename' });
            sortableHeaders.push({ label: 'Category Name', sortKey: 'category_name' }, { label: 'Findings Count', sortKey: 'count' });
        } else {
            sortableHeaders = [
                { label: 'Finding ID', sortKey: 'synack_finding_id' },
                { label: 'Category', sortKey: 'category_name' },
                { label: 'Status', sortKey: 'status' },
                { label: 'Reported', sortKey: 'reported_at' },
            ];
        }

        if (itemsToDisplay.length > 0) {
            let tableHTML = `<table><thead><tr>
                ${sortableHeaders.map(h => {
                let classes = h.sortKey ? 'sortable' : '';
                if (h.sortKey === sortBy) classes += sortOrder === 'ASC' ? ' sorted-asc' : ' sorted-desc';
                return `<th class="${classes}" ${h.sortKey ? `data-sort-key="${h.sortKey}"` : ''}>${escapeHtml(h.label)}</th>`;
            }).join('')}
            </tr></thead><tbody>`;
            itemsToDisplay.forEach(item => {
                tableHTML += `<tr>`;
                if (isGlobalView) {
                    const targetLink = item.target_db_id ? `<a href="#synack-analytics?target_db_id=${item.target_db_id}" title="View analytics for ${escapeHtml(item.target_codename)}">${escapeHtml(item.target_codename || 'N/A')}</a>` : escapeHtml(item.target_codename || 'N/A');
                    tableHTML += `<td>${targetLink}</td>`;
                    tableHTML += `<td>${escapeHtml(item.category_name)}</td>`;
                    tableHTML += `<td>${item.count}</td>`;
                } else {
                    const reportedAtDate = item.reported_at ? new Date(item.reported_at).toLocaleDateString() : 'N/A';
                    tableHTML += `<td>${escapeHtml(item.synack_finding_id || '-')}</td>`;
                    tableHTML += `<td>${escapeHtml(item.category_name || '-')}</td>`;
                    tableHTML += `<td>${escapeHtml(item.status || '-')}</td>`;
                    tableHTML += `<td>${reportedAtDate}</td>`;
                }
                tableHTML += `</tr>`;
            });
            tableHTML += `</tbody></table>`;
            listDiv.innerHTML = tableHTML;
        } else {
            const itemTypeMessage = isGlobalView ? "analytics data" : "findings";
            listDiv.innerHTML = `<p>No ${itemTypeMessage} found ${isGlobalView ? 'across all targets' : 'for this Synack target'}.</p>`;
        }
        renderSynackAnalyticsPaginationControls(paginationControlsDiv);

        document.querySelectorAll('#synackAnalyticsList th.sortable').forEach(th => {
            th.removeEventListener('click', handleSynackAnalyticsSort);
            th.addEventListener('click', handleSynackAnalyticsSort);
        });

    } catch (error) {
        listDiv.innerHTML = `<p class="error-message">Error loading Synack analytics: ${escapeHtml(error.message)}</p>`;
        paginationControlsDiv.innerHTML = '';
    }
}

/**
 * Loads the Synack analytics view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadSynackAnalyticsView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadSynackAnalyticsView!");
        return;
    }
    if (!apiService || !uiService || !stateService) {
        console.error("SynackView not initialized. Call initSynackView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SynackView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { targetDbId, currentPage, sortBy, sortOrder } = appState.paginationState.synackAnalytics;
    let headerText = "All Synack Target Analytics";

    if (targetDbId) {
        let specificTargetCodename = `Target DB ID ${targetDbId}`;
        try {
            const targetDetails = await apiService.getSynackTargetDetails(targetDbId);
            specificTargetCodename = targetDetails.codename || targetDetails.name || specificTargetCodename;
        } catch (e) { console.warn("Could not fetch Synack target details for title:", e); }
        headerText = `Analytics for Synack Target: ${escapeHtml(specificTargetCodename)} (DB ID: ${targetDbId})`;
    }

    viewContentContainer.innerHTML = `
        <h1>${headerText}</h1>
        <div style="margin-bottom: 15px;">
            ${targetDbId ? '<button id="refreshSynackFindingsBtn" class="secondary" style="margin-right:10px;">Refresh Findings</button>' : ''}
            <button id="exportSynackAnalyticsCsvBtn" class="secondary">Export to CSV</button>
        </div>
        <div id="synackAnalyticsList">Loading analytics (Page ${currentPage}, Sort: ${sortBy} ${sortOrder})...</div>
        <div id="synackAnalyticsPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
    `;
    await fetchAndDisplaySynackAnalytics();
    document.getElementById('exportSynackAnalyticsCsvBtn')?.addEventListener('click', handleExportSynackAnalyticsToCSV);
    document.getElementById('refreshSynackFindingsBtn')?.addEventListener('click', handleRefreshSynackFindings);
}
