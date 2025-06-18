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
    // Optionally, clear the message or set a final one after a short delay
    // setTimeout(() => {
    //     const messageArea = document.getElementById('discoverSubdomainsMessage');
    //     if (messageArea && !subfinderStatusIntervalId) { // Check again in case it restarted
    //         messageArea.textContent = 'Subfinder status updates inactive.';
    //         messageArea.className = 'message-area info-message';
    //     }
    // }, 6000); // Slightly longer than the poll interval
}

/**
 * Helper function to set message content and class, and add a close button.
 * @param {HTMLElement} messageAreaElement - The div element to display the message in.
 * @param {string} text - The message text.
 * @param {string} className - The class to apply (e.g., 'success-message', 'error-message').
 * @param {number} autoHideTimeout - Milliseconds to auto-hide the message (0 for no auto-hide).
 */
function setMessageWithCloseButton(messageAreaElement, text, className, autoHideTimeout = 0) {
    if (!messageAreaElement) return;

    messageAreaElement.innerHTML = ''; // Clear previous content
    messageAreaElement.className = 'message-area'; // Reset class first

    const textSpan = document.createElement('span');
    textSpan.textContent = text; // Text content is set directly

    const closeButton = document.createElement('span');
    closeButton.innerHTML = '&times;'; // 'X' character
    closeButton.className = 'message-close-button'; // Apply CSS class for styling (see notes below)
    closeButton.title = 'Close message';
    
    // Clear any existing auto-hide timeout for this specific message area
    if (messageAreaElement.autoHideTimeoutId) {
        clearTimeout(messageAreaElement.autoHideTimeoutId);
        messageAreaElement.autoHideTimeoutId = null;
    }

    const clearMessage = () => {
        messageAreaElement.innerHTML = '';
        messageAreaElement.className = 'message-area'; 
        messageAreaElement.style.display = 'none'; 
        // Ensure the stored timeoutId on the element is also cleared if it was this one
        if (messageAreaElement.autoHideTimeoutId) { // Check if it's still set (it should be if this timeout fired)
            // No need to call clearTimeout again as it has already fired or been cleared by a new message, just nullify our reference
            messageAreaElement.autoHideTimeoutId = null;
        }
    };

    closeButton.onclick = clearMessage;

    messageAreaElement.appendChild(textSpan);   
    messageAreaElement.appendChild(closeButton); 
    messageAreaElement.classList.add(className); 
    messageAreaElement.style.display = 'block'; // Ensure it's visible
    if (autoHideTimeout > 0) {
        messageAreaElement.autoHideTimeoutId = setTimeout(clearMessage, autoHideTimeout);
    }
}

/**
 * Updates the subfinder status message display.
 * @param {Object} statusData - The status data from the API.
 *                              Expected: { is_running: boolean, message: string, completed_tasks_summary?: string }
 */
function updateSubfinderStatusDisplay(statusData) {
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (!messageArea) return;

    const appState = stateService.getState(); // Get current target ID for potential refresh
    const currentTargetId = appState.currentTargetId;

    if (statusData.is_running) {
        // Display the current message from backend, indicate polling continues
        setMessageWithCloseButton(messageArea, `${escapeHtml(statusData.message || "Processing...")} (Status refreshing...)`, 'success-message');
    } else {
        let finalMessage = statusData.message || "Subfinder task processing complete or inactive.";
        if (statusData.completed_tasks_summary) {
            finalMessage += ` ${escapeHtml(statusData.completed_tasks_summary)}`;
        }
        setMessageWithCloseButton(messageArea, finalMessage, 'success-message', 7000); // Auto-hide after 7 seconds
        stopSubfinderStatusUpdates(); // Stop polling if not running
        if (currentTargetId) fetchAndRenderDomainsTable(currentTargetId); // Refresh the table
    }
}

/**
 * Updates the httpx status message display.
 * @param {Object} statusData - The status data from the API.
 */
