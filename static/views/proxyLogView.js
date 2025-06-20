// static/views/proxyLogView.js
import { escapeHtml, escapeHtmlAttribute, debounce, downloadCSV } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService;

// DOM element references (will be queried within functions or passed)
let viewContentContainer; // Main container, passed to load functions

/**
 * Initializes the Proxy Log View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService, tableService.
 */
export function initProxyLogView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService;
    console.log("[ProxyLogView] Initialized.");
}

function formatHeaders(headers) {
    if (!headers || Object.keys(headers).length === 0) return '(No headers)';
    return Object.entries(headers).map(([key, value]) => `${key}: ${Array.isArray(value) ? value.join(', ') : value}`).join('\n');
}

function formatBody(body, contentType = '') {
    //if (!body) return '(Empty body)';
    if (!body) return '';
    try {
        const decodedText = atob(body); // Assuming body is always base64 string from backend
        // If after decoding, the text is empty or only whitespace, treat it as an empty body.
        // This handles cases where the body might be " ", "\n", etc.
        if (decodedText.trim() === '') {
            //return '(Empty body)';
            return '';
        }

        const lowerContentType = contentType.toLowerCase();

        if (lowerContentType.includes('json')) {
            try {
                // If it's JSON, pretty-print it.
                return JSON.stringify(JSON.parse(decodedText), null, 2);
            } catch (e) {
                // If JSON parsing fails, treat it as plain text and escape it.
                return escapeHtml(decodedText.replace(/[\x00-\x1F\x7F-\x9F]/g, '.'));
            }
        } else if (
            lowerContentType === '' || // Treat empty contentType as potentially displayable text
            lowerContentType.includes('javascript') || // Existing conditions
            lowerContentType.includes('text') || // Catches text/plain, text/html, text/css etc.
            lowerContentType.includes('xml') ||
            lowerContentType.includes('svg') // SVG is XML-based and often text
        ) {
            // For these text-based types, return the raw decoded text (after cleaning control characters).
            // textContent will handle displaying it literally without rendering HTML.
            return decodedText.replace(/[\x00-\x1F\x7F-\x9F]/g, '.');
        }
        // For other content types (e.g., images, binary), show a placeholder or truncated raw (escaped)
        // Since we've already decoded, showing a placeholder for non-text is better.
        return `(Binary or non-displayable content type: ${escapeHtml(contentType)})`;
    } catch (e) {
        // If atob fails (not valid Base64)
        console.error("Error decoding base64 body:", e);
        // Display '(Empty body)' as requested, instead of the detailed error.
        //return '(Empty body)';
        return '';
    }
}

async function handleLogAddAsFinding(logId) {
    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    if (!currentTargetId) {
        uiService.showModalMessage("Error", "No current target set. Cannot add finding.");
        return;
    }
    if (!logId) {
        uiService.showModalMessage("Error", "Log ID is missing for 'Add as Finding'.");
        return;
    }

    // Navigate to the current target's findings tab and pass the logId for pre-filling
    // The 'action=addFinding' will be picked up by app.js to trigger the form
    // The 'from_log_id' will be used to fetch details and pre-fill
    const newHash = `#current-target?id=${currentTargetId}&tab=findingsTab&action=addFinding&from_log_id=${logId}`;
    window.location.hash = newHash;
}

async function handleViewAssociatedFindingDetailFromProxyLog(findingId) {
    if (!apiService || !uiService) {
        console.error("Services not available for viewing finding detail from proxy log.");
        alert("Error: UI services not available.");
        return;
    }
    try {
        const finding = await apiService.getFindingDetails(findingId); // Assumes this function exists in apiService
        
        let detailHTML = `
            <div class="finding-details-grid">
                <div class="detail-item">Severity:</div><div class="detail-value">${escapeHtml(finding.severity.String || 'N/A')}</div>
                <div class="detail-item">Status:</div><div class="detail-value">${escapeHtml(finding.status)}</div>
                <div class="detail-item">ID:</div><div class="detail-value">${finding.id}</div>
                <div class="detail-item">Target ID:</div><div class="detail-value">${finding.target_id}</div>
                <div class="detail-item">CVSS Score:</div><div class="detail-value">${finding.cvss_score.Valid ? finding.cvss_score.Float64 : 'N/A'}</div>
                <div class="detail-item">CWE ID:</div><div class="detail-value">${finding.cwe_id.Valid ? finding.cwe_id.Int64 : 'N/A'}</div>
                <div class="detail-item">Discovered:</div><div class="detail-value">${new Date(finding.discovered_at).toLocaleString()}</div>
                <div class="detail-item">Last Updated:</div><div class="detail-value">${new Date(finding.updated_at).toLocaleString()}</div>
                <div class="detail-item">HTTP Log ID:</div><div class="detail-value">${finding.http_traffic_log_id.Valid ? finding.http_traffic_log_id.Int64 : 'N/A'}</div>
            </div>
            <div class="finding-detail-full-width"><p><strong>Description:</strong></p><pre>${escapeHtml(finding.description.String || 'N/A')}</pre></div>
            <div class="finding-detail-full-width"><p><strong>Payload:</strong></p><pre>${escapeHtml(finding.payload.String || 'N/A')}</pre></div>
            <div class="finding-detail-full-width"><p><strong>References:</strong></p><pre>${escapeHtml(finding.finding_references.String || 'N/A')}</pre></div>
        `;
        uiService.showModalMessage(`Finding: ${escapeHtml(finding.title)}`, detailHTML);
    } catch (error) {
        console.error("Error fetching finding details from proxy log view:", error);
        uiService.showModalMessage("Error", `Could not load details for finding ID ${findingId}: ${error.message}`);
    }
}


async function handleProxyLogFavoriteToggle(event) {
    const button = event.currentTarget;
    const logId = button.getAttribute('data-log-id');
    const isCurrentlyFavorite = button.getAttribute('data-is-favorite') === 'true';
    const newFavoriteState = !isCurrentlyFavorite;

    try {
        await apiService.setProxyLogFavorite(logId, newFavoriteState);
        button.innerHTML = newFavoriteState ? '★' : '☆';
        button.classList.toggle('favorited', newFavoriteState);
        button.setAttribute('data-is-favorite', newFavoriteState.toString());
    } catch (favError) {
        console.error("Error toggling favorite from table:", favError);
        uiService.showModalMessage("Error", `Failed to update favorite status for log ${logId}: ${favError.message}`);
    }
}

function handleProxyLogFavoriteFilterChange(event) {
    const isChecked = event.target.checked;
    const appState = stateService.getState();
    const currentProxyLogState = appState.paginationState.proxyLog;
    window.location.hash = `#proxy-log?page=1&sort_by=${currentProxyLogState.sortBy}&sort_order=${currentProxyLogState.sortOrder}&favorites_only=${isChecked}&method=${encodeURIComponent(currentProxyLogState.filterMethod)}&status=${encodeURIComponent(currentProxyLogState.filterStatus)}&type=${encodeURIComponent(currentProxyLogState.filterContentType)}&search=${encodeURIComponent(currentProxyLogState.filterSearchText)}`;
}

function handleProxyLogSearch(event) {
    const newSearchText = event.target.value.trim();
    const appState = stateService.getState();
    const currentProxyLogState = appState.paginationState.proxyLog;
    if (currentProxyLogState.filterSearchText !== newSearchText) {
        window.location.hash = `#proxy-log?page=1&sort_by=${currentProxyLogState.sortBy}&sort_order=${currentProxyLogState.sortOrder}&favorites_only=${currentProxyLogState.filterFavoritesOnly}&method=${encodeURIComponent(currentProxyLogState.filterMethod)}&status=${encodeURIComponent(currentProxyLogState.filterStatus)}&type=${encodeURIComponent(currentProxyLogState.filterContentType)}&search=${encodeURIComponent(newSearchText)}`;
    }
}

function handleProxyLogFilterChange(event) {
    event.stopPropagation(); // Prevent event from bubbling up
    console.log('[ProxyLogView] handleProxyLogFilterChange triggered by:', event.target);
    const filterKey = event.target.getAttribute('data-filter-key');
    const newFilterValue = event.target.value;
    const appState = stateService.getState();
    console.log(`[ProxyLogView] Filter Key: "${filterKey}", New Filter Value: "${newFilterValue}"`);
    const pState = appState.paginationState.proxyLog;

    let finalMethod = pState.filterMethod;
    let finalStatus = pState.filterStatus;
    let finalContentType = pState.filterContentType;

    if (filterKey === 'method') finalMethod = newFilterValue;
    else if (filterKey === 'status') finalStatus = newFilterValue;
    else if (filterKey === 'type') finalContentType = newFilterValue;

    const queryParams = new URLSearchParams({
        page: '1',
        sort_by: pState.sortBy,
        sort_order: pState.sortOrder,
        favorites_only: pState.filterFavoritesOnly.toString(),
        method: finalMethod,
        status: finalStatus,
        type: finalContentType,
        search: pState.filterSearchText
    });
    console.log('[ProxyLogView] New Hash Query Params:', queryParams.toString());
    window.location.hash = `#proxy-log?${queryParams.toString()}`;
}

function handleProxyLogSort(event) {
    // Prevent sorting if a column resize was just completed
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) {
        console.log('[ProxyLogView] Sort prevented due to active resize operation.');
        return;
    }
    const newSortBy = event.target.closest('th').getAttribute('data-sort-key');
    console.log('[ProxyLogView] handleProxyLogSort triggered for key:', newSortBy);
    if (!newSortBy) return;
    const appState = stateService.getState();
    const currentProxyLogState = appState.paginationState.proxyLog;
    let newSortOrder = 'ASC';
    if (currentProxyLogState.sortBy === newSortBy) {
        newSortOrder = currentProxyLogState.sortOrder === 'ASC' ? 'DESC' : 'ASC';
    }
    window.location.hash = `#proxy-log?page=1&sort_by=${newSortBy}&sort_order=${newSortOrder}&favorites_only=${currentProxyLogState.filterFavoritesOnly}&method=${encodeURIComponent(currentProxyLogState.filterMethod)}&status=${encodeURIComponent(currentProxyLogState.filterStatus)}&type=${encodeURIComponent(currentProxyLogState.filterContentType)}&search=${encodeURIComponent(currentProxyLogState.filterSearchText)}`;
}

function renderProxyLogPagination(container) {
    if (!container) return;
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords, limit, sortBy, sortOrder, filterFavoritesOnly, filterMethod, filterStatus, filterContentType, filterSearchText } = appState.paginationState.proxyLog;
    let paginationHTML = '';

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total log(s) found.</p>` : '';
        return;
    }
    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total logs)</p>`;
    const buildHash = (page, newLimit = limit) => `#proxy-log?page=${page}&limit=${newLimit}&sort_by=${sortBy}&sort_order=${sortOrder}&favorites_only=${filterFavoritesOnly}&method=${encodeURIComponent(filterMethod)}&status=${encodeURIComponent(filterStatus)}&type=${encodeURIComponent(filterContentType)}&search=${encodeURIComponent(filterSearchText)}`;

    const firstButton = document.createElement('button');
    firstButton.style.marginRight = '5px';
    firstButton.innerHTML = '&laquo; First';
    firstButton.disabled = (currentPage <= 1);
    firstButton.className = firstButton.disabled ? 'secondary small-button' : 'primary small-button';
    firstButton.addEventListener('click', () => { if (currentPage > 1) window.location.hash = buildHash(1); });

    const prevButton = document.createElement('button');
    prevButton.style.marginRight = '5px';
    prevButton.innerHTML = '&laquo; Previous';
    prevButton.disabled = (currentPage <= 1);
    prevButton.className = prevButton.disabled ? 'secondary' : 'primary';
    prevButton.addEventListener('click', () => { if (currentPage > 1) window.location.hash = buildHash(currentPage - 1); });

    const nextButton = document.createElement('button');
    nextButton.style.marginRight = '5px'; // Added margin for spacing from Last button
    nextButton.innerHTML = 'Next &raquo;';
    nextButton.disabled = (currentPage >= totalPages);
    nextButton.className = nextButton.disabled ? 'secondary' : 'primary';
    nextButton.addEventListener('click', () => { if (currentPage < totalPages) window.location.hash = buildHash(currentPage + 1); });

    const lastButton = document.createElement('button');
    lastButton.innerHTML = 'Last &raquo;';
    lastButton.disabled = (currentPage >= totalPages);
    lastButton.className = lastButton.disabled ? 'secondary small-button' : 'primary small-button';
    lastButton.addEventListener('click', () => { if (currentPage < totalPages) window.location.hash = buildHash(totalPages); });

    const itemsPerPageSelect = document.createElement('select');
    itemsPerPageSelect.id = 'proxyLogItemsPerPageSelect';
    itemsPerPageSelect.style.marginLeft = '15px';
    [5, 10, 15, 25, 50, 100, 200].forEach(val => {
        const option = document.createElement('option');
        option.value = val;
        option.textContent = `${val} per page`;
        if (limit === val) option.selected = true;
        itemsPerPageSelect.appendChild(option);
    });
    itemsPerPageSelect.addEventListener('change', (e) => {
        const newLimit = parseInt(e.target.value, 10);
        window.location.hash = buildHash(1, newLimit); // Go to page 1 with new limit
    });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    // Always append all buttons
    container.appendChild(firstButton);
    container.appendChild(prevButton);
    container.appendChild(nextButton);
    container.appendChild(lastButton);
    container.appendChild(itemsPerPageSelect);
}

function handleViewLogDetail(event) {
    const button = event.target.closest('button');
    const logId = button.dataset.logId; // Use dataset for cleaner access

    if (!logId) {
        console.error('Log ID not found for view action.');
        return;
    }

    const detailHashPath = `#proxy-log-detail?id=${logId}`;

    if (event.ctrlKey || event.metaKey) { // Check for Ctrl or Command key
        event.preventDefault(); // Prevent default click behavior

        // Construct the full URL for the new tab
        // window.location.origin gives http://localhost:8778
        // window.location.pathname gives the path of the current page (e.g., /)
        // .replace(/\/$/, '') removes a trailing slash from pathname if it exists,
        // to prevent double slashes if detailHashPath already starts with one.
        const baseUrl = window.location.origin + window.location.pathname.replace(/\/$/, '');
        const fullUrl = baseUrl + detailHashPath;

        window.open(fullUrl, '_blank'); // Open in new tab
    } else {
        // Default action: navigate in the current tab using hash change
        window.location.hash = detailHashPath;
    }
}

