// static/views/domainsView.js
import { escapeHtml, escapeHtmlAttribute, debounce, downloadCSV, downloadTXT } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService;

// DOM element references
let viewContentContainer;
let currentDomainsData = []; // To store currently displayed domains for export
let httpxStatusIntervalId = null; // For polling httpx status
let subfinderStatusIntervalId = null; // For polling subfinder status

/**
 * Initializes the Domains View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService, tableService.
 */
export function initDomainsView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService;
    console.log("[DomainsView] Initialized.");
}

/**
 * Stops any active httpx status polling.
 */
function stopHttpxStatusUpdates() {
    if (httpxStatusIntervalId) {
        clearInterval(httpxStatusIntervalId);
        httpxStatusIntervalId = null;
        console.log("[DomainsView] Httpx status polling stopped.");
        // Optionally clear the message after a delay if needed
    }
}


/**
 * Stops any active subfinder status polling.
 */
function stopSubfinderStatusUpdates() {
    if (subfinderStatusIntervalId) {
        clearInterval(subfinderStatusIntervalId);
        subfinderStatusIntervalId = null;
        console.log("[DomainsView] Subfinder status polling stopped.");
    }
}

/**
 * Helper function to set message content and class, and add a close button.
 * @param {HTMLElement} messageAreaElement - The div element to display the message in.
 * @param {string} htmlContent - The HTML content for the message.
 * @param {string} className - The class to apply (e.g., 'success-message', 'error-message').
 * @param {number} autoHideTimeout - Milliseconds to auto-hide the message (0 for no auto-hide).
 */
function setMessageWithCloseButton(messageAreaElement, htmlContent, className, autoHideTimeout = 0) {
    if (!messageAreaElement) return;

    messageAreaElement.innerHTML = ''; // Clear previous content
    messageAreaElement.className = 'message-area'; // Reset class first

    const messageContentSpan = document.createElement('span');
    messageContentSpan.innerHTML = htmlContent; // Use innerHTML to render the content

    const closeButton = document.createElement('span');
    closeButton.innerHTML = '&times;'; // 'X' character
    closeButton.className = 'message-close-button';
    closeButton.title = 'Close message';
    
    if (messageAreaElement.autoHideTimeoutId) {
        clearTimeout(messageAreaElement.autoHideTimeoutId);
        messageAreaElement.autoHideTimeoutId = null;
    }

    const clearMessage = () => {
        messageAreaElement.innerHTML = '';
        messageAreaElement.className = 'message-area'; 
        messageAreaElement.style.display = 'none'; 
        if (messageAreaElement.autoHideTimeoutId) {
            messageAreaElement.autoHideTimeoutId = null;
        }
    };

    closeButton.onclick = clearMessage;

    messageAreaElement.appendChild(messageContentSpan);
    messageAreaElement.appendChild(closeButton); 
    messageAreaElement.classList.add(className); 
    messageAreaElement.style.display = 'block';
    if (autoHideTimeout > 0) {
        messageAreaElement.autoHideTimeoutId = setTimeout(clearMessage, autoHideTimeout);
    }
}

/**
 * Updates the subfinder status message display.
 * @param {Object} statusData - The status data from the API.
 */
function updateSubfinderStatusDisplay(statusData) {
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (!messageArea) return;

    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    if (statusData.is_running) {
        setMessageWithCloseButton(messageArea, `${escapeHtml(statusData.message || "Processing...")} (Status refreshing...)`, 'info-message');
    } else {
        let finalMessage = statusData.message || "Subfinder task processing complete or inactive.";
        if (statusData.completed_tasks_summary) {
            finalMessage += ` ${escapeHtml(statusData.completed_tasks_summary)}`;
        }
        setMessageWithCloseButton(messageArea, escapeHtml(finalMessage), 'success-message', 7000);
        stopSubfinderStatusUpdates();
        if (currentTargetId) fetchAndRenderDomainsTable(currentTargetId);
    }
}

/**
 * Updates the httpx status message display.
 * @param {Object} statusData - The status data from the API.
 */
function updateHttpxStatusDisplay(statusData) {
    console.log("[DomainsView] updateHttpxStatusDisplay called with:", JSON.parse(JSON.stringify(statusData)));
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (!messageArea) return;

    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    let baseMessageText = statusData.message || "Querying httpx status...";
    let displayMessageHTML = escapeHtml(baseMessageText); 

    if (statusData.is_running) {
        displayMessageHTML += ` (Processed: ${statusData.domains_processed}/${statusData.domains_total})`;
        let stopButtonHTML = '';
        if (currentTargetId) {
            stopButtonHTML = `<button id="stopHttpxScanBtn" class="danger small-button" data-target-id="${currentTargetId}" style="margin-left: 10px;">Stop Scan</button>`;
        } else {
            console.warn("[DomainsView] Httpx scan is running but currentTargetId is not available for stop button.");
        }
        displayMessageHTML += stopButtonHTML;
    }

    const notInitializedMessage = "No active httpx scan for this target or status not initialized.";
    const cancelledByUserMessage = "Httpx scan cancelled by user.";

    if (!(!statusData.is_running && statusData.message === notInitializedMessage)) {
        let messageClass = 'info-message';
        if (!statusData.is_running) {
            if (statusData.last_error && statusData.message !== cancelledByUserMessage) {
                messageClass = 'error-message';
            } else {
                messageClass = 'success-message';
            }
        }
        setMessageWithCloseButton(messageArea, displayMessageHTML, messageClass, statusData.is_running ? 0 : 7000);
    } else {
        console.log("[DomainsView] Suppressing 'No active httpx scan...' message display. Polling continues.");
    }
    
    const stopBtn = document.getElementById('stopHttpxScanBtn');
    if (stopBtn) {
        stopBtn.addEventListener('click', () => handleStopHttpxScan(currentTargetId));
    }

    if (!statusData.is_running) {
        if (statusData.message !== notInitializedMessage || statusData.message === cancelledByUserMessage) {
            console.log("[DomainsView] Httpx scan is definitively not running. Stopping polling.");
            stopHttpxStatusUpdates();
            if (statusData.last_error && statusData.message !== cancelledByUserMessage) {
                setMessageWithCloseButton(messageArea, `Httpx Error: ${escapeHtml(statusData.last_error)}`, 'error-message', 10000);
            }
            if (currentTargetId) fetchAndRenderDomainsTable(currentTargetId);
        } else {
            console.log("[DomainsView] Httpx scan reported as not running, but message indicates it might be initial 'not initialized' state. Polling continues.");
        }
    }
}

/**
 * Fetches and displays the current httpx status.
 * @param {number|string} targetId - The ID of the target.
 */
async function fetchAndDisplayHttpxStatus(targetId) {
    if (!targetId) {
        stopHttpxStatusUpdates();
        return;
    }
    console.log("[DomainsView] fetchAndDisplayHttpxStatus: Fetching status for target", targetId);
    try {
        const statusData = await apiService.getHttpxStatus(targetId);
        updateHttpxStatusDisplay(statusData);
    } catch (error) {
        console.error("Error fetching httpx status:", error);
        const messageArea = document.getElementById('discoverSubdomainsMessage');
        if (messageArea) setMessageWithCloseButton(messageArea, `Error fetching httpx status: ${escapeHtml(error.message)}`, 'error-message');
        stopHttpxStatusUpdates();
    }
}

/**
 * Fetches and displays the current subfinder status.
 * @param {number|string} targetId - The ID of the target for which to fetch status.
 */