function updateHttpxStatusDisplay(statusData) {
    console.log("[DomainsView] updateHttpxStatusDisplay called with:", JSON.parse(JSON.stringify(statusData)));
    const messageArea = document.getElementById('discoverSubdomainsMessage'); // Reusing for now
    if (!messageArea) return;

    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    let displayMessage = statusData.message || "Querying httpx status...";
    if (statusData.is_running) {
        displayMessage += ` (Processed: ${statusData.domains_processed}/${statusData.domains_total})`;
    }

    const notInitializedMessage = "No active httpx scan for this target or status not initialized.";
    // Only set a message if it's not the specific "not initialized" message when not running,
    // or if it's any other type of message (running, completed, error).
    if (!(!statusData.is_running && statusData.message === notInitializedMessage)) {
        setMessageWithCloseButton(messageArea, escapeHtml(displayMessage), statusData.is_running ? 'info-message' : 'success-message', statusData.is_running ? 0 : 7000);
    } else {
        console.log("[DomainsView] Suppressing 'No active httpx scan...' message display. Polling continues.");
    }

    if (!statusData.is_running) {
        // Only stop polling if it's a definitive "not running" state (e.g., completed, failed),
        // not the initial "not initialized" state.
        const notInitializedMessage = "No active httpx scan for this target or status not initialized.";
        if (statusData.message !== notInitializedMessage) {
            console.log("[DomainsView] Httpx scan is definitively not running (e.g. completed/failed). Stopping polling.");
            if (statusData.last_error) {
                // Display error more prominently or for longer if it's a real error
                setMessageWithCloseButton(messageArea, `Httpx Error: ${escapeHtml(statusData.last_error)}`, 'error-message', 10000);
            }
            if (currentTargetId) fetchAndRenderDomainsTable(currentTargetId); // Refresh table on completion/error
        } else {
            console.log("[DomainsView] Httpx scan reported as not running, but message indicates it might be the initial 'not initialized' state. Polling will continue.");
            // The message was suppressed above. Polling continues.
            // The next poll should ideally get an updated status like "Initializing..." or "Starting..."
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
        stopSubfinderStatusUpdates(); // Stop polling on error
    }
}

/**
 * Starts polling for httpx status updates.
 * @param {number|string} targetId - The ID of the target.
 * @param {string} initialMessage - An initial message to display.
 */
function startHttpxStatusUpdates(targetId, initialMessage) {
    stopHttpxStatusUpdates(); // Clear any existing interval

    console.log("[DomainsView] startHttpxStatusUpdates called for target", targetId, "with initial message:", initialMessage);
    const messageArea = document.getElementById('discoverSubdomainsMessage'); // Reusing for now
    if (messageArea && initialMessage) {
        setMessageWithCloseButton(messageArea, initialMessage, 'info-message');
    }

    if (!targetId) return;

    fetchAndDisplayHttpxStatus(targetId); // Fetch immediately once to get initial status
    httpxStatusIntervalId = setInterval(() => fetchAndDisplayHttpxStatus(targetId), 5000); // Poll every 5 seconds
    console.log("[DomainsView] Httpx status polling started for target:", targetId);
}

/**
 * Starts polling for subfinder status updates.
 * @param {number|string} targetId - The ID of the target.
 * @param {string} initialMessage - An initial message to display.
 */
function startSubfinderStatusUpdates(targetId, initialMessage) {
    stopSubfinderStatusUpdates(); // Clear any existing interval

    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea && initialMessage) {
        setMessageWithCloseButton(messageArea, initialMessage, 'success-message');
    }

    if (!targetId) return;

    // fetchAndDisplaySubfinderStatus(targetId); // REMOVED: Immediate initial fetch
    subfinderStatusIntervalId = setInterval(() => fetchAndDisplaySubfinderStatus(targetId), 5000); // Poll every 5 seconds
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

    // Stop status polling when the view is reloaded/changed
    stopSubfinderStatusUpdates();
    stopHttpxStatusUpdates(); // Also stop httpx polling

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

    // Ensure pagination state for domainsView exists
    if (!appState.paginationState.domainsView) {
        console.warn("[DomainsView] paginationState.domainsView not found, initializing with defaults.");
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: {
                    currentPage: 1,
                    limit: 25,
                    sortBy: 'domain_name',
                    sortOrder: 'ASC',
                    filterDomainName: '',
                    filterSource: '',
                    filterIsFavorite: false, 
                    filterIsInScope: null, // null means 'all'
                    totalPages: 0,
                    totalRecords: 0,
                }
            }
        });
    }
     // Ensure table layout state for domainsTable exists
    if (!appState.paginationState.domainsTableLayout) {
        console.warn("[DomainsView] paginationState.domainsTableLayout not found, initializing with defaults from stateService.js.");
        // The actual defaults are in stateService.js, this is just a fallback message.
        // The rendering logic will use what's in stateService.js.
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
            <!-- The old checkbox for Hide HTTPX Scanned is removed -->
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

    // Populate filter controls with current state values from the router/URL params
    // This ensures that when fetchAndRenderDomainsTable reads from these UI elements,
    // they will have the correct values reflecting the current navigation state.
    const currentDomainsFilters = appState.paginationState.domainsView;
    const domainNameSearchInput = document.getElementById('domainNameSearch');
    if (domainNameSearchInput) domainNameSearchInput.value = currentDomainsFilters.filterDomainName;
    const domainSourceSearchInput = document.getElementById('domainSourceSearch');
    if (domainSourceSearchInput) domainSourceSearchInput.value = currentDomainsFilters.filterSource;
    const domainInScopeFilterInput = document.getElementById('domainInScopeFilter');
    if (domainInScopeFilterInput) domainInScopeFilterInput.value = currentDomainsFilters.filterIsInScope === null ? "" : String(currentDomainsFilters.filterIsInScope);
    const domainFavoriteFilterInput = document.getElementById('domainFavoriteFilter');
    if (domainFavoriteFilterInput) domainFavoriteFilterInput.checked = currentDomainsFilters.filterIsFavorite;
    const domainHttpxScanStatusFilterInput = document.getElementById('domainHttpxScanStatusFilter'); // For the existing dropdown
    if (domainHttpxScanStatusFilterInput) domainHttpxScanStatusFilterInput.value = currentDomainsFilters.filterHttpxScanStatus || 'all';
    // New: Initialize new dropdowns if they exist (they will be created by renderDomainsTable)
    const statusCodeFilterEl = document.getElementById('colHeaderFilter-http_status_code');
    if (statusCodeFilterEl) statusCodeFilterEl.value = currentDomainsFilters.filterHTTPStatusCode || '';
    const serverFilterEl = document.getElementById('colHeaderFilter-http_server');
    if (serverFilterEl) serverFilterEl.value = currentDomainsFilters.filterHTTPServer || '';
    const techFilterEl = document.getElementById('colHeaderFilter-http_tech');
    if (techFilterEl) techFilterEl.value = currentDomainsFilters.filterHTTPTech || '';

    // Add event listeners for filter controls
    document.getElementById('resetDomainFiltersBtn')?.addEventListener('click', resetAndFetchDomains);
    document.getElementById('domainNameSearch')?.addEventListener('input', debounce(() => { 
        const appState = stateService.getState(); // Get fresh state
        stateService.updateState({ paginationState: { ...appState.paginationState, domainsView: { ...appState.paginationState.domainsView, currentPage: 1}}}); 
        fetchAndRenderDomainsTable(currentTargetId); 
    }, 500));
    document.getElementById('domainSourceSearch')?.addEventListener('input', debounce(() => { 
        const appState = stateService.getState(); // Get fresh state
        stateService.updateState({ paginationState: { ...appState.paginationState, domainsView: { ...appState.paginationState.domainsView, currentPage: 1}}}); 
        fetchAndRenderDomainsTable(currentTargetId); 
    }, 500));
    document.getElementById('domainInScopeFilter')?.addEventListener('change', () => fetchAndRenderDomainsTable(currentTargetId));
    document.getElementById('domainFavoriteFilter')?.addEventListener('change', () => fetchAndRenderDomainsTable(currentTargetId));
    document.getElementById('domainHttpxScanStatusFilter')?.addEventListener('change', () => fetchAndRenderDomainsTable(currentTargetId));
    // New: Event listeners for new header dropdowns will be attached in renderDomainsTable
    // because those elements are dynamically created.


    
    document.getElementById('addDomainBtn')?.addEventListener('click', () => displayAddDomainModal(currentTargetId));
    document.getElementById('saveDomainsLayoutBtn')?.addEventListener('click', () => {
        tableService.saveCurrentTableLayout('domainsTable', 'domainsTableHead');
    });

    // Event listeners for items moved into "More Actions" dropdown
    document.getElementById('importInScopeDomainsBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleImportInScopeDomains(currentTargetId, currentTargetName);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('deleteAllDomainsBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleDeleteAllDomains(currentTargetId, currentTargetName);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('sendSelectedToSubfinderBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleSendSelectedToSubfinder(currentTargetId);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('sendSelectedToHttpxBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleSendSelectedToHttpx(currentTargetId);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('discoverSubdomainsBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        displayDiscoverSubdomainsModal(currentTargetId, currentTargetName);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('favoriteAllFilteredBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleFavoriteAllFiltered(currentTargetId);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('sendAllFilteredToHttpxBtnLink')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleSendAllFilteredToHttpx(currentTargetId);
        document.getElementById('moreDomainActionsDropdownMenu').classList.remove('show');
    });
    document.getElementById('refreshDomainsTableBtn')?.addEventListener('click', () => fetchAndRenderDomainsTable(currentTargetId));

    // Export dropdown logic
    const exportDropdownBtn = document.getElementById('exportDomainsDropdownBtn');
    const exportDropdownMenu = document.getElementById('exportDomainsDropdownMenu');
    if (exportDropdownBtn && exportDropdownMenu) {
        exportDropdownBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            exportDropdownMenu.classList.toggle('show');
        });
        document.getElementById('exportDomainsCSV')?.addEventListener('click', handleExportDomainsCSV);
        document.getElementById('exportDomainsTXT')?.addEventListener('click', handleExportDomainsTXT);
    }

    // "More Actions" dropdown logic
    const moreActionsDropdownBtn = document.getElementById('moreDomainActionsDropdownBtn');
    const moreActionsDropdownMenu = document.getElementById('moreDomainActionsDropdownMenu');
    if (moreActionsDropdownBtn && moreActionsDropdownMenu) {
        moreActionsDropdownBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            moreActionsDropdownMenu.classList.toggle('show');
        });
    }
    
    // Shared logic to close dropdowns if clicked outside
    document.addEventListener('click', (e) => {
        const isClickInsideMoreActions = moreActionsDropdownBtn?.contains(e.target) || moreActionsDropdownMenu?.contains(e.target);
        const isClickInsideExport = exportDropdownBtn?.contains(e.target) || exportDropdownMenu?.contains(e.target);

        if (!isClickInsideMoreActions && moreActionsDropdownMenu) {
            moreActionsDropdownMenu.classList.remove('show');
        }
        if (!isClickInsideExport && exportDropdownMenu) {
            exportDropdownMenu.classList.remove('show');
        }
    });

    await fetchAndRenderDomainsTable(currentTargetId);
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
    let domainsViewState = { ...appState.paginationState.domainsView }; // Create a mutable copy

    // Get filter values from UI elements if they exist, otherwise use state
    const domainNameSearchEl = document.getElementById('domainNameSearch');
    const sourceSearchEl = document.getElementById('domainSourceSearch');
    const inScopeFilterEl = document.getElementById('domainInScopeFilter');
    const favoriteFilterEl = document.getElementById('domainFavoriteFilter');
    const httpxScanStatusEl = document.getElementById('domainHttpxScanStatusFilter'); // Existing dropdown
    // New: Read from new header dropdowns
    const statusCodeFilterEl = document.getElementById('colHeaderFilter-http_status_code');
    const serverFilterEl = document.getElementById('colHeaderFilter-http_server');
    const techFilterEl = document.getElementById('colHeaderFilter-http_tech');
    
    // Update domainsViewState with current UI filter values
    domainsViewState.filterDomainName = domainNameSearchEl ? domainNameSearchEl.value : domainsViewState.filterDomainName;
    domainsViewState.filterSource = sourceSearchEl ? sourceSearchEl.value : domainsViewState.filterSource;
    domainsViewState.filterIsInScope = (inScopeFilterEl && inScopeFilterEl.value !== "") ? (inScopeFilterEl.value === 'true') : null;
    domainsViewState.filterIsFavorite = favoriteFilterEl ? favoriteFilterEl.checked : domainsViewState.filterIsFavorite; // Update state here
    domainsViewState.filterHttpxScanStatus = httpxScanStatusEl ? httpxScanStatusEl.value : (domainsViewState.filterHttpxScanStatus || 'all');
    domainsViewState.filterHTTPStatusCode = statusCodeFilterEl ? statusCodeFilterEl.value : (domainsViewState.filterHTTPStatusCode || '');
    domainsViewState.filterHTTPServer = serverFilterEl ? serverFilterEl.value : (domainsViewState.filterHTTPServer || '');
    domainsViewState.filterHTTPTech = techFilterEl ? techFilterEl.value : (domainsViewState.filterHTTPTech || '');


    const params = {
        // ... existing params
        page: domainsViewState.currentPage,
        limit: domainsViewState.limit,
        sort_by: domainsViewState.sortBy,
        sort_order: domainsViewState.sortOrder,
        domain_name_search: domainsViewState.filterDomainName,
        source_search: domainsViewState.filterSource,
    };
    if (domainsViewState.filterIsInScope !== null) {
        params.is_in_scope = domainsViewState.filterIsInScope.toString();
    }
    if (domainsViewState.filterIsFavorite === true) { // Only add if true
        params.is_favorite = 'true';
    }
    if (domainsViewState.filterHttpxScanStatus && domainsViewState.filterHttpxScanStatus !== 'all') {
        params.httpx_scan_status = domainsViewState.filterHttpxScanStatus;
    }
    // New: Add new filters to API params
    if (domainsViewState.filterHTTPStatusCode) {
        params.filter_http_status_code = domainsViewState.filterHTTPStatusCode;
    }
    if (domainsViewState.filterHTTPServer) {
        params.filter_http_server = domainsViewState.filterHTTPServer;
    }
    if (domainsViewState.filterHTTPTech) {
        params.filter_http_tech = domainsViewState.filterHTTPTech;
    }

    try {
        const response = await apiService.getDomains(targetId, params);
        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                domainsView: {
                    ...domainsViewState, // Use the locally updated domainsViewState which now includes the correct filterIsFavorite
                    currentPage: response.page,
                    limit: response.limit,
                    totalPages: response.total_pages,
                    totalRecords: response.total_records,
                    distinctHttpStatusCodes: response.distinct_http_status_codes || [], // Store distinct values
                    distinctHttpServers: response.distinct_http_servers || [],
                    distinctHttpTechs: response.distinct_http_techs || [],
                    sortBy: response.sort_by || domainsViewState.sortBy,
                    sortOrder: response.sort_order || domainsViewState.sortOrder
                }
            }
        });

        currentDomainsData = response.records || []; // Store for export
        renderDomainsTable(response.records || [], response); // Pass full response for distinct values
        
        if (topPaginationControlsDiv) renderDomainsPaginationControls(topPaginationControlsDiv, response);
        if (bottomPaginationControlsDiv) renderDomainsPaginationControls(bottomPaginationControlsDiv, response);

    } catch (error) {
        console.error("Error fetching domains:", error);
        tableContainer.innerHTML = `<p class="error-message">Error loading domains: ${escapeHtml(error.message)}</p>`;
        if (topPaginationControlsDiv) topPaginationControlsDiv.innerHTML = '';
        if (bottomPaginationControlsDiv) bottomPaginationControlsDiv.innerHTML = '';
    }
}