async function fetchAndDisplayProxyLogs(passedParams = null) {
    const listDiv = document.getElementById('proxyLogListContainer');
    const paginationControlsDiv = document.getElementById('proxyLogPaginationControlsContainer');
    if (!listDiv || !paginationControlsDiv) {
        console.error("Proxy log list or pagination container not found.");
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;
    const defaultProxyLogPaginationState = appState.paginationState.proxyLog;
    const globalTableLayouts = appState.globalTableLayouts || {};
    const tableKey = 'proxyLogTable';
    const savedLayoutForThisTable = globalTableLayouts[tableKey];

    // Ensure all parameters are correctly typed, especially limit and currentPage
    const baseParams = passedParams || defaultProxyLogPaginationState;
    const validatedParams = {
        ...baseParams,
        // Prioritize limit from passedParams (e.g., from hash change via dropdown)
        // Then fallback to saved layout's page size, then to default state's limit.
        limit: (baseParams && baseParams.limit)
               ? parseInt(baseParams.limit, 10)
               : (savedLayoutForThisTable?.pageSize
                  ? parseInt(savedLayoutForThisTable.pageSize, 10)
                  : (defaultProxyLogPaginationState.limit ? parseInt(defaultProxyLogPaginationState.limit, 10) : 25)),
        currentPage: (baseParams && baseParams.currentPage)
                     ? parseInt(baseParams.currentPage, 10)
                     : (defaultProxyLogPaginationState.currentPage ? parseInt(defaultProxyLogPaginationState.currentPage, 10) : 1)
    };
    // Ensure limit and currentPage are valid numbers after potential parsing
    if (isNaN(validatedParams.limit) || validatedParams.limit <= 0) validatedParams.limit = defaultProxyLogPaginationState.limit || 25;
    if (isNaN(validatedParams.currentPage) || validatedParams.currentPage <= 0) validatedParams.currentPage = defaultProxyLogPaginationState.currentPage || 1;


    // Use passedParams if available, otherwise fallback to global state (for initial load or non-filter-driven reloads)
    const { currentPage, limit, sortBy, sortOrder, filterFavoritesOnly, filterMethod, filterStatus, filterContentType, filterSearchText, filterTagIDs } = validatedParams;

    console.log(`[ProxyLogView] fetchAndDisplayProxyLogs using filterMethod: "${filterMethod}"`, validatedParams);
    // const globalTableLayouts = appState.globalTableLayouts; // Already declared above
    // const tableKey = 'proxyLogTable'; // Already declared above
    const columnDefinitions = appState.paginationState.proxyLogTableLayout; // This holds default visibility, labels etc.

    listDiv.innerHTML = `<p>Fetching proxy logs for target ${escapeHtml(currentTargetName)} (ID: ${currentTargetId}), page ${currentPage}, sort by ${sortBy} ${sortOrder}...</p>`;

    try {
        const paramsForAPI = {
            target_id: currentTargetId,
            page: currentPage,
            limit: limit,
            sort_by: sortBy,
            sort_order: sortOrder,
            favorites_only: filterFavoritesOnly,
            method: filterMethod,
            status: filterStatus,
            type: filterContentType,
            search: filterSearchText,
            filter_tag_ids: filterTagIDs && filterTagIDs.length > 0 ? filterTagIDs.join(',') : ''
        };
        const apiResponse = await apiService.getProxyLog(paramsForAPI);
        const logs = apiResponse.logs || [];

        // Store distinct tags from response for the filter dropdown
        const distinctTagsForFilter = apiResponse.distinct_values?.tags || [];
        renderTagFilterDropdown(distinctTagsForFilter); // New function call

        stateService.updateState({
            paginationState: {
                ...appState.paginationState, // Preserve other pagination states (e.g., for checklist)
                proxyLog: {
                    // Use the validated parameters (which have numeric limit and currentPage)
                    ...validatedParams,
                    // Then, override with pagination details from the API response
                    currentPage: apiResponse.page || 1,
                    totalPages: apiResponse.total_pages || 1,
                    totalRecords: apiResponse.total_records || 0,
                }
            }
        });
        const distinctMethods = (apiResponse.distinct_values?.method || []).filter(val => val !== null && String(val).trim() !== '');
        const distinctStatuses = (apiResponse.distinct_values?.status || []).filter(val => val !== null && String(val).trim() !== '');
        const distinctContentTypes = (apiResponse.distinct_values?.type || []).filter(val => val !== null && val !== '' && String(val).toLowerCase() !== 'all');

        const savedLayout = globalTableLayouts[tableKey] || { columns: {}, pageSize: validatedParams.limit };
        const savedColumnSettings = savedLayout.columns || {};

        // Define headers based on columnDefinitions, respecting visibility
        const displayableHeaders = [ // Order matters for display
            { key: 'index', sortKey: 'id', filter: false },
            { key: 'timestamp', sortKey: 'timestamp', filter: false },
            { key: 'method', sortKey: 'request_method', filter: true },
            { key: 'page_name', sortKey: 'page_sitemap_name', filter: false },
            { key: 'url', sortKey: 'request_url', filter: false },
            { key: 'status', sortKey: 'response_status_code', filter: true },
            { key: 'type', sortKey: 'response_content_type', filter: true },
            { key: 'size', sortKey: 'response_body_size', filter: false },
            { key: 'tags', sortKey: 'tags', filter: false },
            { key: 'actions', sortKey: null, filter: false }
        ];

        // For debugging, let's log what layouts are available when rendering
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - globalTableLayouts:", JSON.parse(JSON.stringify(globalTableLayouts)));
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - tableKey:", tableKey);
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - savedLayout for this tableKey:", JSON.parse(JSON.stringify(savedLayout)));
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - columnDefinitions (defaults from state):", JSON.parse(JSON.stringify(columnDefinitions)));

        if (logs.length > 0) {
            let tableHTML = `<table style="table-layout: fixed;"><thead id="proxyLogTableHead"><tr>`;
            displayableHeaders.forEach(h => {
                const colDef = columnDefinitions[h.key];
                const savedColSetting = savedColumnSettings[h.key];
                const isVisible = savedColSetting ? savedColSetting.visible : (colDef ? colDef.visible : true); // Default to true if not in saved or default config

                if (!isVisible) return; // Skip rendering this column

                let classes = h.sortKey ? 'sortable' : '';
                if (h.sortKey === sortBy) classes += sortOrder === 'ASC' ? ' sorted-asc' : ' sorted-desc';
                let filterDropdownHTML = '';
                const colKey = h.key; // Use the key directly
                let thStyleWidth;
                
                // Use saved width if available, otherwise default width from columnDefinitions
                thStyleWidth = savedColSetting?.width || colDef?.default || 'auto';
                if (colDef?.nonResizable) { // Apply fixed width for non-resizable like 'actions'
                    thStyleWidth = colDef.default; 
                }

                // Add the specific class for the actions column header
                if (colKey === 'actions') {
                    classes += ' proxy-log-actions-column';
                }
                if (h.filter) {
                    let options = [];
                    let currentFilterValue = '';
                    switch(h.key) {
                        case 'method': options = [...distinctMethods]; currentFilterValue = filterMethod; break;
                        case 'status': options = [...distinctStatuses]; currentFilterValue = filterStatus; break;
                        case 'type': options = [...distinctContentTypes]; currentFilterValue = filterContentType; break;
                    }
                    options.unshift('');
                    filterDropdownHTML = `<br><select class="proxy-log-filter" data-filter-key="${h.key}" style="margin-top: 5px; width: 90%;">${options.map(opt => `<option value="${escapeHtmlAttribute(String(opt))}" ${String(opt) === String(currentFilterValue) ? 'selected' : ''}>${opt === '' ? 'All' : escapeHtmlAttribute(String(opt))}</option>`).join('')}</select>`;
                }
                // Log the width calculation for each header
                console.log(`[ProxyLogView] Header: ${colDef?.label || h.key}, colKey: '${colKey}', savedWidth: '${savedColSetting?.width}', defaultWidth: '${colDef?.default}', finalWidth: '${thStyleWidth}'`);

                tableHTML += `<th class="${classes}" style="width: ${thStyleWidth};" ${h.sortKey ? `data-sort-key="${h.sortKey}"` : ''} data-col-key="${colKey}" id="${colDef?.id || 'col-proxylog-' + colKey}">${colDef?.label || h.key}${filterDropdownHTML}</th>`;
            });
            tableHTML += `</tr></thead><tbody>`;
            logs.forEach((log, index) => {
                let itemIndex;
                // Use total_records directly from the apiResponse for this rendering pass
                // as appState.paginationState.proxyLog.totalRecords might not be updated yet
                // for this specific execution context of fetchAndDisplayProxyLogs.
                const recordsCountForThisRender = apiResponse.total_records || 0;
                const { currentPage: currentDisplayPage, limit: currentLimit } = appState.paginationState.proxyLog;

                // DEBUGGING: Log values used for itemIndex calculation
                console.log(`[ProxyLogView] Calculating itemIndex: recordsCountForThisRender=${recordsCountForThisRender}, currentPage=${currentDisplayPage}, limit=${currentLimit}, index=${index}, sortBy=${sortBy}, sortOrder=${sortOrder}`);

                // Corrected logic for descending ID sort
                // The goal is to show the highest ID as #1 on page 1, then count down.
                if (sortBy === 'id' && sortOrder === 'DESC') {
                    itemIndex = recordsCountForThisRender - ((currentDisplayPage - 1) * currentLimit) - index;
                } else if (sortBy === 'id' && sortOrder === 'ASC') { // Handle ascending ID sort for index
                    itemIndex = (currentDisplayPage - 1) * currentLimit + index + 1;
                    // If you want to display the actual ID, you'd use log.id here instead of a calculated index
                } else {
                    itemIndex = (currentDisplayPage - 1) * currentLimit + index + 1;
                }
                // Use full URL with fragment if available, otherwise fall back to request_url
                const requestURLString = (log.request_full_url_with_fragment && log.request_full_url_with_fragment.Valid && log.request_full_url_with_fragment.String)
                                           ? log.request_full_url_with_fragment.String
                                           : (log.request_url?.String || '');
                // Log the log object to inspect its contents, especially request_full_url_with_fragment
                console.log('[ProxyLogView] Log object for list view (ID ' + log.id + '):', JSON.parse(JSON.stringify(log)));
                const safeURL = escapeHtml(requestURLString);
                const pageNameDisplay = (log.page_sitemap_id?.Valid && log.page_sitemap_name?.Valid && log.page_sitemap_name.String)
                    ? `<a href="#page-sitemap?page_id=${log.page_sitemap_id.Int64}" title="View Page: ${escapeHtmlAttribute(log.page_sitemap_name.String)}">${escapeHtml(log.page_sitemap_name.String)}</a>`
                    : '-';
                const logSourceDisplay = log.log_source?.Valid ? escapeHtml(log.log_source.String) : 'mitmproxy';

                const ts = log.timestamp ? new Date(log.timestamp).toLocaleString() : 'N/A';
                tableHTML += `<tr>`;
                displayableHeaders.forEach(h => {
                    const colDef = columnDefinitions[h.key];
                    const savedColSetting = savedColumnSettings[h.key];
                    const isVisible = savedColSetting ? savedColSetting.visible : (colDef ? colDef.visible : true);

                    if (!isVisible) return;

                    switch(h.key) {
                        case 'index': tableHTML += `<td>${itemIndex}</td>`; break;
                        case 'timestamp': tableHTML += `<td>${ts}</td>`; break;
                        case 'method': tableHTML += `<td>${escapeHtml(log.request_method?.String || '')}</td>`; break; // Ensure .String is accessed
                        case 'page_name': tableHTML += `<td>${pageNameDisplay}</td>`; break;
                        case 'url':
                            const displayURL = safeURL.length > 256 ? safeURL.substring(0, 253) + '...' : safeURL;
                            tableHTML += `<td class="proxy-log-url-cell" title="${safeURL}">${displayURL}</td>`;
                            break;
                        case 'status': tableHTML += `<td>${log.response_status_code || '-'}</td>`; break;
                        case 'type':
                            tableHTML += `<td title="${escapeHtmlAttribute(log.response_content_type?.String || '-')}">` +
                                         `${escapeHtml(log.response_content_type?.Valid && log.response_content_type.String ? log.response_content_type.String.substring(0,30) : '-')}` +
                                         `${log.response_content_type?.Valid && log.response_content_type.String && log.response_content_type.String.length > 30 ? '...' : ''}` +
                                         `</td>`;
                            break;
                        case 'tags':
                            let tagsHTML = log.tags && log.tags.length > 0
                                ? log.tags.map(tag =>
                                    `<span class="tag-chip-table" style="background-color: ${tag.color?.String || '#6c757d'}; color: white; padding: 1px 4px; border-radius: 3px; margin-right: 3px; font-size: 0.8em; display: inline-block;">
                                        ${escapeHtml(tag.name)}
                                    </span>`).join(' ')
                                : '-';
                            tableHTML += `<td>${tagsHTML}</td>`; break;
                        case 'size': tableHTML += `<td>${log.response_body_size || 0}</td>`; break;
                        case 'actions':
                            tableHTML += `<td class="actions-cell proxy-log-actions-column">
                                <span class="favorite-toggle table-row-favorite-toggle ${log.is_favorite ? 'favorited' : ''}" data-log-id="${log.id}" data-is-favorite="${log.is_favorite ? 'true' : 'false'}" title="Toggle Favorite" style="cursor: pointer; margin-right: 8px; font-size: 1.2em; vertical-align: middle;">${log.is_favorite ? '★' : '☆'}</span>
                                <button class="action-button view-log-detail" data-log-id="${log.id}" title="View Details">👁️</button>
                                <button class="action-button more-actions" data-log-id="${log.id}" data-log-method="${escapeHtmlAttribute(log.request_method?.String || '')}" data-log-path="${escapeHtmlAttribute((log.request_url?.String || '').split('?')[0])}" title="More Actions">⋮</button>
                            </td>`;
                            break;
                    }
                });
                tableHTML += `</tr>`;
            });

            tableHTML += `</tbody></table>`;
            listDiv.innerHTML = tableHTML;
            // For debugging, you can uncomment the next line to see the HTML being set:
            // console.log("[ProxyLogView] listDiv.innerHTML was set. First 500 chars:", listDiv.innerHTML.substring(0, 500));
        } else {
            listDiv.innerHTML = `<p>No proxy logs found for target ${escapeHtml(currentTargetName)} (ID: ${currentTargetId}) with current filters.</p>`;
        }
        renderProxyLogPagination(paginationControlsDiv);

        // Using requestAnimationFrame for DOM updates to ensure elements are ready
        requestAnimationFrame(() => {
            const tableHeadElement = document.getElementById('proxyLogTableHead');
            console.log("[ProxyLogView] Inside requestAnimationFrame, tableHeadElement found:", tableHeadElement ? "Yes" : "No");

            if (tableHeadElement) {
                tableHeadElement.querySelectorAll('th.sortable').forEach(th => th.addEventListener('click', handleProxyLogSort));
                tableHeadElement.querySelectorAll('select.proxy-log-filter').forEach(select => {
                    select.addEventListener('change', handleProxyLogFilterChange);
                    // Prevent clicks on the select from bubbling up to the TH and triggering a sort
                    select.addEventListener('click', (event) => {
                        event.stopPropagation();
                    });
                });

                if (tableService) {
                    // Pass columnDefinitions to makeTableColumnsResizable
                    // so it knows which columns are non-resizable
                    const currentColumnDefinitions = stateService.getState().paginationState.proxyLogTableLayout;
                    tableService.makeTableColumnsResizable('proxyLogTableHead', currentColumnDefinitions);
                }
            } else if (logs.length > 0) {
                console.error("[ProxyLogView] Table head 'proxyLogTableHead' not found after rendering table (using requestAnimationFrame).");
            }

            listDiv.querySelectorAll('.view-log-detail').forEach(button => button.addEventListener('click', handleViewLogDetail));
            listDiv.querySelectorAll('.table-row-favorite-toggle').forEach(starBtn => starBtn.addEventListener('click', handleProxyLogFavoriteToggle));
            listDiv.querySelectorAll('.more-actions').forEach(button => button.addEventListener('click', openMoreActionsDropdown));
        });

    } catch (error) {
        listDiv.innerHTML = `<p class="error-message">Error loading proxy logs: ${escapeHtml(error.message)}</p>`;
        console.error('Error fetching proxy logs:', error);
        paginationControlsDiv.innerHTML = '';
    }
}

function closeMoreActionsDropdown() {
    const existingDropdown = document.getElementById('proxyLogMoreActionsDropdown');
    if (existingDropdown) {
        existingDropdown.remove();
    }
    document.removeEventListener('click', closeMoreActionsDropdownOnClickOutside);
}

function closeMoreActionsDropdownOnClickOutside(event) {
    const dropdown = document.getElementById('proxyLogMoreActionsDropdown');
    const moreActionsButton = event.target.closest('.more-actions');
    if (dropdown && !dropdown.contains(event.target) && !moreActionsButton) {
        closeMoreActionsDropdown();
    }
}

function openMoreActionsDropdown(event) {
    event.stopPropagation(); // Prevent click from immediately closing due to document listener
    closeMoreActionsDropdown(); // Close any existing dropdown

    const button = event.currentTarget;
    const logId = button.getAttribute('data-log-id');
    // const logMethod = button.getAttribute('data-log-method'); // For future use if needed
    // const logPath = button.getAttribute('data-log-path');   // For future use if needed

    const dropdown = document.createElement('div');
    dropdown.id = 'proxyLogMoreActionsDropdown';
    dropdown.className = 'actions-dropdown-menu'; // Add a class for styling

    const rect = button.getBoundingClientRect();
    dropdown.style.top = `${rect.bottom + window.scrollY}px`;
    dropdown.style.left = `${rect.left + window.scrollX - (dropdown.offsetWidth > button.offsetWidth ? dropdown.offsetWidth - button.offsetWidth : 0)}px`; // Adjust left position

    dropdown.innerHTML = `
        <ul>
            <li><a href="#proxy-log-detail?id=${logId}" data-action="view-detail">View Details</a></li>
            <li><a href="#proxy-log-detail?id=${logId}&tab=jsAnalysisTab" data-action="analyze-js">Analyze JS</a></li>
            <li><a href="#" data-action="find-comments-dropdown" data-log-id="${logId}">Find Comments</a></li>
            <li><a href="#" data-action="send-to-findings" data-log-id="${logId}">Send to Findings (TBD)</a></li>
            <li><a href="#" data-action="send-to-modifier-dropdown" data-log-id="${logId}">Send to Modifier</a></li>
            <li><a href="#" data-action="add-as-finding" data-log-id="${logId}">Add as Finding</a></li>
        </ul>
    `;

    document.body.appendChild(dropdown);

    // Adjust left position after rendering to get actual width
    dropdown.style.left = `${rect.left + window.scrollX - (dropdown.offsetWidth > button.offsetWidth ? dropdown.offsetWidth - button.offsetWidth : 0) + (button.offsetWidth / 2) - (dropdown.offsetWidth / 2)}px`;
    if ((rect.left + window.scrollX + dropdown.offsetWidth) > window.innerWidth) {
        dropdown.style.left = `${window.innerWidth - dropdown.offsetWidth - 5}px`; // Adjust if it overflows right
    }
    if (rect.left + window.scrollX - dropdown.offsetWidth < 0 && (rect.left + window.scrollX - (dropdown.offsetWidth > button.offsetWidth ? dropdown.offsetWidth - button.offsetWidth : 0) + (button.offsetWidth / 2) - (dropdown.offsetWidth / 2)) < 0) {
         dropdown.style.left = '5px'; // Adjust if it overflows left
    }

    dropdown.querySelector('a[data-action="find-comments-dropdown"]')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleFindCommentsFromDropdown(logId);
        closeMoreActionsDropdown();
    });

    dropdown.querySelector('a[data-action="add-as-finding"]')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleLogAddAsFinding(logId); // New handler
        closeMoreActionsDropdown();
    });

    // Placeholder for "Send to Findings"
    dropdown.querySelector('a[data-action="send-to-findings"]').addEventListener('click', (e) => {
        e.preventDefault();
        uiService.showModalMessage("Not Implemented", `Sending log ID ${logId} to Findings is not yet implemented.`);
        closeMoreActionsDropdown();
    });

    // Add event listener for "Send to Modifier"
    dropdown.querySelector('a[data-action="send-to-modifier-dropdown"]')?.addEventListener('click', async (e) => {
        e.preventDefault();
        await handleSendLogToModifier(logId); // You'll need to create or adapt this function
        closeMoreActionsDropdown();
    });

    // Add a slight delay before attaching the outside click listener
    // to prevent it from firing immediately from the same click that opened it.
    setTimeout(() => document.addEventListener('click', closeMoreActionsDropdownOnClickOutside), 0);
}