async function fetchAndDisplaySubfinderStatus(targetId) {
    if (!targetId) {
        stopSubfinderStatusUpdates();
        return;
    }
    try {
        const statusData = await apiService.getSubfinderStatus(targetId);
        updateSubfinderStatusDisplay(statusData);
    } catch (error) {
        console.error("Error fetching subfinder status:", error);
        setMessageWithCloseButton(document.getElementById('discoverSubdomainsMessage'), `Error fetching subfinder status: ${escapeHtml(error.message)}`, 'error-message');
        stopSubfinderStatusUpdates();
    }
}

/**
 * Starts polling for httpx status updates.
 * @param {number|string} targetId - The ID of the target.
 * @param {string} initialMessage - An initial message to display.
 */
function startHttpxStatusUpdates(targetId, initialMessage) {
    stopHttpxStatusUpdates();

    console.log("[DomainsView] startHttpxStatusUpdates called for target", targetId, "with initial message:", initialMessage);
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea && initialMessage) {
        setMessageWithCloseButton(messageArea, escapeHtml(initialMessage), 'info-message');
    }

    if (!targetId) return;

    fetchAndDisplayHttpxStatus(targetId);
    httpxStatusIntervalId = setInterval(() => fetchAndDisplayHttpxStatus(targetId), 5000);
    console.log("[DomainsView] Httpx status polling started for target:", targetId);
}

/**
 * Starts polling for subfinder status updates.
 * @param {number|string} targetId - The ID of the target.
 * @param {string} initialMessage - An initial message to display.
 */
function startSubfinderStatusUpdates(targetId, initialMessage) {
    stopSubfinderStatusUpdates();

    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea && initialMessage) {
        setMessageWithCloseButton(messageArea, escapeHtml(initialMessage), 'info-message');
    }

    if (!targetId) return;

    fetchAndDisplaySubfinderStatus(targetId);
    subfinderStatusIntervalId = setInterval(() => fetchAndDisplaySubfinderStatus(targetId), 5000);
    console.log("[DomainsView] Subfinder status polling started for target:", targetId);
}