function renderDomainsTable(domains, apiResponseData = {}) { // Added apiResponseData
    const tableContainer = document.getElementById('domainsTableContainer');
    if (!tableContainer) {
        console.error("domainsTableContainer not found in renderDomainsTable");
        return;
    }

    let appState = stateService.getState(); // Use let for potential re-fetch if needed for distinct values
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
        created_at: { default: 'auto', id: 'col-domain-created', visible: true, label: 'Created At', sortKey: 'created_at' }, // Ensure this is after last_httpx_result if 'auto' is used for width
        actions: { default: '150px', id: 'col-domain-actions', visible: true, label: 'Actions', nonResizable: true, nonHideable: true }
    };
    const globalTableLayouts = appState.globalTableLayouts || {};
    const tableKey = 'domainsTable';
    const savedTableWidths = globalTableLayouts[tableKey]?.columns || {};

    // Get distinct values from the API response or state
    const distinctHttpStatusCodes = apiResponseData.distinct_http_status_codes || appState.paginationState.domainsView.distinctHttpStatusCodes || [];
    const distinctHttpServers = apiResponseData.distinct_http_servers || appState.paginationState.domainsView.distinctHttpServers || [];
    const distinctHttpTechs = apiResponseData.distinct_http_techs || appState.paginationState.domainsView.distinctHttpTechs || [];


    if (!domains || domains.length === 0) {
        tableContainer.innerHTML = "<p>No domains found for this target with the current filters.</p>";
        const topPaginationForReset = document.getElementById('domainsPaginationControlsTop');
        if (topPaginationForReset) topPaginationForReset.innerHTML = '';
        const bottomPaginationForReset = document.getElementById('domainsPaginationControls');
        if (bottomPaginationForReset) bottomPaginationForReset.innerHTML = '';
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

        // Add dropdowns for specific columns
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
        if (headerDropdownHTML) {
            labelContent += headerDropdownHTML;
        }
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
                case 'checkbox':
                    cellContent = `<input type="checkbox" class="domain-item-checkbox" value="${domain.id}" data-domain-name="${escapeHtmlAttribute(domain.domain_name)}">`;
                    break;
                case 'id':
                    const { currentPage, limit, totalRecords, sortBy: currentSortBy, sortOrder: currentSortOrder } = appState.paginationState.domainsView;
                    if (currentSortBy === 'id' && currentSortOrder === 'DESC') {
                        cellContent = totalRecords - ((currentPage - 1) * limit) - index;
                    } else {
                        cellContent = (currentPage - 1) * limit + index + 1;
                    }
                    break;
                case 'domain_name':       cellContent = escapeHtml(domain.domain_name); break;
                case 'source':            cellContent = escapeHtml(domain.source?.String || ''); break;
                case 'http_status_code':  cellContent = domain.http_status_code?.Valid ? domain.http_status_code.Int64 : '-'; break;
                case 'http_content_length': cellContent = domain.http_content_length?.Valid ? domain.http_content_length.Int64 : '-'; break;
                case 'http_title':        cellContent = escapeHtml(domain.http_title?.String || '-'); break;
                case 'http_tech':         cellContent = escapeHtml(domain.http_tech?.String || '-'); break;
                case 'http_server':       cellContent = escapeHtml(domain.http_server?.String || '-'); break;
                case 'is_in_scope':       cellContent = domain.is_in_scope ? 'Yes' : ''; break;
                case 'is_wildcard_scope': cellContent = domain.is_wildcard_scope ? 'Yes' : ''; break;
                case 'notes':             cellContent = escapeHtml(domain.notes?.String || '-'); break;
                case 'last_httpx_result':
                    cellContent = (domain.httpx_full_json && domain.httpx_full_json.Valid && domain.httpx_full_json.String !== "") ? new Date(domain.updated_at).toLocaleString() : '-';
                    break;
                case 'created_at':        cellContent = new Date(domain.created_at).toLocaleString(); break;
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

    document.querySelectorAll('#domainsTableHead th.sortable').forEach(th => {
        th.addEventListener('click', (event) => handleDomainSort(event.currentTarget.dataset.sortKey));
    });
    document.querySelectorAll('.edit-domain').forEach(btn => btn.addEventListener('click', (e) => handleEditDomain(e.currentTarget.dataset.id)));
    document.querySelectorAll('.delete-domain').forEach(btn => btn.addEventListener('click', (e) => handleDeleteDomain(e.currentTarget.dataset.id)));
    document.querySelectorAll('.domain-favorite-toggle').forEach(btn => btn.addEventListener('click', handleDomainFavoriteToggle));

    const selectAllCheckbox = document.getElementById('selectAllDomainsCheckbox');
    if (selectAllCheckbox) {
        selectAllCheckbox.removeEventListener('change', handleSelectAllDomainsChange);
        selectAllCheckbox.addEventListener('change', handleSelectAllDomainsChange);
    }

    // Add event listeners for new header dropdowns
    document.querySelectorAll('.table-header-filter').forEach(select => {
        select.addEventListener('change', (event) => {
            const currentAppState = stateService.getState(); // Get current state
            const filterElementId = event.target.id; // e.g., "colHeaderFilter-http_status_code"
            const filterValue = event.target.value;
            let newFilterStatePartial = {};

            if (filterElementId === 'colHeaderFilter-http_status_code') {
                newFilterStatePartial.filterHTTPStatusCode = filterValue;
            } else if (filterElementId === 'colHeaderFilter-http_server') {
                newFilterStatePartial.filterHTTPServer = filterValue;
            } else if (filterElementId === 'colHeaderFilter-http_tech') {
                newFilterStatePartial.filterHTTPTech = filterValue;
            }
            
            // Update the global state immediately
            stateService.updateState({
                paginationState: {
                    ...currentAppState.paginationState,
                    domainsView: {
                        ...currentAppState.paginationState.domainsView,
                        ...newFilterStatePartial, // Apply the specific filter change
                        currentPage: 1 // Reset to page 1 on any filter change
                    }
                }
            });
            // Now call fetchAndRender, which will use the updated global state
            fetchAndRenderDomainsTable(currentAppState.currentTargetId);
        });
    });

    tableService.makeTableColumnsResizable('domainsTableHead', columnConfig);
}