// --- Functions for Proxy Log Detail View More Actions Dropdown ---
function closeDetailViewMoreActionsDropdown() {
    const existingDropdown = document.getElementById('proxyLogDetailMoreActionsDropdown');
    if (existingDropdown) {
        existingDropdown.remove();
    }
    document.removeEventListener('click', closeDetailViewMoreActionsDropdownOnClickOutside);
}

function closeDetailViewMoreActionsDropdownOnClickOutside(event) {
    const dropdown = document.getElementById('proxyLogDetailMoreActionsDropdown');
    const moreActionsButton = event.target.closest('#proxyLogDetailMoreActionsBtn');

    console.log('[ProxyLogView] closeDetailViewMoreActionsDropdownOnClickOutside called.');
    console.log('  - event.target:', event.target);
    console.log('  - dropdown exists:', !!dropdown);
    if (dropdown) {
        console.log('  - dropdown.contains(event.target):', dropdown.contains(event.target));
    }
    console.log('  - moreActionsButton (closest #proxyLogDetailMoreActionsBtn):', moreActionsButton);

    if (dropdown && !dropdown.contains(event.target) && !moreActionsButton) {
        console.log('  - Condition MET. Closing dropdown.');
        closeDetailViewMoreActionsDropdown();
    } else {
        console.log('  - Condition NOT MET. Not closing dropdown.');
    }
}

function openDetailViewMoreActionsDropdown(event, logId) {
    event.stopPropagation();
    closeDetailViewMoreActionsDropdown(); // Close any existing detail view dropdown

    const button = event.currentTarget;
    const dropdown = document.createElement('div');

    dropdown.id = 'proxyLogDetailMoreActionsDropdown';
    dropdown.className = 'actions-dropdown-menu'; // Use existing styling

    const rect = button.getBoundingClientRect();
    dropdown.style.top = `${rect.bottom + window.scrollY}px`;
    // Position it to the left, aligning its right edge with the button's right edge, or slightly offset
    dropdown.style.left = `${rect.left + window.scrollX}px`;

    dropdown.innerHTML = `
        <ul>
            <li><a href="#" data-action="analyze-js-detail" data-log-id="${logId}">Analyze JS</a></li>
            <li><a href="#" data-action="find-comments-detail" data-log-id="${logId}">Find Comments</a></li>
            <li><a href="#" data-action="send-to-modifier-detail" data-log-id="${logId}">Send to Modifier</a></li>
            <li><a href="#" data-action="add-as-finding-detail" data-log-id="${logId}">Add as Finding</a></li>
        </ul>
    `;

    document.body.appendChild(dropdown);

    // Event listeners for dropdown items
    dropdown.querySelector('a[data-action="analyze-js-detail"]').addEventListener('click', (e) => {
        e.preventDefault();
        document.querySelector('.tab-button[data-tab="jsAnalysisTab"]')?.click(); // Switch tab
        closeDetailViewMoreActionsDropdown();
    });
    dropdown.querySelector('a[data-action="find-comments-detail"]').addEventListener('click', (e) => {
        e.preventDefault();
        document.querySelector('.tab-button[data-tab="commentsTab"]')?.click(); // Switch tab
        closeDetailViewMoreActionsDropdown();
    });
    dropdown.querySelector('a[data-action="send-to-modifier-detail"]').addEventListener('click', async (e) => {
        e.preventDefault();
        await handleSendLogToModifier(logId); // Reuse existing function
        closeDetailViewMoreActionsDropdown();
    });
    dropdown.querySelector('a[data-action="add-as-finding-detail"]')?.addEventListener('click', (e) => {
        e.preventDefault();
        handleLogAddAsFinding(logId); // Reuse the same handler
        closeDetailViewMoreActionsDropdown();
    });

    setTimeout(() => document.addEventListener('click', closeDetailViewMoreActionsDropdownOnClickOutside), 0);
}
// --- End of Functions for Proxy Log Detail View More Actions Dropdown ---

function handleFindCommentsFromDropdown(logId) {
    // Navigate to the detail view and activate the comments tab
    window.location.hash = `#proxy-log-detail?id=${logId}&tab=commentsTab`;
}

function closeAddToSitemapModal() {
    const modal = document.getElementById('addToSitemapModal');
    if (modal) {
        modal.remove();
    }
}

async function handleSaveSitemapEntry() {
    const logId = document.getElementById('sitemapLogId').value;
    const folderPath = document.getElementById('sitemapFolderPath').value.trim();
    const notes = document.getElementById('sitemapNotes').value.trim();

    if (!folderPath) {
        uiService.showModalMessage("Validation Error", "Folder Path cannot be empty.");
        return;
    }

    try {
        // This function will be created in apiService.js in the next step
        await apiService.addSitemapManualEntry(logId, folderPath, notes);
        uiService.showModalMessage("Success", "Entry added to sitemap.");
        closeAddToSitemapModal();
    } catch (error) {
        uiService.showModalMessage("Error", `Failed to add to sitemap: ${escapeHtml(error.message)}`);
    }
}

async function handleDeleteAllTargetLogs() {
    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    if (!currentTargetId) {
        uiService.showModalMessage("Error", "No current target selected to delete logs for.");
        return;
    }

    uiService.showModalConfirm(
        "Confirm Delete Logs",
        `Are you sure you want to delete ALL proxy logs for target "${escapeHtml(currentTargetName)}" (ID: ${currentTargetId})? This action cannot be undone.`,
        async () => {
            try {
                await apiService.deleteProxyLogsForTarget(currentTargetId);
                uiService.showModalMessage("Success", `All proxy logs for target "${escapeHtml(currentTargetName)}" have been deleted.`);
                // Refresh the view - using the current filter/sort params from state
                const currentProxyLogParams = stateService.getState().paginationState.proxyLog;
                fetchAndDisplayProxyLogs(currentProxyLogParams);
            } catch (error) {
                console.error("Error deleting all target logs:", error);
                uiService.showModalMessage("Error", `Failed to delete logs: ${escapeHtml(error.message)}`);
            }
        }
    );
}

async function handleRefreshProxyLog() {
    console.log("[ProxyLogView] Refresh button clicked.");
    const statusMessageEl = document.getElementById('proxyLogRefreshStatusMessage');

    if (statusMessageEl) {
        statusMessageEl.textContent = 'Refreshing logs...';
        statusMessageEl.className = 'message-area info-message'; // Use info for in-progress
        statusMessageEl.style.display = 'inline';
    }

    const currentProxyLogParams = stateService.getState().paginationState.proxyLog;
    await fetchAndDisplayProxyLogs(currentProxyLogParams);

    if (statusMessageEl) {
        statusMessageEl.textContent = 'Logs refreshed!';
        statusMessageEl.className = 'message-area success-message'; // Use success for completion
        setTimeout(() => { statusMessageEl.style.display = 'none'; statusMessageEl.textContent = ''; }, 2000); // Hide after 2 seconds
    }
}

async function triggerAndFetchParamAnalysis(targetId, targetName) {
    const paramAnalysisContentDiv = document.getElementById('paramAnalysisContent');
    if (!paramAnalysisContentDiv) {
        console.error("Parameter analysis content div not found.");
        return;
    }
    paramAnalysisContentDiv.innerHTML = `<p>Running analysis for target ${escapeHtml(targetName)} (ID: ${targetId}). This may take a moment...</p>`;

    try {
        const analysisSummary = await apiService.analyzeTargetParameters(targetId);
        uiService.showModalMessage("Analysis Complete",
            `Scanned: ${analysisSummary.total_logs_scanned} logs.<br>
             Processed: ${analysisSummary.parameterized_urls_processed} URLs with params.<br>
             New Unique Entries: ${analysisSummary.new_unique_entries_found}.<br>
             Now fetching results...`, true);

        // Reset pagination for this view and fetch the first page
        const appState = stateService.getState();
        const currentParamAnalysisState = appState.paginationState.parameterizedUrlsView;
        const newState = {
            ...currentParamAnalysisState,
            currentPage: 1, // Reset to page 1 after analysis
        };
        stateService.updateState({ paginationState: { ...appState.paginationState, parameterizedUrlsView: newState } });
        await displayParameterizedURLs(); // New function to display results

    } catch (error) {
        paramAnalysisContentDiv.innerHTML = `<p class="error-message">Error running or fetching parameter analysis: ${escapeHtml(error.message)}</p>`;
        console.error('Error in triggerAndFetchParamAnalysis:', error);
    }
}