/**
 * Loads the domains view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadDomainsView(mainViewContainer) {
    console.log("[DomainsView] loadDomainsView called");
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadDomainsView!");
        return;
    }

    stopSubfinderStatusUpdates();
    stopHttpxStatusUpdates();

    if (!apiService || !uiService || !stateService || !tableService ) {
        console.error("DomainsView not initialized. Call initDomainsView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>DomainsView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    if (!currentTargetId) {
        viewContentContainer.innerHTML = `
            <h1>Domains</h1>
            <p class="info-message">Please select a current target to view and manage its domains.</p>
        `;
        return;
    }

    if (!appState.paginationState.domainsView) {
        console.warn("[DomainsView] paginationState.domainsView not found, initializing with defaults.");
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: {
                    currentPage: 1, limit: 25, sortBy: 'domain_name', sortOrder: 'ASC',
                    filterDomainName: '', filterSource: '', filterIsFavorite: false, 
                    filterIsInScope: null, filterHttpxScanStatus: 'all',
                    filterHTTPStatusCode: '', filterHTTPServer: '', filterHTTPTech: '',
                    totalPages: 0, totalRecords: 0,
                }
            }
        });
    }
    if (!appState.paginationState.domainsTableLayout) {
        console.warn("[DomainsView] paginationState.domainsTableLayout not found, using defaults from stateService.js.");
    }

    viewContentContainer.innerHTML = `
        <h1>Domains for Target: ${escapeHtml(currentTargetName)} (ID: ${currentTargetId})</h1>
        <div id="domainsFilterControls" style="margin-bottom: 15px; display: flex; gap: 10px; align-items: center;">
            <input type="search" id="domainNameSearch" placeholder="Filter by Domain Name..." style="flex-grow: 1; padding: 6px 10px; border-radius: 4px; border: 1px solid #bdc3c7;">
            <input type="search" id="domainSourceSearch" placeholder="Filter by Source..." style="padding: 6px 10px; border-radius: 4px; border: 1px solid #bdc3c7;">
            <select id="domainInScopeFilter">
                <option value="">All Scope</option>
                <option value="true">In Scope</option>
                <option value="false">Out of Scope</option>
            </select>
            <div class="form-group" style="display: flex; align-items: center; margin-bottom: 0; margin-left: 10px;">
                <input type="checkbox" id="domainFavoriteFilter" style="margin-right: 5px;">
                <label for="domainFavoriteFilter" style="font-weight: normal;">Favorites Only</label>
            </div>
            <select id="domainHttpxScanStatusFilter" style="margin-left: 10px;">
                <option value="all">All (HTTPX Status)</option>
                <option value="scanned">HTTPX Scanned</option>
                <option value="not_scanned">HTTPX Not Scanned</option>
            </select>
            <button id="resetDomainFiltersBtn" class="secondary small-button">Reset</button>
        </div>
        <div id="discoverSubdomainsMessage" class="message-area" style="margin-bottom: 10px;"></div>
        <div id="domainsTableActions" style="margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center;">
            <div>
                <button id="refreshDomainsTableBtn" class="action-button" title="Refresh Domain List" style="font-size: 1.3em; margin-right: 10px; padding: 2px 5px;">🔄</button>
                <button id="addDomainBtn" class="primary small-button" style="margin-right: 10px;">Add Domain</button>
                <div class="dropdown" style="display: inline-block; margin-right: 10px;">
                    <button id="moreDomainActionsDropdownBtn" class="secondary small-button dropdown-toggle">More Actions</button>
                    <div class="dropdown-menu" id="moreDomainActionsDropdownMenu">
                        <a href="#" id="importInScopeDomainsBtnLink">Import In-Scope Domains</a>
                        <a href="#" id="deleteAllDomainsBtnLink">Delete All Domains</a>
                        <a href="#" id="sendSelectedToSubfinderBtnLink">Send Selected to Subfinder</a>
                        <a href="#" id="sendSelectedToHttpxBtnLink">Send Selected to httpx</a>
                        <a href="#" id="discoverSubdomainsBtnLink">Discover Subdomains (subfinder)</a>
                        <a href="#" id="sendAllFilteredToHttpxBtnLink">Send All Filtered to httpx</a>
                        <a href="#" id="favoriteAllFilteredBtnLink">Favorite All Filtered</a>
                    </div>
                </div>
            </div>
            <div style="display: flex; align-items: center;">
                <div class="dropdown" style="margin-left: 10px;">
                    <button id="exportDomainsDropdownBtn" class="secondary small-button dropdown-toggle">Export</button>
                    <div class="dropdown-menu" id="exportDomainsDropdownMenu">
                        <a href="#" id="exportDomainsCSV">Export as CSV</a>
                        <a href="#" id="exportDomainsTXT">Export as TXT</a>
                    </div>
                </div>
                <button id="saveDomainsLayoutBtn" class="secondary small-button" style="margin-left: 10px;">Save Column Layout</button>
            </div>
        </div>
        <div id="domainsPaginationControlsTop" class="pagination-controls" style="margin-bottom: 10px; text-align:center;"></div>
        <div id="domainsTableContainer"><p>Loading domains...</p></div>
        <div id="domainsPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
    `;

    const currentDomainsFilters = appState.paginationState.domainsView;
    document.getElementById('domainNameSearch').value = currentDomainsFilters.filterDomainName;
    document.getElementById('domainSourceSearch').value = currentDomainsFilters.filterSource;
    document.getElementById('domainInScopeFilter').value = currentDomainsFilters.filterIsInScope === null ? "" : String(currentDomainsFilters.filterIsInScope);
    document.getElementById('domainFavoriteFilter').checked = currentDomainsFilters.filterIsFavorite;
    document.getElementById('domainHttpxScanStatusFilter').value = currentDomainsFilters.filterHttpxScanStatus || 'all';

    document.getElementById('resetDomainFiltersBtn')?.addEventListener('click', resetAndFetchDomains);
    document.getElementById('domainNameSearch')?.addEventListener('input', debounce((event) => {
        const appState = stateService.getState();
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: { ...appState.paginationState.domainsView, filterDomainName: event.target.value, currentPage: 1 }
            }
        });
        fetchAndRenderDomainsTable(currentTargetId);
    }, 500));
    document.getElementById('domainSourceSearch')?.addEventListener('input', debounce((event) => {
        const appState = stateService.getState();
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: { ...appState.paginationState.domainsView, filterSource: event.target.value, currentPage: 1 }
            }
        });
        fetchAndRenderDomainsTable(currentTargetId);
    }, 500));
    document.getElementById('domainInScopeFilter')?.addEventListener('change', (event) => {
        const appState = stateService.getState();
        const value = event.target.value;
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: { ...appState.paginationState.domainsView, filterIsInScope: (value !== "") ? (value === 'true') : null, currentPage: 1 }
            }
        });
        fetchAndRenderDomainsTable(currentTargetId);
    });
    document.getElementById('domainFavoriteFilter')?.addEventListener('change', (event) => {
        const appState = stateService.getState();
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: { ...appState.paginationState.domainsView, filterIsFavorite: event.target.checked, currentPage: 1 }
            }
        });
        fetchAndRenderDomainsTable(currentTargetId);
    });
    document.getElementById('domainHttpxScanStatusFilter')?.addEventListener('change', (event) => {
        const appState = stateService.getState();
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: { ...appState.paginationState.domainsView, filterHttpxScanStatus: event.target.value, currentPage: 1 }
            }
        });
        fetchAndRenderDomainsTable(currentTargetId);
    });
    
    document.getElementById('addDomainBtn')?.addEventListener('click', () => displayAddDomainModal(currentTargetId));
    document.getElementById('saveDomainsLayoutBtn')?.addEventListener('click', () => {
        tableService.saveCurrentTableLayout('domainsTable', 'domainsTableHead');
    });

    document.getElementById('importInScopeDomainsBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleImportInScopeDomains(currentTargetId, currentTargetName); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('deleteAllDomainsBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleDeleteAllDomains(currentTargetId, currentTargetName); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('sendSelectedToSubfinderBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleSendSelectedToSubfinder(currentTargetId); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('sendSelectedToHttpxBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleSendSelectedToHttpx(currentTargetId); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('discoverSubdomainsBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); displayDiscoverSubdomainsModal(currentTargetId, currentTargetName); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('favoriteAllFilteredBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleFavoriteAllFiltered(currentTargetId); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('sendAllFilteredToHttpxBtnLink')?.addEventListener('click', (e) => { e.preventDefault(); handleSendAllFilteredToHttpx(currentTargetId); document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show'); });
    document.getElementById('refreshDomainsTableBtn')?.addEventListener('click', () => fetchAndRenderDomainsTable(currentTargetId));

    const exportDropdownBtn = document.getElementById('exportDomainsDropdownBtn');
    const exportDropdownMenu = document.getElementById('exportDomainsDropdownMenu');
    if (exportDropdownBtn && exportDropdownMenu) {
        exportDropdownBtn.addEventListener('click', (e) => { e.stopPropagation(); exportDropdownMenu.classList.toggle('show'); });
        document.getElementById('exportDomainsCSV')?.addEventListener('click', handleExportDomainsCSV);
        document.getElementById('exportDomainsTXT')?.addEventListener('click', handleExportDomainsTXT);
    }

    const moreActionsDropdownBtn = document.getElementById('moreDomainActionsDropdownBtn');
    const moreActionsDropdownMenu = document.getElementById('moreDomainActionsDropdownMenu');
    if (moreActionsDropdownBtn && moreActionsDropdownMenu) {
        moreActionsDropdownBtn.addEventListener('click', (e) => { e.stopPropagation(); moreActionsDropdownMenu.classList.toggle('show'); });
    }
    
    document.addEventListener('click', (e) => {
        const isClickInsideMoreActions = moreActionsDropdownBtn?.contains(e.target) || moreActionsDropdownMenu?.contains(e.target);
        const isClickInsideExport = exportDropdownBtn?.contains(e.target) || exportDropdownMenu?.contains(e.target);
        if (!isClickInsideMoreActions && moreActionsDropdownMenu) moreActionsDropdownMenu.classList.remove('show');
        if (!isClickInsideExport && exportDropdownMenu) exportDropdownMenu.classList.remove('show');
    });

    await fetchAndRenderDomainsTable(currentTargetId);
    await checkAndResumeActiveScans(currentTargetId);
}

async function fetchAndRenderDomainsTable(targetId) {
    const tableContainer = document.getElementById('domainsTableContainer');
    const topPaginationControlsDiv = document.getElementById('domainsPaginationControlsTop');
    const bottomPaginationControlsDiv = document.getElementById('domainsPaginationControls');

    if (!tableContainer || !topPaginationControlsDiv || !bottomPaginationControlsDiv) {
        console.error("[DomainsView] Required elements not found for fetchAndRenderDomainsTable");
        return;
    }
    tableContainer.innerHTML = '<p>Fetching domains...</p>';

    const appState = stateService.getState();
    // Use filter values directly from the state
    const {
        currentPage, limit, sortBy, sortOrder,
        filterDomainName, filterSource, filterIsInScope, filterIsFavorite,
        filterHttpxScanStatus, filterHTTPStatusCode, filterHTTPServer, filterHTTPTech
    } = appState.paginationState.domainsView;

    const params = {
        page: currentPage, limit: limit, sort_by: sortBy, sort_order: sortOrder,
        domain_name_search: filterDomainName, source_search: filterSource,
    };
    if (filterIsInScope !== null) params.is_in_scope = filterIsInScope.toString();
    if (filterIsFavorite === true) params.is_favorite = 'true';
    if (filterHttpxScanStatus && filterHttpxScanStatus !== 'all') params.httpx_scan_status = filterHttpxScanStatus;
    if (filterHTTPStatusCode) params.filter_http_status_code = filterHTTPStatusCode;
    if (filterHTTPServer) params.filter_http_server = filterHTTPServer;
    if (filterHTTPTech) params.filter_http_tech = filterHTTPTech;

    try {
        const response = await apiService.getDomains(targetId, params);
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: {
                    ...appState.paginationState.domainsView,
                    currentPage: response.page,
                    limit: response.limit,
                    totalPages: response.total_pages,
                    totalRecords: response.total_records,
                    distinctHttpStatusCodes: response.distinct_http_status_codes || [],
                    distinctHttpServers: response.distinct_http_servers || [],
                    distinctHttpTechs: response.distinct_http_techs || [],
                    sortBy: response.sort_by || sortBy,
                    sortOrder: response.sort_order || sortOrder
                }
            }
        });

        currentDomainsData = response.records || [];
        renderDomainsTable(response.records || [], response);
        
        if (topPaginationControlsDiv) renderDomainsPaginationControls(topPaginationControlsDiv, response);
        if (bottomPaginationControlsDiv) renderDomainsPaginationControls(bottomPaginationControlsDiv, response);

    } catch (error) {
        console.error("Error fetching domains:", error);
        tableContainer.innerHTML = `<p class="error-message">Error loading domains: ${escapeHtml(error.message)}</p>`;
        if (topPaginationControlsDiv) topPaginationControlsDiv.innerHTML = '';
        if (bottomPaginationControlsDiv) bottomPaginationControlsDiv.innerHTML = '';
    }
}

function renderDomainsTable(domains, apiResponseData = {}) {
    const tableContainer = document.getElementById('domainsTableContainer');
    if (!tableContainer) {
        console.error("domainsTableContainer not found in renderDomainsTable");
        return;
    }

    let appState = stateService.getState();
    const { sortBy, sortOrder } = appState.paginationState.domainsView;
    const columnConfig = appState.paginationState.domainsTableLayout || {
        checkbox: { default: '3%', id: 'col-domain-checkbox', visible: true, label: '<input type="checkbox" id="selectAllDomainsCheckbox" title="Select/Deselect All Visible">', sortKey: null, nonResizable: true, nonHideable: true, isHtmlLabel: true },
        id: { default: '4%', id: 'col-domain-row-number', visible: true, label: '#', sortKey: 'id' },
        domain_name: { default: '25%', id: 'col-domain-name', visible: true, label: 'Domain Name', sortKey: 'domain_name' },
        source: { default: '10%', id: 'col-domain-source', visible: true, label: 'Source', sortKey: 'source' },
        http_status_code: { default: '7%', id: 'col-domain-status', visible: true, label: 'Status', sortKey: 'http_status_code' },
        http_content_length: { default: '7%', id: 'col-domain-length', visible: true, label: 'Length', sortKey: 'http_content_length' },
        http_title: { default: '15%', id: 'col-domain-title', visible: true, label: 'Title', sortKey: 'http_title' },
        http_tech: { default: '15%', id: 'col-domain-tech', visible: true, label: 'Tech', sortKey: 'http_tech' },
        http_server: { default: '10%', id: 'col-domain-server', visible: true, label: 'Server', sortKey: 'http_server' },
        is_in_scope: { default: '8%', id: 'col-domain-inscope', visible: false, label: 'In Scope?', sortKey: 'is_in_scope' },
        is_wildcard_scope: { default: '8%', id: 'col-domain-wildcard', visible: false, label: 'Wildcard?', sortKey: 'is_wildcard_scope'},
        notes: { default: '15%', id: 'col-domain-notes', visible: false, label: 'Notes', sortKey: 'notes' },
        last_httpx_result: { default: '12%', id: 'col-domain-httpx-scan', visible: true, label: 'Last HTTPX Result', sortKey: 'updated_at' },
        created_at: { default: 'auto', id: 'col-domain-created', visible: false, label: 'Created At', sortKey: 'created_at' },
        actions: { default: '150px', id: 'col-domain-actions', visible: true, label: 'Actions', nonResizable: true, nonHideable: true }
    };
    const globalTableLayouts = appState.globalTableLayouts || {};
    const tableKey = 'domainsTable';
    const savedTableWidths = globalTableLayouts[tableKey]?.columns || {};

    const distinctHttpStatusCodes = apiResponseData.distinct_http_status_codes || appState.paginationState.domainsView.distinctHttpStatusCodes || [];
    const distinctHttpServers = apiResponseData.distinct_http_servers || appState.paginationState.domainsView.distinctHttpServers || [];
    const distinctHttpTechs = apiResponseData.distinct_http_techs || appState.paginationState.domainsView.distinctHttpTechs || [];

    if (!domains || domains.length === 0) {
        tableContainer.innerHTML = "<p>No domains found for this target with the current filters.</p>";
        document.getElementById('domainsPaginationControlsTop').innerHTML = '';
        document.getElementById('domainsPaginationControls').innerHTML = '';
        return;
    }

    let tableHTML = `<table style="table-layout: fixed;"><thead id="domainsTableHead"><tr>`;
    for (const key in columnConfig) {
        const col = columnConfig[key];
        if (!col.visible) continue;

        const sortableClass = col.sortKey ? 'sortable' : '';
        let sortIndicator = '';
        if (col.sortKey === sortBy) {
            sortIndicator = sortOrder === 'ASC' ? ' <span class="sort-arrow">▲</span>' : ' <span class="sort-arrow">▼</span>';
        }
        let labelContent = col.isHtmlLabel ? col.label : escapeHtml(col.label);
        const thStyleWidth = savedTableWidths[key]?.width || col.default || 'auto';

        let headerDropdownHTML = '';
        if (key === 'http_status_code' && distinctHttpStatusCodes.length > 0) {
            const currentFilter = appState.paginationState.domainsView.filterHTTPStatusCode || '';
            headerDropdownHTML = `<br><select class="table-header-filter" id="colHeaderFilter-http_status_code" data-filter-key="http_status_code" style="width: 90%; margin-top: 5px;">
                <option value="">All Status</option>
                ${distinctHttpStatusCodes.map(valObj => `<option value="${valObj.Int64}" ${String(valObj.Int64) === currentFilter ? 'selected' : ''}>${valObj.Int64}</option>`).join('')}
                <option value="NULL" ${currentFilter === 'NULL' ? 'selected' : ''}>N/A</option>
            </select>`;
        } else if (key === 'http_server' && distinctHttpServers.length > 0) {
            const currentFilter = appState.paginationState.domainsView.filterHTTPServer || '';
            headerDropdownHTML = `<br><select class="table-header-filter" id="colHeaderFilter-http_server" data-filter-key="http_server" style="width: 90%; margin-top: 5px;">
                <option value="">All Servers</option>
                ${distinctHttpServers.map(valObj => `<option value="${escapeHtmlAttribute(valObj.String)}" ${valObj.String === currentFilter ? 'selected' : ''}>${escapeHtml(valObj.String)}</option>`).join('')}
                <option value="NULL" ${currentFilter === 'NULL' ? 'selected' : ''}>N/A</option>
            </select>`;
        } else if (key === 'http_tech' && distinctHttpTechs.length > 0) {
            const currentFilter = appState.paginationState.domainsView.filterHTTPTech || '';
            headerDropdownHTML = `<br><select class="table-header-filter" id="colHeaderFilter-http_tech" data-filter-key="http_tech" style="width: 90%; margin-top: 5px;">
                <option value="">All Tech</option>
                ${distinctHttpTechs.map(valObj => `<option value="${escapeHtmlAttribute(valObj.String)}" ${valObj.String === currentFilter ? 'selected' : ''}>${escapeHtml(valObj.String)}</option>`).join('')}
                <option value="NULL" ${currentFilter === 'NULL' ? 'selected' : ''}>N/A</option>
            </select>`;
        }
        if (headerDropdownHTML) labelContent += headerDropdownHTML;
        tableHTML += `<th style="width: ${thStyleWidth};" class="${sortableClass}" ${col.sortKey ? `data-sort-key="${col.sortKey}"` : ''} data-col-key="${key}" id="${col.id}">${labelContent}${sortIndicator}</th>`;
    }
    tableHTML += `</tr></thead><tbody>`;

    domains.forEach((domain, index) => {
        tableHTML += `<tr data-domain-id="${domain.id}">`;
        for (const key in columnConfig) {
            const col = columnConfig[key];
            if (!col.visible) continue;
            let cellContent = '';
            switch (key) {
                case 'checkbox': cellContent = `<input type="checkbox" class="domain-item-checkbox" value="${domain.id}" data-domain-name="${escapeHtmlAttribute(domain.domain_name)}">`; break;
                case 'id':
                    const { currentPage, limit, totalRecords, sortBy: currentSortBy, sortOrder: currentSortOrder } = appState.paginationState.domainsView;
                    cellContent = (currentSortBy === 'id' && currentSortOrder === 'DESC') ? (totalRecords - ((currentPage - 1) * limit) - index) : ((currentPage - 1) * limit + index + 1);
                    break;
                case 'domain_name': cellContent = escapeHtml(domain.domain_name); break;
                case 'source': cellContent = escapeHtml(domain.source?.String || ''); break;
                case 'http_status_code': cellContent = domain.http_status_code?.Valid ? domain.http_status_code.Int64 : '-'; break;
                case 'http_content_length': cellContent = domain.http_content_length?.Valid ? domain.http_content_length.Int64 : '-'; break;
                case 'http_title': cellContent = escapeHtml(domain.http_title?.String || '-'); break;
                case 'http_tech': cellContent = escapeHtml(domain.http_tech?.String || '-'); break;
                case 'http_server': cellContent = escapeHtml(domain.http_server?.String || '-'); break;
                case 'is_in_scope': cellContent = domain.is_in_scope ? 'Yes' : ''; break;
                case 'is_wildcard_scope': cellContent = domain.is_wildcard_scope ? 'Yes' : ''; break;
                case 'notes': cellContent = escapeHtml(domain.notes?.String || '-'); break;
                case 'last_httpx_result': cellContent = (domain.httpx_full_json && domain.httpx_full_json.Valid && domain.httpx_full_json.String !== "") ? new Date(domain.updated_at).toLocaleString() : '-'; break;
                case 'created_at': cellContent = new Date(domain.created_at).toLocaleString(); break;
                case 'actions':
                    const favClassForAction = domain.is_favorite ? 'favorited' : '';
                    const favoriteStarHTML = `<span class="favorite-toggle domain-favorite-toggle ${favClassForAction}" data-domain-id="${domain.id}" data-is-favorite="${domain.is_favorite}" title="Toggle Favorite" style="margin-right: 8px; vertical-align: middle; cursor:pointer;">★</span>`;
                    const viewDetailsLink = `<a href="#domain-detail?id=${domain.id}" class="action-button view-domain-details" data-id="${domain.id}" title="View Domain Details" style="margin-right: 5px; vertical-align: middle;">👁️</a>`;
                    const editButton = `<button class="action-button edit-domain" data-id="${domain.id}" title="Edit Domain" style="vertical-align: middle;">✏️</button>`;
                    const deleteButton = `<button class="action-button delete-domain" data-id="${domain.id}" title="Delete Domain" style="margin-left: 5px; vertical-align: middle;">🗑️</button>`;
                    cellContent = `${favoriteStarHTML}${viewDetailsLink}${editButton}${deleteButton}`;
                    break;
                default: cellContent = '-';
            }
            tableHTML += `<td>${cellContent}</td>`;
        }
        tableHTML += `</tr>`;
    });
    tableHTML += `</tbody></table>`;
    tableContainer.innerHTML = tableHTML;

    document.querySelectorAll('#domainsTableHead th.sortable').forEach(th => th.addEventListener('click', (event) => handleDomainSort(event.currentTarget.dataset.sortKey)));
    document.querySelectorAll('.edit-domain').forEach(btn => btn.addEventListener('click', (e) => handleEditDomain(e.currentTarget.dataset.id)));
    document.querySelectorAll('.delete-domain').forEach(btn => btn.addEventListener('click', (e) => handleDeleteDomain(e.currentTarget.dataset.id)));
    document.querySelectorAll('.domain-favorite-toggle').forEach(btn => btn.addEventListener('click', handleDomainFavoriteToggle));
    document.getElementById('selectAllDomainsCheckbox')?.addEventListener('change', handleSelectAllDomainsChange);

    document.querySelectorAll('.table-header-filter').forEach(select => {
        select.addEventListener('change', (event) => {
            const currentAppState = stateService.getState();
            const filterKey = event.target.dataset.filterKey; // Use data-filter-key
            const filterValue = event.target.value;
            let newFilterStatePartial = {};

            if (filterKey === 'http_status_code') newFilterStatePartial.filterHTTPStatusCode = filterValue;
            else if (filterKey === 'http_server') newFilterStatePartial.filterHTTPServer = filterValue;
            else if (filterKey === 'http_tech') newFilterStatePartial.filterHTTPTech = filterValue;
            
            stateService.updateState({
                paginationState: { ...currentAppState.paginationState, domainsView: { ...currentAppState.paginationState.domainsView, ...newFilterStatePartial, currentPage: 1 }}
            });
            fetchAndRenderDomainsTable(currentAppState.currentTargetId);
        });
    });
    tableService.makeTableColumnsResizable('domainsTableHead', columnConfig);
}

function renderDomainsPaginationControls(containerElement, paginationData) {
    const container = containerElement;
    if (!container) return;
    
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords, limit } = appState.paginationState.domainsView;

    let paginationHTML = '';
    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total domain(s) found.</p>` : '';
        return;
    }
    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total domains)</p>`;

    const buildHash = (page, newLimit = limit) => {
        const currentDomainsViewState = stateService.getState().paginationState.domainsView;
        const queryParams = new URLSearchParams({
            page: page, limit: newLimit, sort_by: currentDomainsViewState.sortBy, sort_order: currentDomainsViewState.sortOrder,
            domain_name_search: currentDomainsViewState.filterDomainName, source_search: currentDomainsViewState.filterSource,
        });
        if (currentDomainsViewState.filterIsInScope !== null) queryParams.set('is_in_scope', currentDomainsViewState.filterIsInScope.toString());
        if (currentDomainsViewState.filterIsFavorite === true) queryParams.set('is_favorite', 'true');
        if (currentDomainsViewState.filterHttpxScanStatus && currentDomainsViewState.filterHttpxScanStatus !== 'all') queryParams.set('httpx_scan_status', currentDomainsViewState.filterHttpxScanStatus);
        if (currentDomainsViewState.filterHTTPStatusCode) queryParams.set('filter_http_status_code', currentDomainsViewState.filterHTTPStatusCode);
        if (currentDomainsViewState.filterHTTPServer) queryParams.set('filter_http_server', currentDomainsViewState.filterHTTPServer);
        if (currentDomainsViewState.filterHTTPTech) queryParams.set('filter_http_tech', currentDomainsViewState.filterHTTPTech);
        return `#domains?${queryParams.toString()}`;
    };

    const firstButton = uiService.createButton('&laquo; First', () => { if (currentPage > 1) window.location.hash = buildHash(1); }, { disabled: currentPage <= 1, classNames: ['secondary', 'small-button'], marginRight: '5px' });
    const prevButton = uiService.createButton('&laquo; Previous', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); }, { disabled: currentPage <= 1, classNames: ['primary'], marginRight: '5px' });
    const nextButton = uiService.createButton('Next &raquo;', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); }, { disabled: currentPage >= totalPages, classNames: ['primary'], marginRight: '5px' });
    const lastButton = uiService.createButton('Last &raquo;', () => { if (currentPage < totalPages) window.location.hash = buildHash(totalPages); }, { disabled: currentPage >= totalPages, classNames: ['secondary', 'small-button'] });
    const itemsPerPageSelect = uiService.createSelect([5, 10, 15, 25, 50, 100, 200].map(val => ({ value: val, text: `${val} per page` })), limit, (e) => { window.location.hash = buildHash(1, parseInt(e.target.value, 10)); }, { id: `domainsItemsPerPageSelect_${container.id}`, marginLeft: '15px' });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    container.appendChild(firstButton); container.appendChild(prevButton); container.appendChild(nextButton); container.appendChild(lastButton); container.appendChild(itemsPerPageSelect);
}

function handleDomainSort(sortKey) {
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) {
        console.log('[DomainsView] Sort prevented due to active resize operation.');
        return;
    }
    if (!sortKey) return;

    const appState = stateService.getState();
    const currentSort = appState.paginationState.domainsView;
    let newSortOrder = (currentSort.sortBy === sortKey && currentSort.sortOrder === 'ASC') ? 'DESC' : 'ASC';
    stateService.updateState({ paginationState: { ...appState.paginationState, domainsView: { ...currentSort, sortBy: sortKey, sortOrder: newSortOrder, currentPage: 1 } } });
    fetchAndRenderDomainsTable(appState.currentTargetId);
}

function resetAndFetchDomains() {
    const appState = stateService.getState();
    const defaultDomainsState = {
        currentPage: 1, limit: 25, sortBy: 'domain_name', sortOrder: 'ASC',
        filterDomainName: '', filterSource: '', filterIsFavorite: false, filterIsInScope: null,
        filterHTTPStatusCode: '', filterHTTPServer: '', filterHTTPTech: '', filterHttpxScanStatus: 'all',
        totalPages: 0, totalRecords: 0,
    };
    stateService.updateState({ paginationState: { ...appState.paginationState, domainsView: defaultDomainsState } });
    
    document.getElementById('domainNameSearch').value = '';
    document.getElementById('domainFavoriteFilter').checked = false;
    document.getElementById('domainSourceSearch').value = '';
    document.getElementById('domainInScopeFilter').value = '';
    document.getElementById('domainHttpxScanStatusFilter').value = 'all';
    const statusCodeFilterEl = document.getElementById('colHeaderFilter-http_status_code'); if(statusCodeFilterEl) statusCodeFilterEl.value = '';
    const serverFilterEl = document.getElementById('colHeaderFilter-http_server'); if(serverFilterEl) serverFilterEl.value = '';
    const techFilterEl = document.getElementById('colHeaderFilter-http_tech'); if(techFilterEl) techFilterEl.value = '';

    document.getElementById('domainsPaginationControlsTop').innerHTML = '';
    document.getElementById('domainsPaginationControls').innerHTML = '';
    fetchAndRenderDomainsTable(appState.currentTargetId);
}

function displayAddDomainModal(targetId) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing. Cannot add domain.");
        return;
    }
    const modalContentHTML = `
        <form id="addDomainForm">
            <div class="form-group"><label for="newDomainName">Domain Name (Required):</label><input type="text" id="newDomainName" name="domain_name" required></div>
            <div class="form-group"><label for="newDomainSource">Source:</label><input type="text" id="newDomainSource" name="source"></div>
            <div class="form-group"><label for="newDomainIsInScope">Is In Scope:</label><input type="checkbox" id="newDomainIsInScope" name="is_in_scope" checked></div>
            <div class="form-group"><label for="newDomainNotes">Notes:</label><textarea id="newDomainNotes" name="notes" rows="3"></textarea></div>
            <div id="addDomainModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>`;
    uiService.showModalConfirm("Add New Domain", modalContentHTML, async () => { 
        const form = document.getElementById('addDomainForm');
        const modalMessageArea = document.getElementById('addDomainModalMessage');
        if (!form || !modalMessageArea) return false; 
        const domainName = form.elements.domain_name.value.trim();
        if (!domainName) { modalMessageArea.textContent = "Domain Name is required."; modalMessageArea.className = 'message-area error-message'; return false; }
        const domainData = {
            target_id: parseInt(targetId, 10), domain_name: domainName,
            source: { String: form.elements.source.value.trim(), Valid: !!form.elements.source.value.trim() },
            is_in_scope: form.elements.is_in_scope.checked,
            notes: { String: form.elements.notes.value.trim(), Valid: !!form.elements.notes.value.trim() }
        };
        try {
            await apiService.createDomain(domainData);
            uiService.showModalMessage("Success", `Domain "${escapeHtml(domainName)}" added successfully.`, true, 2000);
            fetchAndRenderDomainsTable(targetId); return true; 
        } catch (error) {
            modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`; modalMessageArea.className = 'message-area error-message';
            console.error("Error adding domain:", error); return false; 
        }
    }, () => {}, "Add Domain", "Cancel", true);
    document.getElementById('newDomainName')?.focus();
}

function displayDiscoverSubdomainsModal(targetId, targetName) {
    if (!targetId) { uiService.showModalMessage("Error", "Target ID is missing."); return; }
    const modalContentHTML = `
        <form id="discoverSubdomainsForm">
            <p>Run <strong>subfinder</strong> for target: <strong>${escapeHtml(targetName)}</strong> (ID: ${targetId})</p>
            <div class="form-group"><label for="subfinderDomain">Domain to Scan (Required):</label><input type="text" id="subfinderDomain" name="domain" value="" required></div>
            <div class="form-group"><label for="subfinderRecursive">Recursive Scan (-r):</label><input type="checkbox" id="subfinderRecursive" name="recursive"></div>
            <div class="form-group"><label for="subfinderSources">Sources (comma-separated, optional):</label><input type="text" id="subfinderSources" name="sources" placeholder="e.g., virustotal,certspotter"></div>
            <div id="discoverSubdomainsModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>`;
    uiService.showModalConfirm("Discover Subdomains", modalContentHTML, async () => {
        const form = document.getElementById('discoverSubdomainsForm');
        const modalMessageArea = document.getElementById('discoverSubdomainsModalMessage');
        const domain = form.elements.domain.value.trim();
        if (!domain) { modalMessageArea.textContent = "Domain to scan is required."; modalMessageArea.className = 'message-area error-message'; return false; }
        const discoveryOptions = { domain: domain, recursive: form.elements.recursive.checked, sources: form.elements.sources.value.trim() ? form.elements.sources.value.split(',').map(s => s.trim()).filter(s => s) : [] };
        try {
            await apiService.discoverSubdomains(targetId, discoveryOptions);
            startSubfinderStatusUpdates(targetId, `Subdomain discovery for "${escapeHtml(domain)}" initiated.`); return true;
        } catch (error) { modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`; modalMessageArea.className = 'message-area error-message'; return false; }
    }, () => {}, "Start Discovery", "Cancel", true);
    document.getElementById('subfinderDomain')?.focus();
}

async function handleDeleteDomain(domainId) {
    if (!domainId) { uiService.showModalMessage("Error", "Domain ID is missing."); return; }
    const domainRow = document.querySelector(`tr[data-domain-id="${domainId}"]`);
    const domainName = domainRow ? domainRow.cells[1]?.textContent : `ID ${domainId}`;
    uiService.showModalConfirm("Confirm Delete Domain", `Are you sure you want to delete domain "${escapeHtml(domainName)}"?`, async () => {
        try {
            await apiService.deleteDomain(domainId);
            uiService.showModalMessage("Success", `Domain "${escapeHtml(domainName)}" deleted.`, true, 2000);
            fetchAndRenderDomainsTable(stateService.getState().currentTargetId);
        } catch (error) { uiService.showModalMessage("Error", `Failed to delete domain: ${escapeHtml(error.message)}`); }
    });
}

function handleSelectAllDomainsChange(event) {
    document.querySelectorAll('.domain-item-checkbox').forEach(checkbox => checkbox.checked = event.target.checked);
}

async function handleSendSelectedToSubfinder(targetId) {
    const selectedCheckboxes = document.querySelectorAll('.domain-item-checkbox:checked');
    if (selectedCheckboxes.length === 0) { uiService.showModalMessage("Info", "No domains selected."); return; }
    startSubfinderStatusUpdates(targetId, `Initiating Subfinder for ${selectedCheckboxes.length} selected domain(s)...`);
    let successCount = 0, errorCount = 0;
    for (const checkbox of selectedCheckboxes) {
        try { await apiService.discoverSubdomains(targetId, { domain: checkbox.dataset.domainName }); successCount++; }
        catch (error) { errorCount++; console.error(`Error initiating Subfinder for ${checkbox.dataset.domainName}:`, error); }
    }
    console.log(`[DomainsView] Subfinder initiation: ${successCount} successful, ${errorCount} failed.`);
}

async function handleExportDomainsCSV(event) {
    if (event) event.preventDefault();
    document.getElementById('exportDomainsDropdownMenu').classList.remove('show');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId;
    const domainsViewState = appState.paginationState.domainsView;
    uiService.showModalMessage("Exporting...", "Fetching all domains for CSV export...", true);
    let allDomainsForExport = [];
    try {
        const exportParams = {
            page: 1, limit: 0, sort_by: domainsViewState.sortBy, sort_order: domainsViewState.sortOrder,
            domain_name_search: domainsViewState.filterDomainName, source_search: domainsViewState.filterSource,
            is_in_scope: domainsViewState.filterIsInScope, is_favorite: domainsViewState.filterIsFavorite,
            httpx_scan_status: domainsViewState.filterHttpxScanStatus,
            filter_http_status_code: domainsViewState.filterHTTPStatusCode,
            filter_http_server: domainsViewState.filterHTTPServer,
            filter_http_tech: domainsViewState.filterHTTPTech,
        };
        const response = await apiService.getDomains(targetId, exportParams);
        allDomainsForExport = response.records || [];
    } catch (error) { uiService.showModalMessage("Export Error", `Failed to fetch domains: ${escapeHtml(error.message)}`); return; }

    const columnConfig = appState.paginationState.domainsTableLayout;
    const headers = Object.keys(columnConfig).filter(key => columnConfig[key].visible && key !== 'actions' && key !== 'checkbox').map(key => columnConfig[key].label);
    const rows = allDomainsForExport.map(domain => Object.keys(columnConfig).filter(key => columnConfig[key].visible && key !== 'actions' && key !== 'checkbox').map(key => {
        switch (key) {
            case 'id': return allDomainsForExport.indexOf(domain) + 1;
            case 'domain_name': return domain.domain_name;
            case 'source': return domain.source?.String || '';
            case 'http_status_code': return domain.http_status_code?.Valid ? domain.http_status_code.Int64 : '';
            case 'http_content_length': return domain.http_content_length?.Valid ? domain.http_content_length.Int64 : '';
            case 'http_title': return domain.http_title?.String || '';
            case 'http_tech': return domain.http_tech?.String || '';
            case 'http_server': return domain.http_server?.String || '';
            case 'is_favorite': return domain.is_favorite ? 'Yes' : 'No';
            case 'is_in_scope': return domain.is_in_scope ? 'Yes' : 'No';
            case 'is_wildcard_scope': return domain.is_wildcard_scope ? 'Yes' : 'No';
            case 'notes': return domain.notes?.String || '';
            case 'created_at': return new Date(domain.created_at).toLocaleString();
            case 'last_httpx_result': return (domain.httpx_full_json && domain.httpx_full_json.Valid && domain.httpx_full_json.String !== "") ? new Date(domain.updated_at).toLocaleString() : '';
            default: return '';
        }
    }));
    let csvContent = headers.join(",") + "\n" + rows.map(rowArray => rowArray.map(field => `"${String(field).replace(/"/g, '""')}"`).join(",")).join("\n");
    downloadCSV(csvContent, `domains_target_${targetId}.csv`);
    uiService.showModalMessage("Success", "Domains exported to CSV.", true, 2000);
}

async function handleExportDomainsTXT(event) {
    if (event) event.preventDefault();
    document.getElementById('exportDomainsDropdownMenu').classList.remove('show');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId;
    const domainsViewState = appState.paginationState.domainsView;
    uiService.showModalMessage("Exporting...", "Fetching all domains for TXT export...", true);
    let allDomainsForExport = [];
    try {
        const exportParams = {
            page: 1, limit: 0, sort_by: domainsViewState.sortBy, sort_order: domainsViewState.sortOrder,
            domain_name_search: domainsViewState.filterDomainName, source_search: domainsViewState.filterSource,
            is_in_scope: domainsViewState.filterIsInScope, is_favorite: domainsViewState.filterIsFavorite,
            httpx_scan_status: domainsViewState.filterHttpxScanStatus,
            filter_http_status_code: domainsViewState.filterHTTPStatusCode,
            filter_http_server: domainsViewState.filterHTTPServer,
            filter_http_tech: domainsViewState.filterHTTPTech,
        };
        const response = await apiService.getDomains(targetId, exportParams);
        allDomainsForExport = response.records || [];
    } catch (error) { uiService.showModalMessage("Export Error", `Failed to fetch domains: ${escapeHtml(error.message)}`); return; }
    const txtContent = allDomainsForExport.map(domain => domain.domain_name).join("\n");
    downloadTXT(txtContent, `domains_target_${targetId}.txt`);
    uiService.showModalMessage("Success", "Domains exported to TXT.", true, 2000);
}

async function handleEditDomain(domainId) {
    if (!domainId) { uiService.showModalMessage("Error", "Domain ID missing."); return; }
    const domainData = currentDomainsData.find(d => d.id == domainId);
    if (!domainData) { uiService.showModalMessage("Error", "Could not retrieve domain data."); return; }
    const modalContentHTML = `
        <form id="editDomainForm">
            <p><strong>Domain:</strong> ${escapeHtml(domainData.domain_name)} (ID: ${domainId})</p>
            <div class="form-group"><label for="editDomainSource">Source:</label><input type="text" id="editDomainSource" name="source" value="${escapeHtmlAttribute(domainData.source?.String || '')}"></div>
            <div class="form-group"><label for="editDomainIsInScope">Is In Scope:</label><input type="checkbox" id="editDomainIsInScope" name="is_in_scope" ${domainData.is_in_scope ? 'checked' : ''}></div>
            <div class="form-group"><label for="editDomainNotes">Notes:</label><textarea id="editDomainNotes" name="notes" rows="3">${escapeHtml(domainData.notes?.String || '')}</textarea></div>
            <div id="editDomainModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>`;
    uiService.showModalConfirm(`Edit Domain: ${escapeHtml(domainData.domain_name)}`, modalContentHTML, async () => {
        const form = document.getElementById('editDomainForm');
        const modalMessageArea = document.getElementById('editDomainModalMessage');
        if (!form || !modalMessageArea) return false;
        const domainUpdateData = {
            source: { String: form.elements.source.value.trim(), Valid: !!form.elements.source.value.trim() },
            is_in_scope: form.elements.is_in_scope.checked,
            notes: { String: form.elements.notes.value.trim(), Valid: !!form.elements.notes.value.trim() }
        };
        try {
            await apiService.updateDomain(domainId, domainUpdateData);
            uiService.showModalMessage("Success", `Domain "${escapeHtml(domainData.domain_name)}" updated.`, true, 2000);
            fetchAndRenderDomainsTable(stateService.getState().currentTargetId); return true;
        } catch (error) { modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`; modalMessageArea.className = 'message-area error-message'; console.error("Error updating domain:", error); return false; }
    }, () => {}, "Save Changes", "Cancel", true);
    document.getElementById('editDomainSource')?.focus();
}

async function handleImportInScopeDomains(targetId, targetName) {
    if (!targetId) { uiService.showModalMessage("Error", "Target ID missing."); return; }
    stopSubfinderStatusUpdates();
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea) setMessageWithCloseButton(messageArea, `Importing in-scope domains for "${escapeHtml(targetName)}"...`, 'info-message');
    try {
        const result = await apiService.importInScopeDomains(targetId);
        if (messageArea) setMessageWithCloseButton(messageArea, `Imported: ${result.imported_count || 0}, Skipped: ${result.skipped_count || 0}. ${escapeHtml(result.message || '')}`, 'success-message', 3000);
        fetchAndRenderDomainsTable(targetId);
    } catch (error) { if (messageArea) setMessageWithCloseButton(messageArea, `Error importing: ${escapeHtml(error.message)}`, 'error-message'); stopSubfinderStatusUpdates(); console.error("Error importing domains:", error); }
}

async function handleDeleteAllDomains(targetId, targetName) {
    if (!targetId) { uiService.showModalMessage("Error", "Target ID missing."); return; }
    uiService.showModalConfirm("Confirm Delete All Domains", `Delete ALL domains for target "${escapeHtml(targetName)}"?`, async () => {
        try {
            const result = await apiService.deleteAllDomainsForTarget(targetId);
            uiService.showModalMessage("Success", `All domains for "${escapeHtml(targetName)}" deleted. Count: ${result.deleted_count || 0}.`, true, 3000);
            fetchAndRenderDomainsTable(targetId);
        } catch (error) { uiService.showModalMessage("Error", `Failed to delete domains: ${escapeHtml(error.message)}`); console.error("Error deleting domains:", error); }
    });
}

async function handleSendSelectedToHttpx(targetId) {
    const selectedCheckboxes = document.querySelectorAll('.domain-item-checkbox:checked');
    if (selectedCheckboxes.length === 0) { uiService.showModalMessage("Info", "No domains selected."); return; }
    const domainIds = Array.from(selectedCheckboxes).map(cb => parseInt(cb.value, 10));
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea) {
        try { startHttpxStatusUpdates(targetId, `Sending ${domainIds.length} selected domain(s) for httpx scan...`); await apiService.runHttpxForDomains(targetId, domainIds); }
        catch (error) { if (messageArea) setMessageWithCloseButton(messageArea, `Error initiating httpx: ${escapeHtml(error.message)}`, 'error-message'); console.error("Error initiating httpx:", error); }
    }
}

async function handleDomainFavoriteToggle(event) {
    const button = event.currentTarget;
    const domainId = button.dataset.domainId;
    const newFavoriteState = !(button.dataset.isFavorite === 'true');
    try {
        await apiService.setDomainFavorite(domainId, newFavoriteState);
        button.innerHTML = '★'; button.classList.toggle('favorited', newFavoriteState); button.dataset.isFavorite = newFavoriteState.toString();
    } catch (favError) { console.error("Error toggling favorite:", favError); uiService.showModalMessage("Error", `Failed to update favorite: ${favError.message}`); }
}

async function handleFavoriteAllFiltered(targetId) {
    if (!targetId) { uiService.showModalMessage("Error", "Target ID missing."); return; }
    const appState = stateService.getState();
    const filters = appState.paginationState.domainsView;
    const filterDescription = `filters (Domain: '${filters.filterDomainName}', Source: '${filters.filterSource}', In Scope: ${filters.filterIsInScope === null ? 'Any' : filters.filterIsInScope}, HTTPX: ${filters.filterHttpxScanStatus})`;
    uiService.showModalConfirm("Confirm Favorite All", `Mark ALL domains matching ${filterDescription} as favorite?`, async () => {
        uiService.showModalMessage("Processing...", "Favoriting filtered domains...", true);
        try {
            const apiFilters = { domain_name_search: filters.filterDomainName, source_search: filters.filterSource, is_in_scope: filters.filterIsInScope, httpx_scan_status: filters.filterHttpxScanStatus };
            const result = await apiService.favoriteAllFilteredDomains(targetId, apiFilters);
            uiService.showModalMessage("Success", `${result.updated_count || 0} domain(s) marked as favorite.`, true, 3000);
            fetchAndRenderDomainsTable(targetId);
        } catch (error) { uiService.showModalMessage("Error", `Failed to favorite domains: ${escapeHtml(error.message)}`); }
    });
}

async function handleSendAllFilteredToHttpx(targetId) {
    if (!targetId) { uiService.showModalMessage("Error", "Target ID missing."); return; }
    const appState = stateService.getState();
    const currentFilters = appState.paginationState.domainsView;
    const filtersForAPI = {
        domain_name_search: currentFilters.filterDomainName,
        source_search: currentFilters.filterSource,
        is_in_scope: currentFilters.filterIsInScope,
        is_favorite: currentFilters.filterIsFavorite,
        httpx_scan_status: currentFilters.filterHttpxScanStatus,
        filter_http_status_code: currentFilters.filterHTTPStatusCode,
        filter_http_server: currentFilters.filterHTTPServer,
        filter_http_tech: currentFilters.filterHTTPTech,
    };
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea) {
        try { startHttpxStatusUpdates(targetId, `Initiating httpx scan for ALL filtered domains...`); await apiService.runHttpxForAllFilteredDomains(targetId, filtersForAPI); }
        catch (error) { if (messageArea) setMessageWithCloseButton(messageArea, `Error initiating httpx: ${escapeHtml(error.message)}`, 'error-message'); console.error("Error initiating httpx:", error); }
    }
}

async function checkAndResumeActiveScans(targetId) {
    if (!targetId) return;
    console.log(`[DomainsView] checkAndResumeActiveScans for targetId: ${targetId}`);
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    try {
        const httpxStatus = await apiService.getHttpxStatus(targetId);
        console.log(`[DomainsView] Initial httpxStatus for target ${targetId}:`, httpxStatus);
        if (httpxStatus && httpxStatus.is_running) startHttpxStatusUpdates(targetId, httpxStatus.message);
        else if (httpxStatus && !httpxStatus.is_running && httpxStatus.message && httpxStatus.message !== "No active httpx scan for this target or status not initialized.") updateHttpxStatusDisplay(httpxStatus);
    } catch (error) { console.error("[DomainsView] Error checking httpx status:", error); if (messageArea) setMessageWithCloseButton(messageArea, `Could not fetch httpx status: ${error.message}`, 'error-message', 5000); }
    try {
        const subfinderStatus = await apiService.getSubfinderStatus(targetId);
        console.log(`[DomainsView] Initial subfinderStatus for target ${targetId}:`, subfinderStatus);
        if (subfinderStatus && subfinderStatus.is_running) startSubfinderStatusUpdates(targetId, subfinderStatus.message);
        else if (subfinderStatus && !subfinderStatus.is_running && subfinderStatus.message) updateSubfinderStatusDisplay(subfinderStatus);
    } catch (error) { console.error("[DomainsView] Error checking subfinder status:", error); if (messageArea) setMessageWithCloseButton(messageArea, `Could not fetch subfinder status: ${error.message}`, 'error-message', 5000); }
}

async function handleStopHttpxScan(targetId) {
    if (!targetId) return;
    const stopBtn = document.getElementById('stopHttpxScanBtn');
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (stopBtn) stopBtn.disabled = true;
    if (messageArea) setMessageWithCloseButton(messageArea, 'Attempting to stop httpx scan...', 'info-message');
    try { await apiService.stopHttpxScan(targetId); }
    catch (error) { console.error("Error stopping httpx scan:", error); if (messageArea) setMessageWithCloseButton(messageArea, `Error stopping scan: ${escapeHtml(error.message)}`, 'error-message'); if (stopBtn) stopBtn.disabled = false; }
}