function renderDomainsPaginationControls(containerElement, paginationData) {
    const container = containerElement;
    if (!container) return;
    
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords, limit, sortBy, sortOrder } = appState.paginationState.domainsView;

    let paginationHTML = '';
    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total domain(s) found.</p>` : '';
        return;
    }

    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total domains)</p>`;

    const buildHash = (page, newLimit = limit) => {
        // Use the latest state for building hash, which should have been updated by fetchAndRenderDomainsTable
        const currentDomainsViewState = stateService.getState().paginationState.domainsView;
        const queryParams = new URLSearchParams({
            page: page,
            limit: newLimit,
            sort_by: currentDomainsViewState.sortBy,
            sort_order: currentDomainsViewState.sortOrder,
            domain_name_search: currentDomainsViewState.filterDomainName,
            source_search: currentDomainsViewState.filterSource,
        });
        if (currentDomainsViewState.filterIsInScope !== null) {
            queryParams.set('is_in_scope', currentDomainsViewState.filterIsInScope.toString());
        }
        if (currentDomainsViewState.filterIsFavorite === true) { // Only add if true
            queryParams.set('is_favorite', 'true');
        }
        if (currentDomainsViewState.filterHttpxScanStatus && currentDomainsViewState.filterHttpxScanStatus !== 'all') {
            queryParams.set('httpx_scan_status', currentDomainsViewState.filterHttpxScanStatus);
        }
        // New: Add new filters to pagination links
        if (currentDomainsViewState.filterHTTPStatusCode) {
            queryParams.set('filter_http_status_code', currentDomainsViewState.filterHTTPStatusCode);
        }
        if (currentDomainsViewState.filterHTTPServer) {
            queryParams.set('filter_http_server', currentDomainsViewState.filterHTTPServer);
        }
        if (currentDomainsViewState.filterHTTPTech) {
            queryParams.set('filter_http_tech', currentDomainsViewState.filterHTTPTech);
        }
        return `#domains?${queryParams.toString()}`;
    };

    const firstButton = uiService.createButton('&laquo; First', () => { if (currentPage > 1) window.location.hash = buildHash(1); }, { disabled: currentPage <= 1, classNames: ['secondary', 'small-button'], marginRight: '5px' });
    const prevButton = uiService.createButton('&laquo; Previous', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); }, { disabled: currentPage <= 1, classNames: ['primary'], marginRight: '5px' });
    const nextButton = uiService.createButton('Next &raquo;', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); }, { disabled: currentPage >= totalPages, classNames: ['primary'], marginRight: '5px' });
    const lastButton = uiService.createButton('Last &raquo;', () => { if (currentPage < totalPages) window.location.hash = buildHash(totalPages); }, { disabled: currentPage >= totalPages, classNames: ['secondary', 'small-button'] });

    const itemsPerPageSelect = uiService.createSelect([5, 10, 15, 25, 50, 100, 200].map(val => ({ value: val, text: `${val} per page` })), limit, (e) => {
        const newLimit = parseInt(e.target.value, 10);
        window.location.hash = buildHash(1, newLimit);
    }, { id: `domainsItemsPerPageSelect_${container.id}`, marginLeft: '15px' }); // Ensure unique ID for select if used in multiple places

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    container.appendChild(firstButton);
    container.appendChild(prevButton);
    container.appendChild(nextButton);
    container.appendChild(lastButton);
    container.appendChild(itemsPerPageSelect);
}