async function displayParameterizedURLs() {
    const paramAnalysisContentDiv = document.getElementById('paramAnalysisContent');
    if (!paramAnalysisContentDiv) return;

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;
    const { currentPage, limit, sortBy, sortOrder, filterRequestMethod, filterPathSearch, filterParamKeysSearch } = appState.paginationState.parameterizedUrlsView;
    const tableKey = 'parameterizedUrlsTable';
    const columnConfig = appState.paginationState.parameterizedUrlsTableLayout; // Corrected path
    const globalTableLayouts = appState.globalTableLayouts || {};
    const savedTableWidths = globalTableLayouts[tableKey] || {};

    paramAnalysisContentDiv.innerHTML = `<p>Loading analyzed parameters for target ${escapeHtml(currentTargetName)}...</p>`;

    try {
        const params = {
            target_id: currentTargetId,
            page: currentPage,
            limit: limit,
            sort_by: sortBy,
            sort_order: sortOrder,
            request_method: filterRequestMethod,
            path_search: filterPathSearch,
            param_keys_search: filterParamKeysSearch,
        };
        const response = await apiService.getParameterizedURLs(params);
        const pUrls = response.records || [];

        stateService.updateState({
            paginationState: {
                ...appState.paginationState,
                parameterizedUrlsView: {
                    ...appState.paginationState.parameterizedUrlsView,
                    currentPage: response.page || 1,
                    totalPages: response.total_pages || 1,
                    totalRecords: response.total_records || 0,
                }
            }
        });

        if (pUrls.length > 0) {
            // Add filter controls here (similar to proxy log, but for path, method, param_keys)
            // For brevity, I'll skip adding full filter input HTML here, but it would be similar to proxyLogView's search/dropdowns.
            let tableHTML = `<div style="margin-bottom:10px; display: flex; justify-content: space-between; align-items: center;">
                <div>
                    <input type="text" id="paramUrlPathSearch" placeholder="Search Path..." value="${escapeHtmlAttribute(filterPathSearch)}" style="margin-right:10px;">
                    <input type="text" id="paramUrlKeysSearch" placeholder="Search Param Keys..." value="${escapeHtmlAttribute(filterParamKeysSearch)}" style="margin-right:10px;">
                    <select id="paramUrlMethodFilter" style="margin-right:10px;">
                        <option value="">All Methods</option>
                        <option value="GET" ${filterRequestMethod === 'GET' ? 'selected' : ''}>GET</option>
                        <option value="POST" ${filterRequestMethod === 'POST' ? 'selected' : ''}>POST</option>
                        <option value="PUT" ${filterRequestMethod === 'PUT' ? 'selected' : ''}>PUT</option>
                        <option value="DELETE" ${filterRequestMethod === 'DELETE' ? 'selected' : ''}>DELETE</option>
                    </select>
                    <button id="applyParamUrlFiltersBtn" class="secondary small-button">Apply Filters</button>
                </div>
                <div>
                    <button id="saveParamUrlsLayoutBtn" class="secondary small-button">Save Column Layout</button>
                </div>
            </div>
            <table style="table-layout: fixed;"><thead id="parameterizedUrlsTableHead"><tr>
                <th style="width: ${savedTableWidths.id || columnConfig.id.default};" class="sortable" data-sort-key="id" data-col-key="id">ID</th>
                <th style="width: ${savedTableWidths.method || columnConfig.method.default};" class="sortable" data-sort-key="request_method" data-col-key="method">Method</th>
                <th style="width: ${savedTableWidths.path || columnConfig.path.default};" class="sortable" data-sort-key="request_path" data-col-key="path">Path</th>
                <th style="width: ${savedTableWidths.param_keys || columnConfig.param_keys.default};" class="sortable" data-sort-key="param_keys" data-col-key="param_keys">Param Keys</th>
                <th style="width: ${savedTableWidths.example_url || columnConfig.example_url.default};" data-col-key="example_url">Example URL</th>
                <th style="width: ${savedTableWidths.discovered || columnConfig.discovered.default};" class="sortable" data-sort-key="discovered_at" data-col-key="discovered">Discovered</th>
                <th style="width: ${savedTableWidths.last_seen || columnConfig.last_seen.default};" class="sortable" data-sort-key="last_seen_at" data-col-key="last_seen">Last Seen</th>
                <th style="width: ${savedTableWidths.actions || columnConfig.actions.default};" data-col-key="actions">Actions</th>
            </tr></thead><tbody>`;

            pUrls.forEach(pUrl => {
                const discovered = new Date(pUrl.discovered_at).toLocaleString();
                const lastSeen = new Date(pUrl.last_seen_at).toLocaleString();
                const fullExampleURL = pUrl.example_full_url?.String || '';
                const displayExampleURL = fullExampleURL.length > 256 ? escapeHtml(fullExampleURL.substring(0, 253)) + '...' : escapeHtml(fullExampleURL);
                tableHTML += `<tr>
                    <td>${pUrl.id}</td>
                    <td>${escapeHtml(pUrl.request_method?.String || '')}</td>
                    <td class="proxy-log-url-cell" title="${escapeHtmlAttribute(pUrl.request_path?.String || '')}">${escapeHtml(pUrl.request_path?.String || '')}</td>
                    <td style="overflow-wrap: break-word; white-space: normal;" title="${escapeHtmlAttribute(pUrl.param_keys)}">${escapeHtml(pUrl.param_keys)}</td>
                    <td class="proxy-log-url-cell" title="${escapeHtmlAttribute(fullExampleURL)}">${displayExampleURL}</td>
                    <td>${discovered}</td>
                    <td>${lastSeen}</td>
                    <td class="actions-cell">
                        <button class="action-button view-log-detail" data-log-id="${pUrl.http_traffic_log_id?.Int64 || ''}" title="View Example Log">👁️</button>
                        <button class="action-button send-to-modifier" data-purl-id="${pUrl.id}" title="Send to Modifier">🔧</button>
                    </td></tr>`;
            });
            tableHTML += `</tbody></table>`;
            paramAnalysisContentDiv.innerHTML = tableHTML; // Replace previous content

            // Add event listeners for table
            const tableHead = document.getElementById('parameterizedUrlsTableHead');
            if (tableHead) {
                tableHead.querySelectorAll('th.sortable').forEach(th => th.addEventListener('click', handleParamUrlSort));
                if (tableService) tableService.makeTableColumnsResizable('parameterizedUrlsTableHead');
            }
            paramAnalysisContentDiv.querySelectorAll('.view-log-detail').forEach(btn => {
                btn.addEventListener('click', handleViewLogDetail); // Reuse existing detail view handler
            });
            paramAnalysisContentDiv.querySelectorAll('.send-to-modifier').forEach(btn => {
                btn.addEventListener('click', handleSendToModifier);
            });
            document.getElementById('applyParamUrlFiltersBtn')?.addEventListener('click', applyParamUrlFilters);
            document.getElementById('saveParamUrlsLayoutBtn')?.addEventListener('click', () => {
                if (tableService) {
                    tableService.saveCurrentTableLayout('parameterizedUrlsTable', 'parameterizedUrlsTableHead');
                }
            });
            renderParamUrlPagination(document.getElementById('paramUrlPaginationControls')); // Need a container for this
            paramAnalysisContentDiv.insertAdjacentHTML('beforeend', '<div id="paramUrlPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>');
            renderParamUrlPagination(document.getElementById('paramUrlPaginationControls'));
        } else {
            paramAnalysisContentDiv.innerHTML = `<p>No parameterized URLs found for target ${escapeHtml(currentTargetName)} with current filters.</p>`;
        }
    } catch (error) {
        paramAnalysisContentDiv.innerHTML = `<p class="error-message">Error displaying parameterized URLs: ${escapeHtml(error.message)}</p>`;
        console.error('Error in displayParameterizedURLs:', error);
    }
}

function handleParamUrlSort(event) {
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) return;
    const newSortBy = event.target.closest('th').getAttribute('data-sort-key');
    if (!newSortBy) return;

    const appState = stateService.getState();
    const currentParamViewState = appState.paginationState.parameterizedUrlsView;
    let newSortOrder = 'ASC';
    if (currentParamViewState.sortBy === newSortBy) {
        newSortOrder = currentParamViewState.sortOrder === 'ASC' ? 'DESC' : 'ASC';
    }
    stateService.updateState({
        paginationState: {
            ...appState.paginationState,
            parameterizedUrlsView: { ...currentParamViewState, sortBy: newSortBy, sortOrder: newSortOrder, currentPage: 1 }
        }
    });
    displayParameterizedURLs();
}

function applyParamUrlFilters() {
    const appState = stateService.getState();
    const currentParamViewState = appState.paginationState.parameterizedUrlsView;
    const newFilterPath = document.getElementById('paramUrlPathSearch')?.value || '';
    const newFilterKeys = document.getElementById('paramUrlKeysSearch')?.value || '';
    const newFilterMethod = document.getElementById('paramUrlMethodFilter')?.value || '';

    stateService.updateState({
        paginationState: {
            ...appState.paginationState,
            parameterizedUrlsView: {
                ...currentParamViewState,
                filterPathSearch: newFilterPath,
                filterParamKeysSearch: newFilterKeys,
                filterRequestMethod: newFilterMethod,
                currentPage: 1
            }
        }
    });
    displayParameterizedURLs();
}

async function handleSendLogToModifier(logId) {
    if (!logId) {
        uiService.showModalMessage("Error", "Log ID not found for sending to Modifier.");
        return;
    }
    uiService.showModalMessage("Sending...", `Sending log ID ${logId} to Modifier...`, true, 1000);
    try {
        // The addModifierTask API expects an object, even if just with http_traffic_log_id
        const newTask = await apiService.addModifierTask({ http_traffic_log_id: parseInt(logId, 10) });
        uiService.showModalMessage("Sent to Modifier", `Task "${escapeHtml(newTask.name || `Task ${newTask.id}`)}" created from log ${logId}. Navigating...`, true, 2000);
        // Navigate to the modifier view with the new task ID
        window.location.hash = `#modifier?task_id=${newTask.id}`;
    } catch (error) {
        console.error("Error sending log to modifier:", error);
        uiService.showModalMessage("Error", `Failed to send log to Modifier: ${escapeHtml(error.message)}`);
    }
}


async function handleSendToModifier(event) {
    const button = event.currentTarget;
    const pUrlId = button.dataset.purlId;
    if (!pUrlId) {
        uiService.showModalMessage("Error", "Parameterized URL ID not found.");
        return;
    }

    // Find the pUrl data from the currently displayed list (or re-fetch if necessary)
    // For simplicity, we'll assume it's in the current view's data if the table was just rendered.
    // A more robust way might be to fetch details by pUrlId if not readily available.
    const appState = stateService.getState();
    // This is a simplification. Ideally, you'd fetch the pURL details by its ID
    // or have it stored in a way that's easily accessible.
    // For now, we'll just send the ID and let the modifier view fetch details.

    try {
        // In the future, this would send more details like method, path, example_url
        const newTask = await apiService.addModifierTask({ parameterized_url_id: parseInt(pUrlId, 10) });
        uiService.showModalMessage("Sent to Modifier", `Task "${escapeHtml(newTask.name || `Task ${newTask.id}`)}" sent to Modifier. Navigating...`, true, 1500);
        window.location.hash = `#modifier?task_id=${newTask.id}`;
    } catch (error) {
        console.error("Error sending item to modifier:", error);
        uiService.showModalMessage("Error", `Failed to send to Modifier: ${escapeHtml(error.message)}`);
    }
}


function renderParamUrlPagination(container) {
    if (!container) return;
    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords } = appState.paginationState.parameterizedUrlsView;

    let paginationHTML = '';
    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total unique parameterized URL(s) found.</p>` : '';
        return;
    }
    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total entries)</p>`;

    const prevButton = document.createElement('button');
    prevButton.className = 'secondary'; prevButton.style.marginRight = '5px';
    prevButton.innerHTML = '&laquo; Previous';
    if (currentPage <= 1) prevButton.disabled = true;
    prevButton.addEventListener('click', () => {
        if (currentPage > 1) {
            const s = stateService.getState();
            stateService.updateState({ paginationState: { ...s.paginationState, parameterizedUrlsView: {...s.paginationState.parameterizedUrlsView, currentPage: currentPage - 1}}});
            displayParameterizedURLs();
        }
    });

    const nextButton = document.createElement('button');
    nextButton.className = 'secondary';
    nextButton.innerHTML = 'Next &raquo;';
    if (currentPage >= totalPages) nextButton.disabled = true;
    nextButton.addEventListener('click', () => {
        if (currentPage < totalPages) {
            const s = stateService.getState();
            stateService.updateState({ paginationState: { ...s.paginationState, parameterizedUrlsView: {...s.paginationState.parameterizedUrlsView, currentPage: currentPage + 1}}});
            displayParameterizedURLs();
        }
    });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    if (currentPage > 1) container.appendChild(prevButton);
    if (currentPage < totalPages) container.appendChild(nextButton);
}