function handleDomainSort(sortKey) {
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) {
        console.log('[DomainsView] Sort prevented due to active resize operation.');
        return;
    }
    if (!sortKey) return; // Do nothing if sortKey is null or undefined

    const appState = stateService.getState();
    const currentSort = appState.paginationState.domainsView;
    let newSortOrder = 'ASC';
    if (currentSort.sortBy === sortKey && currentSort.sortOrder === 'ASC') {
        newSortOrder = 'DESC';
    }
    stateService.updateState({
        paginationState: {
            ...appState.paginationState,
            domainsView: { ...currentSort, sortBy: sortKey, sortOrder: newSortOrder, currentPage: 1 }
        }
    });

    fetchAndRenderDomainsTable(appState.currentTargetId);
}

function resetAndFetchDomains() {
    const appState = stateService.getState();
    const defaultDomainsState = {
        currentPage: 1,
        limit: 25,
        sortBy: 'domain_name',
        sortOrder: 'ASC',
        filterDomainName: '',
        filterSource: '',
        filterIsFavorite: false,
        filterIsInScope: null,
        filterHTTPStatusCode: '', // Reset new filters
        filterHTTPServer: '',
        filterHTTPTech: '',
        filterHttpxScanStatus: 'all',
        totalPages: 0,
        totalRecords: 0,
    };
    stateService.updateState({
        paginationState: {
            ...appState.paginationState,
            domainsView: defaultDomainsState
        }
    });
    const domainNameSearchEl = document.getElementById('domainNameSearch');
    const sourceSearchEl = document.getElementById('domainSourceSearch');
    const inScopeFilterEl = document.getElementById('domainInScopeFilter');
    const favoriteFilterEl = document.getElementById('domainFavoriteFilter'); // Corrected variable name
    const httpxScanStatusEl = document.getElementById('domainHttpxScanStatusFilter'); // For existing dropdown
    // New: Reset new header dropdowns
    const statusCodeFilterEl = document.getElementById('colHeaderFilter-http_status_code');
    const serverFilterEl = document.getElementById('colHeaderFilter-http_server');
    const techFilterEl = document.getElementById('colHeaderFilter-http_tech');

    if(domainNameSearchEl) domainNameSearchEl.value = '';
    if(favoriteFilterEl) favoriteFilterEl.checked = false; // Reset favorite filter
    if(sourceSearchEl) sourceSearchEl.value = '';
    if(inScopeFilterEl) inScopeFilterEl.value = '';
    if(httpxScanStatusEl) httpxScanStatusEl.value = 'all';
    if(statusCodeFilterEl) statusCodeFilterEl.value = '';
    if(serverFilterEl) serverFilterEl.value = '';
    if(techFilterEl) techFilterEl.value = '';

    // Clear pagination controls as well
    const topPaginationForReset = document.getElementById('domainsPaginationControlsTop');
    if (topPaginationForReset) {
        topPaginationForReset.innerHTML = '';
    }
    const bottomPaginationForReset = document.getElementById('domainsPaginationControls');
    if (bottomPaginationForReset) {
        bottomPaginationForReset.innerHTML = '';
    }

    fetchAndRenderDomainsTable(appState.currentTargetId);
}

function displayAddDomainModal(targetId) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing. Cannot add domain.");
        return;
    }

    const modalContentHTML = `
        <form id="addDomainForm">
            <div class="form-group">
                <label for="newDomainName">Domain Name (Required):</label>
                <input type="text" id="newDomainName" name="domain_name" required>
            </div>
            <div class="form-group">
                <label for="newDomainSource">Source (e.g., manual, subfinder):</label>
                <input type="text" id="newDomainSource" name="source">
            </div>
            <div class="form-group">
                <label for="newDomainIsInScope">Is In Scope:</label>
                <input type="checkbox" id="newDomainIsInScope" name="is_in_scope" checked>
            </div>
            <div class="form-group">
                <label for="newDomainNotes">Notes:</label>
                <textarea id="newDomainNotes" name="notes" rows="3"></textarea>
            </div>
            <div id="addDomainModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>
    `;

    uiService.showModalConfirm(
        "Add New Domain",
        modalContentHTML,
        async () => { 
            const form = document.getElementById('addDomainForm');
            const modalMessageArea = document.getElementById('addDomainModalMessage');
            if (!form || !modalMessageArea) return false; 

            const domainName = form.elements.domain_name.value.trim();
            if (!domainName) {
                modalMessageArea.textContent = "Domain Name is required.";
                modalMessageArea.className = 'message-area error-message';
                return false; 
            }

            const domainData = {
                target_id: parseInt(targetId, 10),
                domain_name: domainName,
                source: { String: form.elements.source.value.trim(), Valid: !!form.elements.source.value.trim() },
                is_in_scope: form.elements.is_in_scope.checked,
                notes: { String: form.elements.notes.value.trim(), Valid: !!form.elements.notes.value.trim() }
            };

            try {
                await apiService.createDomain(domainData);
                uiService.showModalMessage("Success", `Domain "${escapeHtml(domainName)}" added successfully.`, true, 2000);
                fetchAndRenderDomainsTable(targetId); 
                return true; 
            } catch (error) {
                modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`;
                modalMessageArea.className = 'message-area error-message';
                console.error("Error adding domain:", error);
                return false; 
            }
        },
        () => { /* onCancel */ }, "Add Domain", "Cancel", true
    );
    document.getElementById('newDomainName')?.focus();
}

function displayDiscoverSubdomainsModal(targetId, targetName) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing. Cannot start subdomain discovery.");
        return;
    }
    const initialDomainValue = ''; 
    const modalContentHTML = `
        <form id="discoverSubdomainsForm">
            <p>Run <strong>subfinder</strong> for target: <strong>${escapeHtml(targetName)}</strong> (ID: ${targetId})</p>
            <div class="form-group">
                <label for="subfinderDomain">Domain to Scan (Required):</label>
                <input type="text" id="subfinderDomain" name="domain" value="${escapeHtmlAttribute(initialDomainValue)}" required>
            </div>
            <div class="form-group">
                <label for="subfinderRecursive">Recursive Scan (-r):</label>
                <input type="checkbox" id="subfinderRecursive" name="recursive">
            </div>
            <div class="form-group">
                <label for="subfinderSources">Sources (comma-separated, optional):</label>
                <input type="text" id="subfinderSources" name="sources" placeholder="e.g., virustotal,certspotter">
            </div>
            <div id="discoverSubdomainsModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>
    `;
    uiService.showModalConfirm("Discover Subdomains", modalContentHTML, async () => {
        const form = document.getElementById('discoverSubdomainsForm');
        const modalMessageArea = document.getElementById('discoverSubdomainsModalMessage');
        const domain = form.elements.domain.value.trim();
        if (!domain) {
            modalMessageArea.textContent = "Domain to scan is required.";
            modalMessageArea.className = 'message-area error-message';
            return false;
        }
        const discoveryOptions = {
            domain: domain,
            recursive: form.elements.recursive.checked,
            sources: form.elements.sources.value.trim() ? form.elements.sources.value.split(',').map(s => s.trim()).filter(s => s) : []
        };
        try {
            await apiService.discoverSubdomains(targetId, discoveryOptions);
            startSubfinderStatusUpdates(targetId, `Subdomain discovery for "${escapeHtml(domain)}" initiated.`);
            return true;
        } catch (error) {
            modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`;
            modalMessageArea.className = 'message-area error-message';
            return false;
        }
    }, () => {}, "Start Discovery", "Cancel", true);
    document.getElementById('subfinderDomain')?.focus();
}

async function handleDeleteDomain(domainId) {
    if (!domainId) {
        uiService.showModalMessage("Error", "Domain ID is missing for delete operation.");
        return;
    }
    const domainRow = document.querySelector(`tr[data-domain-id="${domainId}"]`);
    const domainName = domainRow ? domainRow.cells[1]?.textContent : `ID ${domainId}`;
    uiService.showModalConfirm("Confirm Delete Domain", `Are you sure you want to delete domain "${escapeHtml(domainName)}"? This action cannot be undone.`, async () => {
        try {
            await apiService.deleteDomain(domainId);
            uiService.showModalMessage("Success", `Domain "${escapeHtml(domainName)}" deleted successfully.`, true, 2000);
            fetchAndRenderDomainsTable(stateService.getState().currentTargetId);
        } catch (error) {
            uiService.showModalMessage("Error", `Failed to delete domain: ${escapeHtml(error.message)}`);
        }
    });
}

function handleSelectAllDomainsChange(event) {
    const isChecked = event.target.checked;
    document.querySelectorAll('.domain-item-checkbox').forEach(checkbox => {
        checkbox.checked = isChecked;
    });
}

async function handleSendSelectedToSubfinder(targetId) {
    const selectedCheckboxes = document.querySelectorAll('.domain-item-checkbox:checked');
    if (selectedCheckboxes.length === 0) {
        uiService.showModalMessage("Info", "No domains selected to send to Subfinder.");
        return;
    }
    
    let initialMsg = `Initiating Subfinder for ${selectedCheckboxes.length} selected domain(s)...`;
    startSubfinderStatusUpdates(targetId, initialMsg);

    let successCount = 0;
    let errorCount = 0;
    for (const checkbox of selectedCheckboxes) {
        const domainName = checkbox.dataset.domainName;
        const discoveryOptions = { domain: domainName }; // Assuming no recursive/sources for batch send for now
        try {
            await apiService.discoverSubdomains(targetId, discoveryOptions);
            successCount++;
        } catch (error) {
            errorCount++;
            console.error(`Error initiating Subfinder for ${domainName}:`, error);
        }
    }
    // The status polling will take over, but we can log the initiation counts.
    console.log(`[DomainsView] Subfinder initiation: ${successCount} successful, ${errorCount} failed.`);
    // The initial message in startSubfinderStatusUpdates will be shown, 
    // and then polling will update it.
}