/**
 * Loads the main proxy log view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export function loadProxyLogView(mainViewContainer, proxyLogParams = null) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadProxyLogView!");
        return;
    }
    console.log("[ProxyLogView] loadProxyLogView called with params:", proxyLogParams);
    if (!apiService || !uiService || !stateService || !tableService) {
        console.error("ProxyLogView not initialized. Call initProxyLogView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>ProxyLogView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;
    // Use passed params for initial render of controls if available, else global state
    const activeParams = proxyLogParams || appState.paginationState.proxyLog;
    const { filterFavoritesOnly, filterSearchText, analysis_type } = activeParams;
    const tableKey = 'proxyLogTable';

    const targetInfo = currentTargetId ? `for Target: ${escapeHtml(currentTargetName)} (ID: ${currentTargetId})` : '(No Target Selected)';
    viewContentContainer.innerHTML = `
        <h1>Proxy Log ${targetInfo}</h1>

        <div class="tabs">
            <button class="tab-button" data-tab="allLogsTab">All Logs</button>
            <button class="tab-button" data-tab="paramAnalysisTab">Parameter Analysis</button>
        </div>

        <div id="allLogsTab" class="tab-content">
            <div style="margin-top:15px; margin-bottom: 15px; display: flex; align-items: center; gap: 20px;">
                <div class="form-group" style="display: flex; align-items: center; margin-bottom: 0;">
                    <input type="checkbox" id="filterFavoritesToggle" style="margin-right: 5px;" ${filterFavoritesOnly ? 'checked' : ''}>
                    <label for="filterFavoritesToggle" style="font-weight: normal;">Favorites Only</label>
                </div>
                <div class="form-group" style="display: flex; align-items: center; margin-left: 20px; margin-bottom: 0;">
                    <input type="checkbox" id="defaultToResponseTabToggle" style="margin-right: 5px;">
                    <label for="defaultToResponseTabToggle" style="font-weight: normal;">Default to Response Tab in Detail View</label>
                </div>
                <div class="form-group" style="flex-grow: 1; margin-bottom: 0;">
                    <input type="search" id="proxyLogSearchInput" placeholder="Search URL, Headers, Body..." value="${escapeHtmlAttribute(filterSearchText)}" style="width: 100%; padding: 6px 10px; border-radius: 4px; border: 1px solid #bdc3c7;">
                </div>
            </div>
            <div id="proxyLogTagFilterContainer" style="margin-top: 10px; margin-bottom: 10px;"></div>
            <div style="margin-bottom: 10px;">
                <button id="refreshProxyLogBtn" class="secondary small-button" title="Refresh Logs" style="margin-right: 10px;">🔄</button>
                <button id="saveProxyLogLayoutBtn" class="secondary small-button" style="margin-right: 10px;">Save Column Layout</button>
                <button id="customizeProxyLogColumnsBtn" class="secondary small-button" style="margin-right: 10px;">Customize Columns</button>
                <button id="deleteAllTargetLogsBtn" class="secondary small-button" ${!currentTargetId ? 'disabled title="No target selected"' : `title="Delete all logs for ${escapeHtml(currentTargetName)}"`}>
                    Delete All Logs for Target
                </button>
                <span id="proxyLogRefreshStatusMessage" class="message-area" style="margin-left: 10px; display: none;"></span>
            </div>
            <div id="proxyLogListContainer">Loading proxy logs...</div>
            <div id="proxyLogPaginationControlsContainer" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
        </div>

        <div id="paramAnalysisTab" class="tab-content">
            <h3 style="margin-top:15px;">Logs with URL Parameters</h3>
            <div style="margin-bottom: 15px;">
                <button id="runParamAnalysisBtn" class="primary">Run/Refresh Analysis</button>
            </div>
            <div id="paramAnalysisContent"><p>Loading parameter analysis...</p></div>
        </div>
    `;

    if (!currentTargetId) {
        const allLogsContent = document.getElementById('allLogsTab');
        if (allLogsContent) allLogsContent.innerHTML = '<p style="margin-top:15px;">Please set a current target to view its proxy log.</p>';

        const paramAnalysisContent = document.getElementById('paramAnalysisTab');
        if (paramAnalysisContent) paramAnalysisContent.innerHTML = '<p style="margin-top:15px;">Please set a current target to perform analysis.</p>';

        // Also disable the run button if no target
        const runBtn = document.getElementById('runParamAnalysisBtn');
        if (runBtn) runBtn.disabled = true;

        return;
    }

    // Tab switching logic
    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.addEventListener('click', (e) => {
            const tabId = e.currentTarget.dataset.tab;
            document.querySelectorAll('.tabs .tab-button').forEach(btn => btn.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
            e.currentTarget.classList.add('active');
            document.getElementById(tabId).classList.add('active');

            // Update hash without triggering full reload if only tab changes
            const currentHash = window.location.hash.split('?')[0];
            const currentParams = new URLSearchParams(window.location.hash.split('?')[1] || '');
            currentParams.set('analysis_type', tabId === 'paramAnalysisTab' ? 'params' : '');
            window.location.hash = `${currentHash}?${currentParams.toString().replace(/analysis_type=&|&analysis_type=$/, '')}`; // Clean up empty analysis_type
        });
    });

    // Initial content load based on analysis_type.
    // At this point, currentTargetId is guaranteed to be truthy due to the check above.
    console.log("[ProxyLogView] loadProxyLogView - activeParams for tab decision:", JSON.parse(JSON.stringify(activeParams)));

    // Deactivate all tabs first to ensure a clean state for initial load
    document.querySelectorAll('.tabs .tab-button').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));

    if (analysis_type === 'params') {
        console.log("[ProxyLogView] CONDITION MET: analysis_type IS 'params'. Activating 'Parameter Analysis' tab.");
        document.querySelector('.tab-button[data-tab="paramAnalysisTab"]')?.classList.add('active');
        document.getElementById('paramAnalysisTab')?.classList.add('active');
        displayParameterizedURLs(); // Directly call, currentTargetId is confirmed
    } else {
        console.log(`[ProxyLogView] CONDITION NOT MET: analysis_type is "${analysis_type}" (type: ${typeof analysis_type}). Activating 'All Logs' tab.`);
        document.querySelector('.tab-button[data-tab="allLogsTab"]')?.classList.add('active');
        document.getElementById('allLogsTab')?.classList.add('active');
        fetchAndDisplayProxyLogs(activeParams); // For "All Logs" tab, currentTargetId is confirmed
    }

    // Add event listener for the new "Run Parameter Analysis" button
    const runParamAnalysisBtn = document.getElementById('runParamAnalysisBtn');
    // currentTargetId is confirmed if we reach here, so no need to check it again for the listener
    if (runParamAnalysisBtn) {
        runParamAnalysisBtn.addEventListener('click', () => {
            triggerAndFetchParamAnalysis(currentTargetId, currentTargetName);
        });
    }
    document.getElementById('filterFavoritesToggle')?.addEventListener('change', handleProxyLogFavoriteFilterChange);
    document.getElementById('proxyLogSearchInput')?.addEventListener('input', debounce(handleProxyLogSearch, 300));
    document.getElementById('saveProxyLogLayoutBtn')?.addEventListener('click', () => {
        prepareAndSaveProxyLogLayout();
    });
    const deleteAllBtn = document.getElementById('deleteAllTargetLogsBtn');
    if (deleteAllBtn) {
        deleteAllBtn.addEventListener('click', handleDeleteAllTargetLogs);
    }
    const refreshBtn = document.getElementById('refreshProxyLogBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', handleRefreshProxyLog); // Listener remains the same
    }
    const customizeColsBtn = document.getElementById('customizeProxyLogColumnsBtn');
    if (customizeColsBtn) {
        customizeColsBtn.addEventListener('click', openCustomizeColumnsModal);
    }

    // Listener for the new "Default to Response Tab" toggle
    const defaultToResponseToggle = document.getElementById('defaultToResponseTabToggle');
    if (defaultToResponseToggle) {
        defaultToResponseToggle.checked = localStorage.getItem('proxyLogDefaultToResponseTab') === 'true';
        defaultToResponseToggle.addEventListener('change', (event) => {
            localStorage.setItem('proxyLogDefaultToResponseTab', event.target.checked);
        });
    }
}

async function handleAnalyzeJS(event) {
    const button = event.target;
    const logIdStr = button.getAttribute('data-log-id');
    const resultsContentDiv = document.getElementById('jsAnalysisResultsContent');

    if (!logIdStr || !resultsContentDiv) {
        console.error("AnalyzeJS: Log ID or results container not found.");
        if (resultsContentDiv) resultsContentDiv.innerHTML = `<p class="error-message">Error: Could not get log ID or results container for analysis.</p>`;
        return;
    }

    document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
    document.querySelector('.tab-button[data-tab="jsAnalysisTab"]')?.classList.add('active');
    document.getElementById('jsAnalysisTab')?.classList.add('active');
    resultsContentDiv.innerHTML = `<p>Analyzing response body for log entry #${logIdStr}...</p>`;

    try {
        const responseData = await apiService.analyzeJsLinks(parseInt(logIdStr, 10));
        resultsContentDiv.innerHTML = ''; // Clear loading message before adding new content
        let currentJsAnalysisData = [];

        if (responseData.message) {
            const p = document.createElement('p');
            p.className = 'message-area info-message';
            p.innerHTML = escapeHtml(responseData.message);
            resultsContentDiv.appendChild(p); // Append message first
        }
        if (responseData.results && Object.keys(responseData.results).length > 0) {
            for (const category in responseData.results) {
                if (responseData.results[category].length > 0) {
                    responseData.results[category].forEach(item => currentJsAnalysisData.push({ category, finding: item }));
                }
            }
        }
        stateService.updateState({ jsAnalysisDataCache: { [logIdStr]: currentJsAnalysisData } });

        // Populate filter and render table
        populateJsAnalysisCategoryFilter(logIdStr); // Populate dropdown with new categories
        renderJsAnalysisTable(logIdStr); // This will now use filters from state

        if (currentJsAnalysisData.length === 0 && !responseData.message) {
             // If no data and no specific message, show a generic "no items" message
            const noItemsP = document.createElement('p');
            noItemsP.textContent = "No specific items extracted by the analysis tool.";
            resultsContentDiv.appendChild(noItemsP);
        }

    } catch (error) {
        resultsContentDiv.innerHTML = `<p class="error-message">Error analyzing log #${logIdStr}: ${escapeHtml(error.message)}</p>`;
    }
}

function populateJsAnalysisCategoryFilter(logIdStr) {
    const filterSelect = document.getElementById('jsAnalysisCategoryFilter');
    if (!filterSelect) return;

    const appState = stateService.getState();
    const analysisData = appState.jsAnalysisDataCache[logIdStr];
    const currentFilterCategory = appState.jsAnalysisFilterCategory;

    // Clear existing options except "All"
    while (filterSelect.options.length > 1) {
        filterSelect.remove(1);
    }

    if (analysisData && analysisData.length > 0) {
        const categories = [...new Set(analysisData.map(item => item.category))].sort();
        categories.forEach(category => {
            const option = document.createElement('option');
            option.value = category;
            option.textContent = escapeHtml(category);
            filterSelect.appendChild(option);
        });
    }
    filterSelect.value = currentFilterCategory; // Set to current filter from state
}


function renderJsAnalysisTable(logIdStr) {
    const resultsContentDiv = document.getElementById('jsAnalysisResultsContent');
    const filterSelect = document.getElementById('jsAnalysisCategoryFilter');
    const searchInput = document.getElementById('jsAnalysisSearchInput');
    const pathSenderControls = document.getElementById('jsAnalysisPathSenderControls');

    if (!resultsContentDiv) return;

    const appState = stateService.getState();
    const currentLogAnalysisData = appState.jsAnalysisDataCache[logIdStr];
    const { sortBy, sortOrder } = appState.jsAnalysisSortState;
    const filterCategory = appState.jsAnalysisFilterCategory;
    const searchText = appState.jsAnalysisSearchText.toLowerCase();

    // Preserve existing message if any (e.g., from handleAnalyzeJS)
    let existingMessageHTML = resultsContentDiv.querySelector('.message-area')?.outerHTML || '';

    // Update control values from state
    if (filterSelect) filterSelect.value = filterCategory;
    if (searchInput) searchInput.value = appState.jsAnalysisSearchText;

    // Show path sender controls only if "Potential Paths (Regex)" is selected or no category is selected (All)
    // And only if there's a current target set
    const currentTargetId = stateService.getState().currentTargetId;
    if (pathSenderControls) {
         pathSenderControls.style.display = (currentTargetId && (!filterCategory || filterCategory === "Potential Paths (Regex)")) ? 'flex' : 'none';
    }


    if (!currentLogAnalysisData) {
        resultsContentDiv.innerHTML = existingMessageHTML + "<p>No analysis data available for this log entry.</p>";
        return;
    }

    let processedData = [...currentLogAnalysisData];

    // Apply category filter
    if (filterCategory) {
        processedData = processedData.filter(item => item.category === filterCategory);
    }

    // Apply search text filter
    if (searchText) {
        processedData = processedData.filter(item =>
            item.category.toLowerCase().includes(searchText) ||
            item.finding.toLowerCase().includes(searchText)
        );
    }

    if (processedData.length === 0) {
        resultsContentDiv.innerHTML = existingMessageHTML + "<p>No analysis data matches the current filters.</p>";
        return;
    }


    const sortedData = processedData.sort((a, b) => {
        const valA = a[sortBy];
        const valB = b[sortBy];
        let comparison = 0;
        if (valA > valB) comparison = 1;
        else if (valA < valB) comparison = -1;
        return sortOrder === 'ASC' ? comparison : comparison * -1;
    });

    let tableHTML = `<table><thead><tr>`;
    // Add checkbox header if "Potential Paths (Regex)" is visible
    if (!filterCategory || filterCategory === "Potential Paths (Regex)") {
        tableHTML += `<th style="width: 30px;"><input type="checkbox" id="selectAllJsPathsCheckbox" title="Select/Deselect All Visible Paths"></th>`;
    }
    tableHTML += `<th class="sortable" data-sort-key="category">Category</th>
                  <th class="sortable" data-sort-key="finding">Finding</th>
                  </tr></thead><tbody>`;

    sortedData.forEach(item => {
        tableHTML += `<tr>`;
        if (!filterCategory || filterCategory === "Potential Paths (Regex)") {
            // Only add checkbox if the item is actually from the "Potential Paths (Regex)" category
            const checkboxHTML = item.category === "Potential Paths (Regex)" ? `<input type="checkbox" class="js-path-checkbox" data-path="${escapeHtmlAttribute(item.finding)}">` : '';
            tableHTML += `<td>${checkboxHTML}</td>`;
        }
        tableHTML += `<td>${escapeHtml(item.category)}</td><td>${escapeHtml(item.finding)}</td></tr>`;
    });
    tableHTML += `</tbody></table>`;

    // Prepend existing message, then add table
    resultsContentDiv.innerHTML = existingMessageHTML + tableHTML;


    resultsContentDiv.querySelectorAll('th.sortable').forEach(th => {
        th.classList.toggle('sorted-asc', sortBy === th.dataset.sortKey && sortOrder === 'ASC');
        th.classList.toggle('sorted-desc', sortBy === th.dataset.sortKey && sortOrder === 'DESC');
        th.addEventListener('click', (event) => handleJsAnalysisSort(event, logIdStr));
    });

    // Add event listener for "Select All" checkbox if it exists
    const selectAllCheckbox = document.getElementById('selectAllJsPathsCheckbox');
    if (selectAllCheckbox) {
        selectAllCheckbox.addEventListener('change', handleSelectAllJsPaths);
    }

}

function handleJsAnalysisSort(event, logIdStr) {
    const newSortBy = event.target.dataset.sortKey;
    const appState = stateService.getState();
    let newSortOrder = 'ASC';
    if (appState.jsAnalysisSortState.sortBy === newSortBy) {
        newSortOrder = appState.jsAnalysisSortState.sortOrder === 'ASC' ? 'DESC' : 'ASC';
    }
    stateService.updateState({ jsAnalysisSortState: { sortBy: newSortBy, sortOrder: newSortOrder } });
    renderJsAnalysisTable(logIdStr);
}

function handleJsAnalysisCategoryFilter(event) {
    const logIdStr = event.target.dataset.logId;
    const newCategory = event.target.value;
    stateService.updateState({ jsAnalysisFilterCategory: newCategory });
    renderJsAnalysisTable(logIdStr);
}

const handleJsAnalysisSearch = debounce((event) => {
    const logIdStr = event.target.dataset.logId;
    const newSearchText = event.target.value;
    stateService.updateState({ jsAnalysisSearchText: newSearchText });
    renderJsAnalysisTable(logIdStr);
}, 300);

function handleSelectAllJsPaths(event) {
    const isChecked = event.target.checked;
    document.querySelectorAll('.js-path-checkbox').forEach(checkbox => {
        checkbox.checked = isChecked;
    });
}


function convertJsAnalysisToCSV(jsonData) {
    const headersConfig = [{ key: 'category', label: 'Category' }, { key: 'finding', label: 'Finding' }];
    const headerRow = headersConfig.map(h => escapeHtml(h.label)).join(',');
    const dataRows = jsonData.map(item => headersConfig.map(header => escapeHtml(item[header.key])).join(','));
    return [headerRow].concat(dataRows).join('\n');
}

function handleExportJsAnalysisToCSV(event) {
    const logIdStr = event.target.getAttribute('data-log-id');
    const appState = stateService.getState();
    // Get data based on current filters for export
    const currentLogAnalysisData = appState.jsAnalysisDataCache[logIdStr] || [];
    const filterCategory = appState.jsAnalysisFilterCategory;
    const searchText = appState.jsAnalysisSearchText.toLowerCase();

    let dataToExport = [...currentLogAnalysisData];
    if (filterCategory) {
        dataToExport = dataToExport.filter(item => item.category === filterCategory);
    }
    if (searchText) {
        dataToExport = dataToExport.filter(item =>
            item.category.toLowerCase().includes(searchText) ||
            item.finding.toLowerCase().includes(searchText)
        );
    }

    if (!dataToExport || dataToExport.length === 0) {
        uiService.showModalMessage("No Data", "No JavaScript analysis data available to export with current filters.");
        return;
    }
    uiService.showModalMessage("Exporting...", "Preparing CSV data...");
    const csvString = convertJsAnalysisToCSV(dataToExport);
    downloadCSV(csvString, `js_analysis_log_${logIdStr}.csv`);
    uiService.hideModal();
}

/**
 * Loads the detail view for a specific proxy log entry.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 * @param {string|number} logId - The ID of the log entry to display.
 */