async function handleExportDomainsCSV(event) {
    if (event) event.preventDefault();
    document.getElementById('exportDomainsDropdownMenu').classList.remove('show');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId;
    const domainsViewState = appState.paginationState.domainsView;

    uiService.showModalMessage("Exporting...", "Fetching all domains for CSV export. This may take a moment...", true);

    let allDomainsForExport = [];
    try {
        // Gather current filter values from the UI, falling back to state if UI elements aren't present
        const domainNameSearch = document.getElementById('domainNameSearch')?.value || domainsViewState.filterDomainName;
        const sourceSearch = document.getElementById('domainSourceSearch')?.value || domainsViewState.filterSource;
        const inScopeFilterEl = document.getElementById('domainInScopeFilter');
        const isInScope = (inScopeFilterEl && inScopeFilterEl.value !== "") ? (inScopeFilterEl.value === 'true') : null;
        const favoriteFilterEl = document.getElementById('domainFavoriteFilter');
        const isFavorite = favoriteFilterEl ? favoriteFilterEl.checked : domainsViewState.filterIsFavorite;

        const exportParams = {
            page: 1,
            limit: 0, // Fetch all matching records
            sort_by: domainsViewState.sortBy, // Keep current sort order for consistency
            sort_order: domainsViewState.sortOrder,
            domain_name_search: domainNameSearch,
            source_search: sourceSearch,
        };
        if (isInScope !== null) {
            exportParams.is_in_scope = isInScope.toString();
        }
        if (isFavorite === true) {
            exportParams.is_favorite = 'true';
        }

        const response = await apiService.getDomains(targetId, exportParams);
        allDomainsForExport = response.records || [];
    } catch (error) {
        uiService.showModalMessage("Export Error", `Failed to fetch all domains for export: ${escapeHtml(error.message)}`);
        return;
    }

    const columnConfig = appState.paginationState.domainsTableLayout;
    const headers = Object.keys(columnConfig)
        .filter(key => columnConfig[key].visible && key !== 'actions' && key !== 'checkbox')
        .map(key => columnConfig[key].label);

    const rows = allDomainsForExport.map(domain => {
        return Object.keys(columnConfig)
            .filter(key => columnConfig[key].visible && key !== 'actions' && key !== 'checkbox')
            .map(key => {
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
                    default: return '';
                }
            });
    });

    let csvContent = headers.join(",") + "\n";
    rows.forEach(rowArray => {
        let row = rowArray.map(field => `"${String(field).replace(/"/g, '""')}"`).join(",");
        csvContent += row + "\n";
    });

    downloadCSV(csvContent, `domains_target_${targetId}.csv`);
    uiService.showModalMessage("Success", "Domains exported to CSV.", true, 2000);
}

async function handleExportDomainsTXT(event) {
    if (event) event.preventDefault();
    document.getElementById('exportDomainsDropdownMenu').classList.remove('show');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId;
    const domainsViewState = appState.paginationState.domainsView;

    uiService.showModalMessage("Exporting...", "Fetching all domains for TXT export. This may take a moment...", true);

    let allDomainsForExport = [];
    try {
        // Gather current filter values from the UI, falling back to state if UI elements aren't present
        const domainNameSearch = document.getElementById('domainNameSearch')?.value || domainsViewState.filterDomainName;
        const sourceSearch = document.getElementById('domainSourceSearch')?.value || domainsViewState.filterSource;
        const inScopeFilterEl = document.getElementById('domainInScopeFilter');
        const isInScope = (inScopeFilterEl && inScopeFilterEl.value !== "") ? (inScopeFilterEl.value === 'true') : null;
        const favoriteFilterEl = document.getElementById('domainFavoriteFilter');
        const isFavorite = favoriteFilterEl ? favoriteFilterEl.checked : domainsViewState.filterIsFavorite;

        const exportParams = {
            page: 1,
            limit: 0, // Fetch all matching records
            sort_by: domainsViewState.sortBy, // Keep current sort order
            sort_order: domainsViewState.sortOrder,
            domain_name_search: domainNameSearch,
            source_search: sourceSearch,
        };
        if (isInScope !== null) {
            exportParams.is_in_scope = isInScope.toString();
        }
        if (isFavorite === true) {
            exportParams.is_favorite = 'true';
        }
        const response = await apiService.getDomains(targetId, exportParams);
        allDomainsForExport = response.records || [];
    } catch (error) {
        uiService.showModalMessage("Export Error", `Failed to fetch all domains for export: ${escapeHtml(error.message)}`);
        return;
    }

    const txtContent = allDomainsForExport.map(domain => domain.domain_name).join("\n");
    downloadTXT(txtContent, `domains_target_${targetId}.txt`);
    uiService.showModalMessage("Success", "Domains exported to TXT.", true, 2000);
}

async function handleEditDomain(domainId) {
    if (!domainId) {
        uiService.showModalMessage("Error", "Domain ID is missing for edit operation.");
        return;
    }
    const domainRow = document.querySelector(`tr[data-domain-id="${domainId}"]`);
    if (!domainRow) {
        uiService.showModalMessage("Error", "Could not find domain data to edit.");
        return;
    }

    // Find the domain data from currentDomainsData to get full, un-truncated values
    const domainData = currentDomainsData.find(d => d.id == domainId);
    if (!domainData) {
        uiService.showModalMessage("Error", "Could not retrieve full domain data for editing.");
        return;
    }

    const currentDomainName = domainData.domain_name;
    const currentSource = domainData.source?.String || '';
    const currentIsInScope = domainData.is_in_scope;
    const currentNotes = domainData.notes?.String || '';


    const modalContentHTML = `
        <form id="editDomainForm">
            <p><strong>Domain:</strong> ${escapeHtml(currentDomainName)} (ID: ${domainId})</p>
            <div class="form-group">
                <label for="editDomainSource">Source:</label>
                <input type="text" id="editDomainSource" name="source" value="${escapeHtmlAttribute(currentSource)}">
            </div>
            <div class="form-group">
                <label for="editDomainIsInScope">Is In Scope:</label>
                <input type="checkbox" id="editDomainIsInScope" name="is_in_scope" ${currentIsInScope ? 'checked' : ''}>
            </div>
            <div class="form-group">
                <label for="editDomainNotes">Notes:</label>
                <textarea id="editDomainNotes" name="notes" rows="3">${escapeHtml(currentNotes)}</textarea>
            </div>
            <div id="editDomainModalMessage" class="message-area" style="margin-top: 10px;"></div>
        </form>
    `;

    uiService.showModalConfirm(`Edit Domain: ${escapeHtml(currentDomainName)}`, modalContentHTML, async () => {
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
            uiService.showModalMessage("Success", `Domain "${escapeHtml(currentDomainName)}" updated successfully.`, true, 2000);
            fetchAndRenderDomainsTable(stateService.getState().currentTargetId);
            return true;
        } catch (error) {
            modalMessageArea.textContent = `Error: ${escapeHtml(error.message)}`;
            modalMessageArea.className = 'message-area error-message';
            console.error("Error updating domain:", error);
            return false;
        }
    }, () => {}, "Save Changes", "Cancel", true);
    document.getElementById('editDomainSource')?.focus();
}