export async function loadProxyLogDetailView(mainViewContainer, logId) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadProxyLogDetailView!");
        return;
    }
    if (!logId) {
        viewContentContainer.innerHTML = `<h1>Log Entry Detail</h1><p class="error-message">No Log ID provided.</p>`;
        uiService.updateBreadcrumbs([{ name: "Proxy Log", hash: "#proxy-log" }, { name: "Error" }]);
        return;
    }
    if (!apiService || !uiService || !stateService) {
        console.error("ProxyLogView not initialized for detail view. Call initProxyLogView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>ProxyLogView module not initialized. Critical services are missing.</p>";
        return;
    }

    viewContentContainer.innerHTML = `<h1>Loading Log Entry #${logId}...</h1>`;
    uiService.updateBreadcrumbs([{ name: "Proxy Log", hash: "#proxy-log" }, { name: `Log #${logId}` }]);

    const appState = stateService.getState();
    const { sortBy, sortOrder, filterFavoritesOnly, filterMethod, filterStatus, filterContentType, filterSearchText } = appState.paginationState.proxyLog;
    const navParams = { sortBy, sortOrder, favorites_only: filterFavoritesOnly, method: filterMethod, status: filterStatus, type: filterContentType, search: filterSearchText };

    // Check for tab parameter from hash
    const hashParams = new URLSearchParams(window.location.hash.split('?')[1] || '');
    const requestedTab = hashParams.get('tab');
    console.log("[ProxyLogDetailView] Requested tab from hash:", requestedTab);

    // Determine initial tab based on persisted setting and hash parameter
    let tabToActivate = 'requestTab'; // Default
    const defaultToResponse = localStorage.getItem('proxyLogDefaultToResponseTab') === 'true';

    if (defaultToResponse) {
        tabToActivate = 'responseTab';
    } else if (requestedTab) { // If not defaulting to response, then respect the hash parameter
        tabToActivate = requestedTab;
    }

    try {
        const logEntry = await apiService.getProxyLogDetail(logId, navParams);

        // --- DEBUGGING STARTS ---
        console.log("[ProxyLogDetailView] Log Entry Data:", JSON.parse(JSON.stringify(logEntry)));
        if (logEntry.request_full_url_with_fragment) {
            console.log("[ProxyLogDetailView] logEntry.request_full_url_with_fragment exists:", logEntry.request_full_url_with_fragment);
            console.log("[ProxyLogDetailView] logEntry.request_full_url_with_fragment.Valid:", logEntry.request_full_url_with_fragment.Valid);
            console.log("[ProxyLogDetailView] logEntry.request_full_url_with_fragment.String:", logEntry.request_full_url_with_fragment.String);
        } else {
            console.log("[ProxyLogDetailView] logEntry.request_full_url_with_fragment is null or undefined.");
        }
        console.log("[ProxyLogDetailView] logEntry.request_url?.String:", logEntry.request_url?.String);
        // --- DEBUGGING ENDS ---

        let reqHeaders = {};
        if (logEntry.request_headers && logEntry.request_headers.Valid && logEntry.request_headers.String) {
            try {
                reqHeaders = JSON.parse(logEntry.request_headers.String);
            } catch (e) {
                console.warn("Error parsing request_headers JSON string:", e, "Original value:", logEntry.request_headers.String);
                // reqHeaders remains {}
            }
        }

        let resHeaders = {};
        if (logEntry.response_headers && logEntry.response_headers.Valid && logEntry.response_headers.String) {
            try {
                resHeaders = JSON.parse(logEntry.response_headers.String);
            } catch (e) {
                console.warn("Error parsing response_headers JSON string:", e, "Original value:", logEntry.response_headers.String);
                // resHeaders remains {}
            }
        }

        const requestHttpVersionDisplay = (logEntry.request_http_version && logEntry.request_http_version.Valid)
            ? logEntry.request_http_version.String
            : 'N/A';
        const responseHttpVersionDisplay = (logEntry.response_http_version && logEntry.response_http_version.Valid)
            ? logEntry.response_http_version.String
            : 'N/A';

        // Log the entire logEntry to inspect its structure, especially associated_findings
        console.log("[ProxyLogDetailView] Received logEntry from API:", JSON.parse(JSON.stringify(logEntry)));

        let associatedFindingsHTML = '';
        if (logEntry.associated_findings && logEntry.associated_findings.length > 0) {
            associatedFindingsHTML = `
                <div class="associated-findings-section" style="margin-bottom: 15px; padding: 10px; background-color: #e9ecef; border-radius: 4px;">
                    <h4 style="margin-top:0; margin-bottom: 5px;">Associated Findings:</h4>
                    <ul style="list-style-type: disc; margin-left: 20px; padding-left: 0;">`;
            logEntry.associated_findings.forEach(finding => {
                associatedFindingsHTML += `<li><a href="#" class="view-associated-finding" data-finding-id="${finding.id}" title="View Finding: ${escapeHtmlAttribute(finding.title)}">${escapeHtml(finding.title)} (ID: ${finding.id})</a></li>`;
            });
            associatedFindingsHTML += `</ul>
                </div>`;
        }

        // Determine request content type from parsed headers for formatBody
        let requestContentTypeForBody = '';
        if (reqHeaders['Content-Type']) {
            requestContentTypeForBody = Array.isArray(reqHeaders['Content-Type']) ? reqHeaders['Content-Type'][0] : reqHeaders['Content-Type'];
        } else if (reqHeaders['content-type']) { // Check for lowercase
            requestContentTypeForBody = Array.isArray(reqHeaders['content-type']) ? reqHeaders['content-type'][0] : reqHeaders['content-type'];
        }

        viewContentContainer.innerHTML = `
            <div class="log-detail-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                <div style="display: flex; align-items: center;">
                    <button id="proxyLogDetailMoreActionsBtn" class="action-button" data-log-id="${logEntry.id}" title="More Actions" style="font-size: 1.5em; margin-right: 10px; padding: 0 5px;">⋮</button>
                    <h1>Log Entry Detail: #${logEntry.id}
                        <button id="analyzeJsBtn" class="secondary small-button" data-log-id="${logEntry.id}" style="margin-left: 15px;">Analyze JS</button>
                        <button id="findCommentsBtn" class="secondary small-button" data-log-id="${logEntry.id}" style="margin-left: 10px;">Find Comments</button>
                        <span id="favoriteToggleBtn" class="favorite-toggle ${logEntry.is_favorite ? 'favorited' : ''}" data-log-id="${logEntry.id}" data-is-favorite="${logEntry.is_favorite}" title="Toggle Favorite" style="margin-left: 10px; font-size: 1.2em; vertical-align: middle;">${logEntry.is_favorite ? '★' : '☆'}</span>
                    </h1>
                </div>
                <div class="log-navigation">
                    ${logEntry.prev_log_id ? `<button id="prevLogBtn" class="secondary" data-log-id="${logEntry.prev_log_id}" title="Previous Log Entry">&laquo; Previous</button>` : ''}
                    ${logEntry.next_log_id ? `<button id="nextLogBtn" class="secondary" data-log-id="${logEntry.next_log_id}" title="Next Log Entry" style="margin-left: ${logEntry.prev_log_id ? '5px' : '0'};">Next &raquo;</button>` : ''}
                </div>
            </div>
            ${associatedFindingsHTML}
            <div class="log-meta-info" style="margin-bottom: 15px; padding: 10px; background-color: #f9f9f9; border-radius: 4px;">
                <p><strong>Timestamp:</strong> ${new Date(logEntry.timestamp).toLocaleString()}</p>
                <!-- Tags Management Section -->
                <div id="logEntryTagsSection" style="margin-top: 10px;">
                    <strong style="display: block; margin-bottom: 5px;">Tags:</strong>
                    <div id="logEntryTagsContainer" style="margin-bottom: 8px; display: flex; flex-wrap: wrap; gap: 5px;">
                        ${logEntry.tags && logEntry.tags.length > 0
                            ? logEntry.tags.map(tag =>
                                `<span class="tag-chip" data-tag-id="${tag.id}" style="background-color: ${tag.color?.String || '#6c757d'}; color: white; padding: 3px 8px; border-radius: 12px; font-size: 0.9em; display: inline-flex; align-items: center;">
                                    ${escapeHtml(tag.name)}
                                    <button class="remove-tag-btn" data-tag-id="${tag.id}" data-log-id="${logEntry.id}" style="margin-left: 6px; background: none; border: none; color: white; cursor: pointer; font-size: 1.1em; padding: 0 3px;" title="Remove tag">&times;</button>
                                </span>`).join('')
                            : '<span id="noTagsMessage" style="font-style: italic;">No tags associated.</span>'}
                    </div>
                    <div id="addTagControls">
                        <input type="text" id="newTagInput" placeholder="Add tag..." list="allTagsDatalist" style="padding: 6px; border-radius: 4px; border: 1px solid #ccc; margin-right: 5px;">
                        <datalist id="allTagsDatalist"></datalist>
                        <button id="addTagBtn" data-log-id="${logEntry.id}" class="secondary small-button">Add</button>
                    </div>
                    <div id="tagManagementMessage" class="message-area" style="margin-top: 5px;"></div>
                </div>
                ${/* Display Page Name in detail view if available - MOVED UP */''}
                ${(logEntry.page_sitemap_id?.Valid && logEntry.page_sitemap_name?.Valid && logEntry.page_sitemap_name.String) ? `<p><strong>Associated Page:</strong> <a href="#page-sitemap?page_id=${logEntry.page_sitemap_id.Int64}" title="View Page: ${escapeHtmlAttribute(logEntry.page_sitemap_name.String)}">${escapeHtml(logEntry.page_sitemap_name.String)} (ID: ${logEntry.page_sitemap_id.Int64})</a></p>`
                : (logEntry.page_sitemap_id?.Valid ? `<p><strong>Associated Page ID:</strong> ${logEntry.page_sitemap_id.Int64} (Name not found)</p>` : '')}
                <p><strong>URL (Server-Seen):</strong> ${escapeHtml(logEntry.request_url?.String || 'N/A')}</p>
                ${ (logEntry.request_full_url_with_fragment &&
                    logEntry.request_full_url_with_fragment.Valid &&
                    logEntry.request_full_url_with_fragment.String &&
                    logEntry.request_full_url_with_fragment.String !== (logEntry.request_url?.String || ''))
                    ? `<p><strong>Full URL (Toolkit-Initiated):</strong> ${escapeHtml(logEntry.request_full_url_with_fragment.String)}</p>`
                    : ''
                }
                <p><strong>Method:</strong> ${escapeHtml(logEntry.request_method?.String || '')}</p> <!-- Access .String -->
                <p><strong>Status:</strong> ${logEntry.response_status_code || '-'}</p>
                <p><strong>Duration:</strong> ${logEntry.duration_ms || 0} ms</p>
                ${logEntry.target_id ? `<p><strong>Target ID:</strong> ${logEntry.target_id}</p>` : ''}
                ${logEntry.log_source?.Valid ? `<p><strong>Log Source:</strong> ${escapeHtml(logEntry.log_source.String)}</p>` : ''}
            </div>
            <div class="tabs">
                <button class="tab-button" data-tab="requestTab">Request</button>
                <button class="tab-button" data-tab="responseTab">Response</button>
                <button class="tab-button" data-tab="jsAnalysisTab">JS Analysis</button>
                <button class="tab-button" data-tab="commentsTab">Comments</button> <!-- New Tab -->
            </div>
            <div id="requestTab" class="tab-content active">
                <h3>Request Details</h3>
                <p><strong>HTTP Version:</strong> ${escapeHtml(requestHttpVersionDisplay)}</p>
                <h4>Headers:</h4>
                <pre class="headers-box" id="requestHeadersPre"></pre> <!-- Give it an ID -->
                <h4>Body:</h4>
                <pre class="body-box" id="requestBodyPre"></pre> <!-- Give it an ID -->
            </div>
            <div id="responseTab" class="tab-content">
                <h3>Response Details</h3>
                <p><strong>HTTP Version:</strong> ${escapeHtml(responseHttpVersionDisplay)}</p>
                <h4>Headers:</h4>
                <pre class="headers-box" id="responseHeadersPre"></pre> <!-- Give it an ID -->
                <h4>Body: (${logEntry.response_body_size} bytes)</h4>
                <pre class="body-box" id="responseBodyPre"></pre> <!-- Give it an ID -->
            </div>
            <div id="commentsTab" class="tab-content"><h3>Response Comments</h3> <!-- New Tab Content -->
                <div style="margin-bottom: 10px;"><button id="exportCommentsCsvBtn" class="secondary small-button" data-log-id="${logEntry.id}">Export to CSV</button></div>
                <div id="commentAnalysisResultsContent"><p>Click "Find Comments" to search for comments in the response.</p></div>
            </div>
            <div id="jsAnalysisTab" class="tab-content">
                <h3>JavaScript Analysis Results</h3>
                <div class="js-analysis-controls" style="margin-bottom: 10px; display: flex; gap: 15px; align-items: center;">
                    <div class="form-group" style="margin-bottom:0;">
                        <label for="jsAnalysisCategoryFilter" style="margin-right: 5px;">Category:</label>
                        <select id="jsAnalysisCategoryFilter" data-log-id="${logEntry.id}" style="min-width: 150px;">
                            <option value="">All</option>
                            <!-- Options will be populated dynamically -->
                        </select>
                    </div>
                    <div class="form-group" style="flex-grow:1; margin-bottom:0;">
                        <input type="search" id="jsAnalysisSearchInput" data-log-id="${logEntry.id}" placeholder="Search findings..." style="width: 100%;">
                    </div>
                    <button id="exportJsAnalysisCsvBtn" class="secondary small-button" data-log-id="${logEntry.id}">Export to CSV</button>
                </div>
                <div id="jsAnalysisPathSenderControls" class="js-analysis-controls" style="margin-bottom: 10px; display: none; gap: 15px; align-items: center; padding: 5px; border-radius: 4px;">
                    <div class="form-group" style="margin-bottom:0; flex-grow: 1;">
                        <label for="jsAnalysisPathPrefix" style="margin-right: 5px;">URL Prefix:</label>
                        <input type="text" id="jsAnalysisPathPrefix" placeholder="e.g., https://example.com" style="width: 80%;">
                    </div>
                    <button id="sendSelectedPathsToProxyBtn" class="primary small-button" data-log-id="${logEntry.id}">Send Selected to Proxy</button>
                </div>
                <div id="jsAnalysisResultsContent">
                    <p>Click "Analyze JS" to perform analysis.</p>
                </div>
            </div>
            <div class="notes-section" style="margin-top: 20px;"><h3>Notes:</h3><textarea id="logEntryNotes" rows="5" style="width: 100%;">${escapeHtml(logEntry.notes && logEntry.notes.Valid ? logEntry.notes.String : '')}</textarea><button id="saveLogEntryNotesBtn" class="primary" data-log-id="${logEntry.id}" style="margin-top: 10px;">Save Notes</button><div id="saveNotesMessage" class="message-area" style="margin-top: 5px;"></div></div>`;

        // Safely set the response body content
        const responseBodyPre = document.getElementById('responseBodyPre');
        if (responseBodyPre) { // For Response
            // Ensure we pass the string value from the sql.NullString object
            const responseContentTypeString = (logEntry.response_content_type && logEntry.response_content_type.Valid) 
                                              ? logEntry.response_content_type.String 
                                              : '';
            responseBodyPre.textContent = formatBody(logEntry.response_body, responseContentTypeString);
        }
        // Safely set the request body content
        const requestBodyPre = document.getElementById('requestBodyPre');
        if (requestBodyPre) { // For Request
            requestBodyPre.textContent = formatBody(logEntry.request_body, requestContentTypeForBody);
        }
        // Safely set the request headers
        const requestHeadersPre = document.getElementById('requestHeadersPre');
        if (requestHeadersPre) {
            requestHeadersPre.textContent = formatHeaders(reqHeaders);
        }
        // Safely set the response headers
        const responseHeadersPre = document.getElementById('responseHeadersPre');
        if (responseHeadersPre) {
            responseHeadersPre.textContent = formatHeaders(resHeaders);
        }

        document.querySelectorAll('.tab-button').forEach(button => {
            button.addEventListener('click', () => {
                document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
                button.classList.add('active');
                document.getElementById(button.getAttribute('data-tab')).classList.add('active');
                // If JS Analysis tab is clicked, and no data, trigger analysis
                if (button.getAttribute('data-tab') === 'jsAnalysisTab' && !appState.jsAnalysisDataCache[String(logId)]) {
                    document.getElementById('analyzeJsBtn')?.dispatchEvent(new Event('click'));
                }
                // If Comments tab is clicked, and no data, trigger analysis
                if (button.getAttribute('data-tab') === 'commentsTab' && !appState.commentAnalysisDataCache[String(logId)]) {
                    document.getElementById('findCommentsBtn')?.dispatchEvent(new Event('click'));
                }
            });
        });

        // --- Attaching listener for the "More Actions" (⋮) button ---
        const moreActionsButtonElement = document.getElementById('proxyLogDetailMoreActionsBtn');
        console.log('[ProxyLogView] loadProxyLogDetailView: Attempting to find #proxyLogDetailMoreActionsBtn. Found:', moreActionsButtonElement ? 'Yes' : 'No');

        if (moreActionsButtonElement) {
            // To prevent multiple listeners if this function is called again for the same view,
            // we clone the node and replace it. This effectively removes all old listeners.
            const newButton = moreActionsButtonElement.cloneNode(true);
            if (moreActionsButtonElement.parentNode) {
                moreActionsButtonElement.parentNode.replaceChild(newButton, moreActionsButtonElement);
                console.log('[ProxyLogView] loadProxyLogDetailView: Replaced #proxyLogDetailMoreActionsBtn node.');

                newButton.addEventListener('click', (event) => {
                    console.log('[ProxyLogView] Detail view "more actions" (⋮) button CLICKED for logId:', logEntry.id);
                    openDetailViewMoreActionsDropdown(event, logEntry.id);
                });
                // Attach listeners for tag management
                document.getElementById('addTagBtn')?.addEventListener('click', handleAddTagToLogEntry);
                document.querySelectorAll('.remove-tag-btn').forEach(btn => btn.addEventListener('click', handleRemoveTagFromLogEntry));
                console.log('[ProxyLogView] loadProxyLogDetailView: Event listener ATTACHED to #proxyLogDetailMoreActionsBtn.');
            } else {
                console.error('[ProxyLogView] loadProxyLogDetailView: #proxyLogDetailMoreActionsBtn found, but its parentNode is null. Cannot attach listener.');
            }
        }

        // Add event listeners for the new "View Associated Finding" links
        viewContentContainer.querySelectorAll('.view-associated-finding').forEach(link => {
            link.addEventListener('click', (e) => { e.preventDefault(); handleViewAssociatedFindingDetailFromProxyLog(e.currentTarget.dataset.findingId); });
        });

        document.getElementById('findCommentsBtn')?.addEventListener('click', handleFindCommentsForDetailView);
        document.getElementById('exportCommentsCsvBtn')?.addEventListener('click', handleExportCommentsToCSV);

        // JS Analysis event listeners
        document.getElementById('analyzeJsBtn')?.addEventListener('click', handleAnalyzeJS);
        document.getElementById('exportJsAnalysisCsvBtn')?.addEventListener('click', handleExportJsAnalysisToCSV);
        document.getElementById('jsAnalysisCategoryFilter')?.addEventListener('change', handleJsAnalysisCategoryFilter);
        document.getElementById('jsAnalysisSearchInput')?.addEventListener('input', handleJsAnalysisSearch);
        document.getElementById('sendSelectedPathsToProxyBtn')?.addEventListener('click', handleSendSelectedPathsToProxy);


        document.getElementById('saveLogEntryNotesBtn').addEventListener('click', async (event) => {
            const notes = document.getElementById('logEntryNotes').value;
            const currentLogId = event.target.getAttribute('data-log-id');
            const messageArea = document.getElementById('saveNotesMessage');
            messageArea.textContent = ''; messageArea.className = 'message-area';
            try {
                await apiService.saveProxyLogNotes(currentLogId, notes);
                messageArea.textContent = 'Notes saved successfully!'; messageArea.classList.add('success-message');
            } catch (saveError) {
                messageArea.textContent = `Error saving notes: ${saveError.message}`; messageArea.classList.add('error-message');
            }
        });

        const prevBtn = document.getElementById('prevLogBtn');
        const nextBtn = document.getElementById('nextLogBtn');
        const currentFiltersAndSortQuery = new URLSearchParams(navParams).toString();
        if (prevBtn) prevBtn.addEventListener('click', () => window.location.hash = `#proxy-log-detail?id=${prevBtn.getAttribute('data-log-id')}&${currentFiltersAndSortQuery}`);
        if (nextBtn) nextBtn.addEventListener('click', () => window.location.hash = `#proxy-log-detail?id=${nextBtn.getAttribute('data-log-id')}&${currentFiltersAndSortQuery}`);

        document.getElementById('favoriteToggleBtn')?.addEventListener('click', async (event) => {
            const button = event.currentTarget;
            const currentLogId = button.getAttribute('data-log-id');
            const isCurrentlyFavorite = button.getAttribute('data-is-favorite') === 'true';
            const newFavoriteState = !isCurrentlyFavorite;
            try {
                await apiService.setProxyLogFavorite(currentLogId, newFavoriteState);
                button.innerHTML = newFavoriteState ? '★' : '☆';
                button.classList.toggle('favorited', newFavoriteState);
                button.setAttribute('data-is-favorite', newFavoriteState.toString());
            } catch (favError) {
                uiService.showModalMessage("Error", `Failed to update favorite status: ${favError.message}`);
            }
        });

        const logIdString = String(logId);
        // If data is already in cache, populate filter and render table
        if (appState.jsAnalysisDataCache[logIdString] && appState.jsAnalysisDataCache[logIdString].length > 0) {
            populateJsAnalysisCategoryFilter(logIdString);
            renderJsAnalysisTable(logIdString);
        }
        // New: Render comments if already in cache
        if (appState.commentAnalysisDataCache[logIdString] && appState.commentAnalysisDataCache[logIdString].length > 0) {
            renderCommentAnalysisTable(logIdString);
        }

        // Activate the determined tab
        const tabButtonToActivate = document.querySelector(`.tab-button[data-tab="${tabToActivate}"]`);
        if (tabButtonToActivate) {
            tabButtonToActivate.click();
        } else if (document.querySelector('.tab-button[data-tab="requestTab"]')) {
            // Default to requestTab if specified tab is invalid
            document.querySelector('.tab-button[data-tab="requestTab"]')?.click();
        }
        fetchAllTagsForDatalist(); // Fetch tags for the datalist
    } catch (error) {
        viewContentContainer.innerHTML = `<h1>Log Entry Detail</h1><p class="error-message">Error loading details for Log ID ${logId}: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleFindCommentsForDetailView(event) {
    const button = event.target;
    const logIdStr = button.getAttribute('data-log-id');
    const resultsContentDiv = document.getElementById('commentAnalysisResultsContent');

    if (!logIdStr || !resultsContentDiv) {
        console.error("FindComments: Log ID or results container not found.");
        if (resultsContentDiv) resultsContentDiv.innerHTML = `<p class="error-message">Error: Could not get log ID or results container for comment analysis.</p>`;
        return;
    }

    // Ensure the "Comments" tab is active
    document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
    document.querySelector('.tab-button[data-tab="commentsTab"]')?.classList.add('active');
    document.getElementById('commentsTab')?.classList.add('active');

    resultsContentDiv.innerHTML = `<p>Searching for comments in response for log entry #${logIdStr}...</p>`;

    try {
        const commentFindings = await apiService.findComments(parseInt(logIdStr, 10)); // API returns array of models.CommentFinding
        resultsContentDiv.innerHTML = ''; // Clear loading message

        stateService.updateState({ commentAnalysisDataCache: { [logIdStr]: commentFindings || [] } });

        if (commentFindings && commentFindings.length > 0) {
            renderCommentAnalysisTable(logIdStr);
        } else {
            resultsContentDiv.innerHTML = `<p>No comments found in the response body.</p>`;
        }
    } catch (error) {
        resultsContentDiv.innerHTML = `<p class="error-message">Error finding comments for log #${logIdStr}: ${escapeHtml(error.message)}</p>`;
    }
}

function renderCommentAnalysisTable(logIdStr) {
    const resultsContentDiv = document.getElementById('commentAnalysisResultsContent');
    if (!resultsContentDiv) return;

    const appState = stateService.getState();
    const currentLogCommentData = appState.commentAnalysisDataCache[logIdStr];
    const currentSortState = appState.commentAnalysisSortState;
    const columnDefinitions = appState.paginationState.commentAnalysisTableLayout;
    const maxContextLength = 150; // Max characters for context display in table
    const globalTableLayouts = appState.globalTableLayouts || {};
    const tableKey = 'commentAnalysisTable'; // Unique key for this table's layout
    const savedLayout = globalTableLayouts[tableKey] || { columns: {} };
    const savedColumnSettings = savedLayout.columns || {};

    if (!currentLogCommentData) {
        resultsContentDiv.innerHTML = "<p>No comment analysis data available for this log entry.</p>";
        return;
    }
    if (currentLogCommentData.length === 0) {
        resultsContentDiv.innerHTML = "<p>No comments found to display.</p>"; // Keep existing message for no data
        return;
    }

    // Button to save layout (optional, but good for consistency)
    let saveLayoutButtonHTML = `
        <div style="margin-bottom: 10px; text-align: right;">
            <button id="saveCommentAnalysisLayoutBtn" class="secondary small-button">Save Column Layout</button>
        </div>
    `;

    // Defined order for columns
    const displayOrder = ['lineNumber', 'commentType', 'commentText', 'contextBefore', 'contextAfter'];

    let tableHTML = `${saveLayoutButtonHTML}<table><thead id="commentAnalysisTableHead"><tr>`; // Added table head ID

    // Helper function to normalize string values for display
    function normalizeStringForDisplay(value) {
        if (value === null || typeof value === 'undefined') {
            return '';
        }
        const strValue = String(value);
        if (strValue === "undefined") {
            return '';
        }
        return strValue;
    }
    const sortedData = [...currentLogCommentData].sort((a, b) => {
        // Ensure sortBy key exists on the objects, default to empty string if not
        const valA = a[currentSortState.sortBy] !== undefined ? a[currentSortState.sortBy] : '';
        const valB = b[currentSortState.sortBy] !== undefined ? b[currentSortState.sortBy] : '';
        let comparison = 0;
        if (valA > valB) comparison = 1;
        else if (valA < valB) comparison = -1;
        return currentSortState.sortOrder === 'ASC' ? comparison : comparison * -1;
    });

    displayOrder.forEach(key => {
        const colDef = columnDefinitions[key];
        if (!colDef || !colDef.visible) return; // Skip if not defined or not visible

        const savedColSetting = savedColumnSettings[key];
        const thStyleWidth = savedColSetting?.width || colDef.default || 'auto';

        // --- Add this logging block ---
        if (key === 'commentType') {
            console.log(`[ProxyLogView] 'commentType' Column ('${key}') Width Calculation:`);
            console.log(`  - Full colDef from stateService:`, JSON.parse(JSON.stringify(colDef || {})));
            console.log(`  - Full savedColSetting from globalTableLayouts:`, JSON.parse(JSON.stringify(savedColSetting || {})));
            console.log(`  - Value used for colDef.default:`, colDef?.default);
            console.log(`  - Value used for savedColSetting?.width:`, savedColSetting?.width);
            console.log(`  - Final thStyleWidth for 'commentType':`, thStyleWidth);
        }
        // --- End of logging block ---

        let classes = colDef.sortKey ? 'sortable' : '';
        if (colDef.sortKey === currentSortState.sortBy) {
            classes += currentSortState.sortOrder === 'ASC' ? ' sorted-asc' : ' sorted-desc';
        }

        tableHTML += `<th style="width: ${thStyleWidth};" class="${classes}" 
                          ${colDef.sortKey ? `data-sort-key="${colDef.sortKey}"` : ''} 
                          data-col-key="${key}" id="${colDef.id}">
                          ${escapeHtml(colDef.label)}
                      </th>`;
    });
    tableHTML += `</tr></thead><tbody>`;

    sortedData.forEach(finding => {
            const normalizedLineNumber = normalizeStringForDisplay(finding.lineNumber); // Use camelCase
            const normalizedCommentText = normalizeStringForDisplay(finding.commentText); // Use camelCase
            const normalizedCommentType = normalizeStringForDisplay(finding.commentType); // Use camelCase
            const normalizedContextBefore = normalizeStringForDisplay(finding.contextBefore); // Use camelCase
            const normalizedContextAfter = normalizeStringForDisplay(finding.contextAfter); // Use camelCase

            const maxCommentLength = 256;
            const fullEscapedCommentTextForTitle = escapeHtmlAttribute(normalizedCommentText);
            let displayCommentText;
            let viewFullCommentButtonHTML = ''; // Initialize as empty

            if (normalizedCommentText.length > maxCommentLength) {
                displayCommentText = escapeHtml(normalizedCommentText.substring(0, maxCommentLength)) + "...";
                // Only create the button if the text is truncated
                viewFullCommentButtonHTML = ` <button class="action-button view-full-comment-btn" data-full-comment="${fullEscapedCommentTextForTitle}" title="View Full Comment" style="margin-left: 5px; padding: 0 3px; font-size: 0.9em;">👁️</button>`;
            } else {
                displayCommentText = escapeHtml(normalizedCommentText);
            }
            tableHTML += `<tr>`;
            displayOrder.forEach(key => {
                const colDef = columnDefinitions[key];
                if (!colDef || !colDef.visible) return;

                switch(key) {
                    case 'lineNumber': tableHTML += `<td>${normalizedLineNumber}</td>`; break;
                    case 'commentType': tableHTML += `<td>${escapeHtml(normalizedCommentType)}</td>`; break;
                    case 'commentText': tableHTML += `<td title="${fullEscapedCommentTextForTitle}">${displayCommentText}${viewFullCommentButtonHTML}</td>`; break;
                    case 'contextBefore': {
                        const fullEscapedContext = escapeHtmlAttribute(normalizedContextBefore);
                        let displayContext = escapeHtml(normalizedContextBefore);
                        let viewFullContextBtn = '';
                        if (normalizedContextBefore.length > maxContextLength) {
                            displayContext = escapeHtml(normalizedContextBefore.substring(0, maxContextLength)) + "...";
                            viewFullContextBtn = ` <button class="action-button view-full-context-btn" data-full-context="${fullEscapedContext}" title="View Full Context Before" style="margin-left: 5px; padding: 0 3px; font-size: 0.9em;">👁️</button>`;
                        }
                        tableHTML += `<td class="comment-context-cell" title="${fullEscapedContext}"><pre class="context-before">${displayContext}</pre>${viewFullContextBtn}</td>`;
                        break;
                    }
                    case 'contextAfter': {
                        const fullEscapedContext = escapeHtmlAttribute(normalizedContextAfter);
                        let displayContext = escapeHtml(normalizedContextAfter);
                        let viewFullContextBtn = '';
                        if (normalizedContextAfter.length > maxContextLength) {
                            displayContext = escapeHtml(normalizedContextAfter.substring(0, maxContextLength)) + "...";
                            viewFullContextBtn = ` <button class="action-button view-full-context-btn" data-full-context="${fullEscapedContext}" title="View Full Context After" style="margin-left: 5px; padding: 0 3px; font-size: 0.9em;">👁️</button>`;
                        }
                        tableHTML += `<td class="comment-context-cell" title="${fullEscapedContext}"><pre class="context-after">${displayContext}</pre>${viewFullContextBtn}</td>`;
                        break;
                    }
                }
            });
            tableHTML += `</tr>`;
    });
    tableHTML += `</tbody></table>`;
    resultsContentDiv.innerHTML = tableHTML;

    const tableHeadElement = document.getElementById('commentAnalysisTableHead');
    if (tableHeadElement) {
        tableHeadElement.querySelectorAll('th.sortable').forEach(th => {
        th.classList.toggle('sorted-asc', currentSortState.sortBy === th.dataset.sortKey && currentSortState.sortOrder === 'ASC');
        th.classList.toggle('sorted-desc', currentSortState.sortBy === th.dataset.sortKey && currentSortState.sortOrder === 'DESC');
        th.addEventListener('click', (event) => handleCommentAnalysisSort(event, logIdStr));
        });
        if (tableService) {
            tableService.makeTableColumnsResizable('commentAnalysisTableHead', columnDefinitions);
        }
    }

    const saveLayoutBtn = document.getElementById('saveCommentAnalysisLayoutBtn');
    if (saveLayoutBtn && tableService) {
        saveLayoutBtn.addEventListener('click', () => {
            tableService.saveCurrentTableLayout(tableKey, 'commentAnalysisTableHead');
        });
    };

    // Add event listeners for the new "View Full Comment" buttons
    resultsContentDiv.querySelectorAll('.view-full-comment-btn').forEach(button => {
        button.addEventListener('click', handleViewFullComment);
    });

    // Add event listeners for the new "View Full Context" buttons
    resultsContentDiv.querySelectorAll('.view-full-context-btn').forEach(button => {
        button.addEventListener('click', handleViewFullContext);
    });
}

function handleCommentAnalysisSort(event, logIdStr) {
    // Prevent sorting if a column resize was just completed
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) {
        console.log('[ProxyLogView - Comments] Sort prevented due to active resize operation.');
        return;
    }

    const newSortBy = event.target.dataset.sortKey;
    const appState = stateService.getState();
    let newSortOrder = 'ASC';
    if (appState.commentAnalysisSortState.sortBy === newSortBy) {
        newSortOrder = appState.commentAnalysisSortState.sortOrder === 'ASC' ? 'DESC' : 'ASC';
    }
    stateService.updateState({ commentAnalysisSortState: { sortBy: newSortBy, sortOrder: newSortOrder } });
    renderCommentAnalysisTable(logIdStr);
}


function convertCommentAnalysisToCSV(jsonData) {
    const headersConfig = [
        { key: 'lineNumber', label: 'Line Number' },
        { key: 'commentType', label: 'Comment Type' },
        { key: 'commentText', label: 'Comment Text' },
        { key: 'contextBefore', label: 'Context Before' },
        { key: 'contextAfter', label: 'Context After' },
    ];
    const headerRow = headersConfig.map(h => escapeHtml(h.label)).join(',');
    const dataRows = jsonData.map(item => headersConfig.map(header => escapeHtml(String(item[header.key]))).join(','));
    return [headerRow].concat(dataRows).join('\n');
}

function handleExportCommentsToCSV(event) {
    const logIdStr = event.target.getAttribute('data-log-id');
    const appState = stateService.getState();
    const currentLogCommentData = appState.commentAnalysisDataCache[logIdStr];
    if (!currentLogCommentData || currentLogCommentData.length === 0) { uiService.showModalMessage("No Data", "No comment data available to export."); return; }
    uiService.showModalMessage("Exporting...", "Preparing CSV data..."); const csvString = convertCommentAnalysisToCSV(currentLogCommentData); downloadCSV(csvString, `comments_log_${logIdStr}.csv`); uiService.hideModal();
}

function handleViewFullComment(event) {
    const button = event.currentTarget;
    // This attribute already contains HTML-escaped text.
    // For example, if original comment was "A < B", this is "A &lt; B".
    // We want to display "A &lt; B" literally in the modal.
    const alreadyEscapedCommentText = button.getAttribute('data-full-comment');

    if (!uiService) {
        console.error("uiService not available in handleViewFullComment");
        alert(alreadyEscapedCommentText); // Fallback
        return;
    }

    // Create a div element to hold the comment and apply styles
    const contentDiv = document.createElement('div');
    contentDiv.style.maxHeight = '70vh';
    contentDiv.style.overflowY = 'auto';
    contentDiv.style.whiteSpace = 'pre-wrap'; // Preserve whitespace and newlines
    contentDiv.style.wordWrap = 'break-word'; // Break long words
    contentDiv.textContent = alreadyEscapedCommentText; // Set as textContent to display literally

    uiService.showModalMessage("Full Comment Text", contentDiv);
}