async function handleImportInScopeDomains(targetId, targetName) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing. Cannot import domains.");
        return;
    }

    // Stop any existing subfinder polling and clear its message area.
    stopSubfinderStatusUpdates();
    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea) {
        setMessageWithCloseButton(messageArea, `Importing in-scope domains for "${escapeHtml(targetName)}"...`, 'info-message');
    }

    try {
        const result = await apiService.importInScopeDomains(targetId);
        if (messageArea) {
            const successMsgText = `Successfully imported in-scope domains. Imported: ${result.imported_count || 0}, Skipped: ${result.skipped_count || 0}. ${escapeHtml(result.message || '')}`;
            setMessageWithCloseButton(messageArea, successMsgText, 'success-message', 3000); // Auto-hide after 3s
        }
        fetchAndRenderDomainsTable(targetId); // Refresh table immediately
    } catch (error) {
        if (messageArea) {
            setMessageWithCloseButton(messageArea, `Error importing in-scope domains: ${escapeHtml(error.message)}`, 'error-message');
        }
        stopSubfinderStatusUpdates(); // Stop polling on error
        console.error("Error importing in-scope domains:", error);
    }
}

async function handleDeleteAllDomains(targetId, targetName) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing. Cannot delete domains.");
        return;
    }
    uiService.showModalConfirm("Confirm Delete All Domains", `Are you sure you want to delete ALL domains for target "${escapeHtml(targetName)}"? This action cannot be undone.`, async () => {
        try {
            const result = await apiService.deleteAllDomainsForTarget(targetId);
            uiService.showModalMessage("Success", `All domains for target "${escapeHtml(targetName)}" deleted. Count: ${result.deleted_count || 0}.`, true, 3000);
            fetchAndRenderDomainsTable(targetId);
        } catch (error) {
            uiService.showModalMessage("Error", `Failed to delete all domains: ${escapeHtml(error.message)}`);
            console.error("Error deleting all domains:", error);
        }
    });
}

async function handleSendSelectedToHttpx(targetId) {
    const selectedCheckboxes = document.querySelectorAll('.domain-item-checkbox:checked');
    if (selectedCheckboxes.length === 0) {
        uiService.showModalMessage("Info", "No domains selected to send to httpx.");
        return;
    }

    const domainIds = Array.from(selectedCheckboxes).map(cb => parseInt(cb.value, 10));

    const messageArea = document.getElementById('discoverSubdomainsMessage');
    if (messageArea) {
        try {
            startHttpxStatusUpdates(targetId, `Sending ${domainIds.length} selected domain(s) for httpx scan...`);
            await apiService.runHttpxForDomains(targetId, domainIds); // Fire and forget, status polling will handle UI
        } catch (error) {
        if (messageArea) setMessageWithCloseButton(messageArea, `Error initiating httpx scan: ${escapeHtml(error.message)}`, 'error-message');
        console.error("Error initiating httpx scan:", error);
    }
}
}

async function handleDomainFavoriteToggle(event) {
    const button = event.currentTarget;
    const domainId = button.getAttribute('data-domain-id');
    const isCurrentlyFavorite = button.getAttribute('data-is-favorite') === 'true';
    const newFavoriteState = !isCurrentlyFavorite;

    try {
        await apiService.setDomainFavorite(domainId, newFavoriteState);
        button.innerHTML = '★'; // Always show star, class handles color
        button.classList.toggle('favorited', newFavoriteState);
        button.setAttribute('data-is-favorite', newFavoriteState.toString());
    } catch (favError) {
        console.error("Error toggling domain favorite:", favError);
        uiService.showModalMessage("Error", `Failed to update favorite status for domain ${domainId}: ${favError.message}`);
    }
}

async function handleFavoriteAllFiltered(targetId) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing.");
        return;
    }

    const domainNameSearch = document.getElementById('domainNameSearch')?.value || '';
    const sourceSearch = document.getElementById('domainSourceSearch')?.value || '';
    const inScopeFilterEl = document.getElementById('domainInScopeFilter');
    const isInScope = (inScopeFilterEl && inScopeFilterEl.value !== "") ? (inScopeFilterEl.value === 'true') : null;

    let filterDescription = "current filters";
    if (domainNameSearch || sourceSearch || isInScope !== null) {
        filterDescription = `filters (Domain: '${domainNameSearch}', Source: '${sourceSearch}', In Scope: ${isInScope === null ? 'Any' : isInScope})`;
    }

    uiService.showModalConfirm(
        "Confirm Favorite All",
        `Are you sure you want to mark ALL domains matching ${filterDescription} as favorite for this target?`,
        async () => {
            uiService.showModalMessage("Processing...", "Favoriting filtered domains...", true);
            try {
                const filters = {
                    domain_name_search: domainNameSearch,
                    source_search: sourceSearch,
                    is_in_scope: isInScope,
                };
                const result = await apiService.favoriteAllFilteredDomains(targetId, filters);
                uiService.showModalMessage("Success", `${result.updated_count || 0} domain(s) marked as favorite.`, true, 3000);
                fetchAndRenderDomainsTable(targetId);
            } catch (error) {
                uiService.showModalMessage("Error", `Failed to favorite all filtered domains: ${escapeHtml(error.message)}`);
            }
        }
    );
}

async function handleSendAllFilteredToHttpx(targetId) {
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID is missing.");
        return;
    }

    const appState = stateService.getState();
    const currentFilters = appState.paginationState.domainsView;

    const filtersForAPI = {
        domain_name_search: currentFilters.filterDomainName,
        source_search: currentFilters.filterSource,
        is_in_scope: currentFilters.filterIsInScope,
        is_favorite: currentFilters.filterIsFavorite,
    };

    const messageArea = document.getElementById('discoverSubdomainsMessage'); // Reuse existing message area
    if (messageArea) {
        try {
            // Start polling and then make the API call
            startHttpxStatusUpdates(targetId, `Initiating httpx scan for ALL filtered domains... This might take a while.`);
            // The API call itself is fire-and-forget from the UI's perspective; status polling handles updates.
            await apiService.runHttpxForAllFilteredDomains(targetId, filtersForAPI);
        } catch (error) {
            if (messageArea) setMessageWithCloseButton(messageArea, `Error initiating httpx scan for all filtered domains: ${escapeHtml(error.message)}`, 'error-message');
            console.error("Error initiating httpx scan for all filtered domains:", error);
        }
    }
}