function handleViewFullContext(event) {
    const button = event.currentTarget;
    // This attribute already contains HTML-escaped text.
    const alreadyEscapedContextText = button.getAttribute('data-full-context');

    if (!uiService) {
        console.error("uiService not available in handleViewFullContext");
        alert(alreadyEscapedContextText); // Fallback
        return;
    }

    const contentDiv = document.createElement('div');
    contentDiv.style.maxHeight = '70vh';
    contentDiv.style.overflowY = 'auto';
    contentDiv.style.whiteSpace = 'pre-wrap';
    contentDiv.style.wordWrap = 'break-word';
    contentDiv.textContent = alreadyEscapedContextText; // Display literally

    uiService.showModalMessage("Full Context", contentDiv);
}

async function handleSendSelectedPathsToProxy(event) {
    const logIdStr = event.target.dataset.logId;
    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    if (!currentTargetId) {
        uiService.showModalMessage("Error", "No current target is set. Cannot send requests.");
        return;
    }

    const prefixInput = document.getElementById('jsAnalysisPathPrefix');
    const prefix = prefixInput ? prefixInput.value.trim() : '';

    if (!prefix) {
        uiService.showModalMessage("Error", "URL Prefix is required to send paths.");
        prefixInput.focus();
        return;
    }
    // Basic URL validation for prefix
    try {
        new URL(prefix);
    } catch (e) {
        uiService.showModalMessage("Error", "Invalid URL Prefix format.");
        prefixInput.focus();
        return;
    }


    const selectedPathCheckboxes = document.querySelectorAll('.js-path-checkbox:checked');
    if (selectedPathCheckboxes.length === 0) {
        uiService.showModalMessage("Info", "No paths selected to send.");
        return;
    }

    const urlsToSend = [];
    console.log("[PathSender] Prefix:", prefix); // DEBUG
    selectedPathCheckboxes.forEach(checkbox => {
        const path = checkbox.dataset.path;
        console.log("[PathSender] Selected checkbox data-path:", path); // DEBUG
        if (path === undefined || path === null || path.trim() === "") {
            console.warn("[PathSender] Empty or undefined path found for a selected checkbox. Skipping."); // DEBUG
            return; // Skip this iteration if path is empty
        }
        let fullUrl = prefix.endsWith('/') ? prefix.slice(0, -1) : prefix;
        fullUrl += path.startsWith('/') ? path : '/' + path;
        console.log("[PathSender] Constructed fullUrl:", fullUrl); // DEBUG
        urlsToSend.push(fullUrl);
    });

    if (urlsToSend.length === 0) {
        uiService.showModalMessage("Info", "No valid paths selected to send (all selected paths were empty).");
        return;
    }

    uiService.showModalMessage("Sending...", `Sending ${urlsToSend.length} request(s) to the proxy...`, true, 2000);

    try {
        await apiService.sendPathsToProxy({ target_id: currentTargetId, urls: urlsToSend });
        uiService.showModalMessage("Success", `${urlsToSend.length} request(s) sent to proxy. Check the Proxy Log for responses.`);
        // Optionally deselect checkboxes after sending
        selectedPathCheckboxes.forEach(checkbox => checkbox.checked = false);
        const selectAllCheckbox = document.getElementById('selectAllJsPathsCheckbox');
        if(selectAllCheckbox) selectAllCheckbox.checked = false;

    } catch (error) {
        console.error("Error sending paths to proxy:", error);
        uiService.showModalMessage("Error", `Failed to send requests: ${escapeHtml(error.message)}`);
    }
}

function openCustomizeColumnsModal() {
    const appState = stateService.getState();
    const columnDefinitions = appState.paginationState.proxyLogTableLayout;
    const globalTableLayouts = appState.globalTableLayouts || {};
    const savedLayout = globalTableLayouts.proxyLogTable || { columns: {} };
    const savedColumnSettings = savedLayout.columns || {};

    let modalContentHTML = `<div id="customizeColumnsModalContent" style="text-align: left;">`;
    modalContentHTML += `<p style="margin-bottom: 15px;">Select columns to display:</p>`;

    // Use a defined order for checkboxes if necessary, or iterate columnDefinitions
    const displayOrder = ['index', 'timestamp', 'method', 'page_name', 'url', 'status', 'type', 'size', 'actions'];

    displayOrder.forEach(key => {
        const colDef = columnDefinitions[key];
        if (!colDef) return; // Should not happen if displayOrder matches keys in columnDefinitions

        if (colDef.nonHideable) return; // Skip non-hideable columns like 'actions'

        const savedSetting = savedColumnSettings[key];
        const isChecked = savedSetting ? savedSetting.visible : colDef.visible; // Use saved visibility, fallback to default

        modalContentHTML += `
            <div class="form-group" style="margin-bottom: 8px;">
                <input type="checkbox" id="colToggle_${key}" data-col-key="${key}" ${isChecked ? 'checked' : ''}>
                <label for="colToggle_${key}" style="font-weight: normal; margin-left: 5px;">${escapeHtml(colDef.label)}</label>
            </div>
        `;
    });
    modalContentHTML += `</div>`;

    uiService.showModalConfirm(
        "Customize Proxy Log Columns",
        modalContentHTML,
        async () => { // onConfirm (Apply)
            const newColumnSettings = { ...savedColumnSettings }; // Start with existing saved settings

            displayOrder.forEach(key => {
                const colDef = columnDefinitions[key];
                if (!colDef || colDef.nonHideable) return;

                const checkbox = document.getElementById(`colToggle_${key}`);
                if (checkbox) {
                    newColumnSettings[key] = {
                        ...(newColumnSettings[key] || {}), // Preserve existing width if any
                        width: newColumnSettings[key]?.width || colDef.default, // Keep existing or use default
                        visible: checkbox.checked
                    };
                }
            });
            
            // Update the globalTableLayouts in state
            const updatedGlobalLayouts = {
                ...globalTableLayouts,
                proxyLogTable: {
                    ...savedLayout, // Preserve existing pageSize
                    columns: newColumnSettings
                }
            };
            stateService.updateState({ globalTableLayouts: updatedGlobalLayouts });
            
            // Now call the unified save function
            await prepareAndSaveProxyLogLayout(); 
            fetchAndDisplayProxyLogs(stateService.getState().paginationState.proxyLog); // Refresh view
            return true; // Close modal
        },
        () => { /* onCancel - do nothing */ },
        "Apply Changes", "Cancel", true
    );
}

async function prepareAndSaveProxyLogLayout() {
    const pageSizeSelect = document.getElementById('proxyLogItemsPerPageSelect');
    const currentPageSize = pageSizeSelect ? pageSizeSelect.value : null;
    // tableService.saveCurrentTableLayout now expects the full layout object for the specific table
    // We need to construct it based on current state and DOM here.
    // For now, this just saves widths and page size. Visibility is handled by updating globalTableLayouts directly.
    tableService.saveCurrentTableLayout('proxyLogTable', 'proxyLogTableHead', currentPageSize);
}

function renderTagFilterDropdown(tags) {
    const container = document.getElementById('proxyLogTagFilterContainer');
    if (!container) return;

    const appState = stateService.getState();
    const currentSelectedTagIDs = appState.paginationState.proxyLog.filterTagIDs || [];

    if (!tags || tags.length === 0) {
        container.innerHTML = '<p style="font-style: italic; font-size: 0.9em;">No tags available for filtering in the current log view.</p>';
        return;
    }

    let selectHTML = `<label for="proxyLogTagFilterSelect" style="margin-right: 5px;">Filter by Tags:</label>
                      <select id="proxyLogTagFilterSelect" multiple style="min-width: 250px; height: auto; max-height: 100px;">`;

    tags.forEach(tag => {
        const isSelected = currentSelectedTagIDs.includes(String(tag.id)); // Ensure comparison with string IDs if needed
        selectHTML += `<option value="${tag.id}" ${isSelected ? 'selected' : ''} style="color: ${tag.color?.String || 'inherit'};">
                           ${escapeHtml(tag.name)}
                       </option>`;
    });
    selectHTML += `</select>
                   <button id="clearTagFilterBtn" class="secondary small-button" style="margin-left: 10px;" title="Clear Tag Filter">Clear</button>`;

    container.innerHTML = selectHTML;

    const selectElement = document.getElementById('proxyLogTagFilterSelect');
    if (selectElement) {
        selectElement.addEventListener('change', handleTagFilterChange);
    }
    const clearButton = document.getElementById('clearTagFilterBtn');
    if (clearButton) {
        clearButton.addEventListener('click', () => {
            if (selectElement) {
                Array.from(selectElement.options).forEach(option => option.selected = false);
            }
            handleTagFilterChange(); // Trigger change with empty selection
        });
    }
}

function handleTagFilterChange() {
    const selectElement = document.getElementById('proxyLogTagFilterSelect');
    const selectedTagIDs = Array.from(selectElement.selectedOptions).map(option => option.value);

    const appState = stateService.getState();
    stateService.updateState({ paginationState: { ...appState.paginationState, proxyLog: { ...appState.paginationState.proxyLog, filterTagIDs: selectedTagIDs, currentPage: 1 } } });
    fetchAndDisplayProxyLogs(stateService.getState().paginationState.proxyLog); // Pass the updated state
}

async function fetchAllTagsForDatalist() {
    const datalist = document.getElementById('allTagsDatalist');
    if (!datalist) return;

    try {
        const tags = await apiService.getAllTags(); // Assuming apiService.getAllTags() exists
        datalist.innerHTML = ''; // Clear existing options
        tags.forEach(tag => {
            const option = document.createElement('option');
            option.value = escapeHtmlAttribute(tag.name);
            datalist.appendChild(option);
        });
    } catch (error) {
        console.error("Error fetching all tags for datalist:", error);
        // Optionally display an error to the user or log it
    }
}

async function handleAddTagToLogEntry(event) {
    const logId = event.target.dataset.logId;
    const newTagInput = document.getElementById('newTagInput');
    const tagName = newTagInput.value.trim();
    const messageArea = document.getElementById('tagManagementMessage');

    if (!tagName) {
        messageArea.textContent = 'Tag name cannot be empty.';
        messageArea.className = 'message-area error-message';
        return;
    }

    try {
        // 1. Create the tag (backend handles if it already exists)
        const tag = await apiService.createTag({ name: tagName }); // Assuming apiService.createTag exists

        // 2. Associate the tag with the log entry
        await apiService.associateTagWithItem(tag.id, logId, 'httplog'); // Assuming this API exists

        // 3. Update UI
        const tagsContainer = document.getElementById('logEntryTagsContainer');
        const noTagsMsg = document.getElementById('noTagsMessage');
        if (noTagsMsg) noTagsMsg.remove();

        const tagChip = document.createElement('span');
        tagChip.className = 'tag-chip';
        tagChip.dataset.tagId = tag.id;
        tagChip.style.backgroundColor = tag.color?.String || '#6c757d';
        tagChip.style.color = 'white';
        tagChip.style.padding = '3px 8px';
        tagChip.style.borderRadius = '12px';
        tagChip.style.fontSize = '0.9em';
        tagChip.style.display = 'inline-flex';
        tagChip.style.alignItems = 'center';
        tagChip.innerHTML = `${escapeHtml(tag.name)} <button class="remove-tag-btn" data-tag-id="${tag.id}" data-log-id="${logId}" style="margin-left: 6px; background: none; border: none; color: white; cursor: pointer; font-size: 1.1em; padding: 0 3px;" title="Remove tag">&times;</button>`;
        tagsContainer.appendChild(tagChip);
        tagChip.querySelector('.remove-tag-btn').addEventListener('click', handleRemoveTagFromLogEntry);

        newTagInput.value = ''; // Clear input
        messageArea.textContent = `Tag "${escapeHtml(tagName)}" added.`;
        messageArea.className = 'message-area success-message';
        fetchAllTagsForDatalist(); // Refresh datalist in case a new tag was created
    } catch (error) {
        console.error("Error adding tag:", error);
        messageArea.textContent = `Error adding tag: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
    }
}

async function handleRemoveTagFromLogEntry(event) {
    const tagId = event.target.dataset.tagId;
    const logId = event.target.dataset.logId;
    const messageArea = document.getElementById('tagManagementMessage');

    try {
        await apiService.removeTagFromItem(tagId, logId, 'httplog'); // Assuming this API exists

        // Update UI
        const tagChipToRemove = document.querySelector(`.tag-chip[data-tag-id="${tagId}"]`);
        if (tagChipToRemove) tagChipToRemove.remove();

        const tagsContainer = document.getElementById('logEntryTagsContainer');
        if (tagsContainer.children.length === 0) {
            tagsContainer.innerHTML = '<span id="noTagsMessage" style="font-style: italic;">No tags associated.</span>';
        }
        messageArea.textContent = 'Tag removed.';
        messageArea.className = 'message-area success-message';
    } catch (error) {
        console.error("Error removing tag:", error);
        messageArea.textContent = `Error removing tag: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
    }
}
