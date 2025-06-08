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
    if (!body) return '(Empty body)';
    try {
        const textContent = atob(body);
        if (contentType.toLowerCase().includes('json')) {
            try {
                return JSON.stringify(JSON.parse(textContent), null, 2);
            } catch (e) { /* fallback to text */ }
        }
        return escapeHtml(textContent.replace(/[\x00-\x1F\x7F-\x9F]/g, '.'));
    } catch (e) {
        return escapeHtml(body.substring(0, 2000) + (body.length > 2000 ? "\n... (truncated)" : ""));
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
    const { currentPage, totalPages, totalRecords, sortBy, sortOrder, filterFavoritesOnly, filterMethod, filterStatus, filterContentType, filterSearchText } = appState.paginationState.proxyLog;
    let paginationHTML = '';

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total log(s) found.</p>` : '';
        return;
    }
    paginationHTML += `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total logs)</p>`;
    const buildHash = (page) => `#proxy-log?page=${page}&sort_by=${sortBy}&sort_order=${sortOrder}&favorites_only=${filterFavoritesOnly}&method=${encodeURIComponent(filterMethod)}&status=${encodeURIComponent(filterStatus)}&type=${encodeURIComponent(filterContentType)}&search=${encodeURIComponent(filterSearchText)}`;

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
    const listDiv = document.getElementById('proxyLogList');
    const paginationControlsDiv = document.getElementById('proxyLogPaginationControls');
    if (!listDiv || !paginationControlsDiv) {
        console.error("Proxy log list or pagination container not found.");
        return;
    }
    
    const appState = stateService.getState(); // Still get currentTargetId etc. from global state
    const { currentTargetId, currentTargetName } = appState;

    // Use passedParams if available, otherwise fallback to global state (for initial load or non-filter-driven reloads)
    const activeParams = passedParams || appState.paginationState.proxyLog;
    const { currentPage, limit, sortBy, sortOrder, filterFavoritesOnly, filterMethod, filterStatus, filterContentType, filterSearchText } = activeParams;

    console.log(`[ProxyLogView] fetchAndDisplayProxyLogs using filterMethod: "${filterMethod}"`, activeParams);
    const globalTableLayouts = appState.globalTableLayouts;
    const tableKey = 'proxyLogTable'; 
    const columnConfig = appState.paginationState.proxyLogTableLayout;

    listDiv.innerHTML = `<p>Fetching proxy logs for target ${escapeHtml(currentTargetName)} (ID: ${currentTargetId}), page ${currentPage}, sort by ${sortBy} ${sortOrder}...</p>`;

    try {
        const params = {
            target_id: currentTargetId,
            page: currentPage,
            limit: limit,
            sort_by: sortBy,
            sort_order: sortOrder,
            favorites_only: filterFavoritesOnly,
            method: filterMethod,
            status: filterStatus,
            type: filterContentType,
            search: filterSearchText
        };
        const apiResponse = await apiService.getProxyLog(params);
        const logs = apiResponse.logs || [];

        stateService.updateState({
            paginationState: {
                proxyLog: {
                    ...appState.paginationState.proxyLog,
                    currentPage: apiResponse.page || 1,
                    totalPages: apiResponse.total_pages || 1,
                    totalRecords: apiResponse.total_records || 0,
                }
            }
        });

        const distinctMethods = (apiResponse.distinct_values?.method || []).filter(val => val !== null && String(val).trim() !== '');
        const distinctStatuses = (apiResponse.distinct_values?.status || []).filter(val => val !== null && String(val).trim() !== '');
        const distinctContentTypes = (apiResponse.distinct_values?.type || []).filter(val => val !== null && val !== '' && String(val).toLowerCase() !== 'all');

        const savedTableWidths = globalTableLayouts[tableKey] || {};
        const sortableHeaders = [
            { key: 'index', label: '#', sortKey: 'id', filter: false }, // Changed key from '#' to 'index'
            { key: 'timestamp', label: 'Timestamp', sortKey: 'timestamp', filter: false },
            { key: 'method', label: 'Method', sortKey: 'request_method', filter: true },
            { key: 'url', label: 'URL', sortKey: 'request_url', filter: false },
            { key: 'status', label: 'Status', sortKey: 'response_status_code', filter: true },
            { key: 'type', label: 'Content-Type', sortKey: 'response_content_type', filter: true },
            { key: 'size', label: 'Size (B)', sortKey: 'response_body_size', filter: false },
            { key: 'actions', label: 'Actions', sortKey: null, filter: false }
        ];
        
        // For debugging, let's log what layouts are available when rendering
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - globalTableLayouts:", JSON.parse(JSON.stringify(globalTableLayouts)));
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - tableKey:", tableKey);
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - savedTableWidths for this tableKey:", JSON.parse(JSON.stringify(savedTableWidths)));
        console.log("[ProxyLogView] fetchAndDisplayProxyLogs - columnConfig (defaults from state):", JSON.parse(JSON.stringify(columnConfig)));

        if (logs.length > 0) {
            let tableHTML = `<table style="table-layout: fixed;"><thead id="proxyLogTableHead"><tr>`;
            sortableHeaders.forEach(h => {
                let classes = h.sortKey ? 'sortable' : '';
                if (h.sortKey === sortBy) classes += sortOrder === 'ASC' ? ' sorted-asc' : ' sorted-desc';
                let filterDropdownHTML = '';
                const colKey = h.key; // Use the key directly
                let thStyleWidth;
                if (colKey === 'actions') {
                    thStyleWidth = '110px'; // Fixed width for the Actions column
                } else {
                    thStyleWidth = savedTableWidths[colKey] || columnConfig[colKey]?.default || 'auto';
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
                console.log(`[ProxyLogView] Header: ${h.label}, colKey: '${colKey}', savedWidth: '${savedTableWidths[colKey]}', defaultWidth: '${columnConfig[colKey]?.default}', finalWidth: '${thStyleWidth}'`);

                tableHTML += `<th style="width: ${thStyleWidth};" class="${classes}" ${h.sortKey ? `data-sort-key="${h.sortKey}"` : ''} data-col-key="${colKey}" id="${columnConfig[colKey]?.id || 'col-proxylog-' + colKey}">${h.label}${filterDropdownHTML}</th>`;
            });
            tableHTML += `</tr></thead><tbody>`;
            logs.forEach((log, index) => {
                let itemIndex;
                const { totalRecords: currentTotalRecords, currentPage: currentDisplayPage } = appState.paginationState.proxyLog;
                if (sortBy === 'id' && sortOrder === 'DESC') {
                    itemIndex = currentTotalRecords - ((currentDisplayPage - 1) * limit) - index;
                } else {
                    itemIndex = (currentDisplayPage - 1) * limit + index + 1;
                }
                const safeURL = escapeHtml(log.request_url);
                const ts = log.timestamp ? new Date(log.timestamp).toLocaleString() : 'N/A';
                tableHTML += `<tr>
                    <td>${itemIndex}</td><td>${ts}</td><td>${escapeHtml(log.request_method)}</td>
                    <td class="proxy-log-url-cell" title="${safeURL}">${safeURL}</td>
                    <td>${log.response_status_code || '-'}</td>
                    <td title="${escapeHtmlAttribute(log.response_content_type || '-')}">${escapeHtml(log.response_content_type?.substring(0,30) || '-')}${log.response_content_type && log.response_content_type.length > 30 ? '...' : ''}</td>
                    <td>${log.response_body_size || 0}</td>
                    <td class="actions-cell">
                        <span class="favorite-toggle table-row-favorite-toggle ${log.is_favorite ? 'favorited' : ''}" data-log-id="${log.id}" data-is-favorite="${log.is_favorite ? 'true' : 'false'}" title="Toggle Favorite" style="cursor: pointer; margin-right: 8px; font-size: 1.2em; vertical-align: middle;">${log.is_favorite ? '★' : '☆'}</span>
                        <button class="action-button view-log-detail" data-log-id="${log.id}" title="View Details">👁️</button>                        
                        <button class="action-button add-to-sitemap" data-log-id="${log.id}" data-log-method="${escapeHtmlAttribute(log.request_method)}" data-log-path="${escapeHtmlAttribute(log.request_url.split('?')[0])}" title="Add to Sitemap">🗺️</button>
                        <button class="action-button more-actions" data-log-id="${log.id}" data-log-method="${escapeHtmlAttribute(log.request_method)}" data-log-path="${escapeHtmlAttribute(log.request_url.split('?')[0])}" title="More Actions">⋮</button>
                    </td></tr>`;
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
                    tableService.makeTableColumnsResizable('proxyLogTableHead');
                }
            } else if (logs.length > 0) { 
                console.error("[ProxyLogView] Table head 'proxyLogTableHead' not found after rendering table (using requestAnimationFrame).");
            }

            listDiv.querySelectorAll('.view-log-detail').forEach(button => button.addEventListener('click', handleViewLogDetail));
            listDiv.querySelectorAll('.table-row-favorite-toggle').forEach(starBtn => starBtn.addEventListener('click', handleProxyLogFavoriteToggle));
            listDiv.querySelectorAll('.add-to-sitemap').forEach(button => button.addEventListener('click', openAddToSitemapModal));
            listDiv.querySelectorAll('.more-actions').forEach(button => button.addEventListener('click', openMoreActionsDropdown));
        }, 0);

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
            <li><a href="#" data-action="add-to-sitemap-dropdown" data-log-id="${logId}">Add to Sitemap</a></li>
            <li><a href="#" data-action="send-to-findings" data-log-id="${logId}">Send to Findings (TBD)</a></li>
            <li><a href="#" data-action="send-to-repeater">Send to Repeater (TBD)</a></li>
            <li><a href="#" data-action="run-gf">Run GF Patterns (TBD)</a></li>
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


    dropdown.querySelector('a[data-action="add-to-sitemap-dropdown"]').addEventListener('click', (e) => {
        e.preventDefault();
        openAddToSitemapModal({ currentTarget: button }); // Reuse existing modal logic, pass original button as target
        closeMoreActionsDropdown();
    });
    // Placeholder for "Send to Findings"
    dropdown.querySelector('a[data-action="send-to-findings"]').addEventListener('click', (e) => {
        e.preventDefault();
        uiService.showModalMessage("Not Implemented", `Sending log ID ${logId} to Findings is not yet implemented.`);
        closeMoreActionsDropdown();
    });

    // Add a slight delay before attaching the outside click listener
    // to prevent it from firing immediately from the same click that opened it.
    setTimeout(() => document.addEventListener('click', closeMoreActionsDropdownOnClickOutside), 0);
}

function openAddToSitemapModal(event) {
    const button = event.currentTarget;
    const logId = button.getAttribute('data-log-id');
    const logMethod = button.getAttribute('data-log-method');
    const logPath = button.getAttribute('data-log-path');

    // Create modal HTML
    const modalHTML = `
        <div id="addToSitemapModal" class="modal-overlay" style="display:flex;">
            <div class="modal-content" style="width: 500px;">
                <div class="modal-header">
                    <h2>Add to Sitemap</h2>
                    <span class="modal-close-btn" id="closeAddToSitemapModalBtn">&times;</span>
                </div>
                <div class="modal-body">
                    <p><strong>Log Entry:</strong> ${escapeHtml(logMethod)} ${escapeHtml(logPath)}</p>
                    <input type="hidden" id="sitemapLogId" value="${logId}">
                    <div class="form-group">
                        <label for="sitemapFolderPath">Folder Path:</label>
                        <input type="text" id="sitemapFolderPath" name="sitemapFolderPath" value="/" required>
                        <small>Define a hierarchical path (e.g., /api/v1/users/). The actual endpoint will be listed under this.</small>
                    </div>
                    <div class="form-group">
                        <label for="sitemapNotes">Notes (Optional):</label>
                        <textarea id="sitemapNotes" name="sitemapNotes" rows="3"></textarea>
                    </div>
                </div>
                <div class="modal-footer">
                    <button id="cancelAddToSitemapBtn" class="secondary">Cancel</button>
                    <button id="saveToSitemapBtn" class="primary">Save to Sitemap</button>
                </div>
            </div>
        </div>
    `;

    // Append modal to body (or a dedicated modal container if you have one)
    document.body.insertAdjacentHTML('beforeend', modalHTML);

    // Add event listeners
    document.getElementById('closeAddToSitemapModalBtn').addEventListener('click', closeAddToSitemapModal);
    document.getElementById('cancelAddToSitemapBtn').addEventListener('click', closeAddToSitemapModal);
    document.getElementById('saveToSitemapBtn').addEventListener('click', handleSaveSitemapEntry);
    document.getElementById('addToSitemapModal').addEventListener('click', (e) => {
        if (e.target.id === 'addToSitemapModal') { // Click on overlay
            closeAddToSitemapModal();
        }
    });
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
    uiService.showModalMessage("Refreshing...", "Reloading proxy logs with current filters...", true); // true for auto-hide
    const currentProxyLogParams = stateService.getState().paginationState.proxyLog;
    await fetchAndDisplayProxyLogs(currentProxyLogParams);
    // The modal will auto-hide if uiService is set up for it,
    // otherwise, you might need a uiService.hideModal() or similar if fetchAndDisplayProxyLogs doesn't handle it.
    // For now, assuming a short-lived message.
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
    const initialControlParams = proxyLogParams || appState.paginationState.proxyLog;
    const { filterFavoritesOnly, filterSearchText } = initialControlParams;
    const tableKey = 'proxyLogTable';

    const targetInfo = currentTargetId ? `for Target: ${escapeHtml(currentTargetName)} (ID: ${currentTargetId})` : '(No Target Selected)';
    viewContentContainer.innerHTML = `
        <h1>Proxy Log ${targetInfo}</h1>
        <div style="margin-bottom: 15px; display: flex; align-items: center; gap: 20px;">
            <div class="form-group" style="display: flex; align-items: center; margin-bottom: 0;">
                <input type="checkbox" id="filterFavoritesToggle" style="margin-right: 5px;" ${filterFavoritesOnly ? 'checked' : ''}>
                <label for="filterFavoritesToggle" style="font-weight: normal;">Favorites Only</label>
            </div>
            <div class="form-group" style="flex-grow: 1; margin-bottom: 0;">
                <input type="search" id="proxyLogSearchInput" placeholder="Search URL, Headers, Body..." value="${escapeHtmlAttribute(filterSearchText)}" style="width: 100%; padding: 6px 10px; border-radius: 4px; border: 1px solid #bdc3c7;">
            </div>
        </div>
        <div style="margin-bottom: 10px;">
            <button id="refreshProxyLogBtn" class="secondary small-button" title="Refresh Logs" style="margin-right: 10px;">🔄</button>
            <button id="saveProxyLogLayoutBtn" class="secondary small-button" style="margin-right: 10px;">Save Column Layout</button>
            <button id="deleteAllTargetLogsBtn" class="secondary small-button" ${!currentTargetId ? 'disabled title="No target selected"' : `title="Delete all logs for ${escapeHtml(currentTargetName)}"`}>
                Delete All Logs for Target
            </button>
        </div>
        <div id="proxyLogList">Loading proxy logs...</div>
        <div id="proxyLogPaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
    `;

    if (!currentTargetId) {
        document.getElementById('proxyLogList').innerHTML = '<p>Please set a current target to view its proxy log.</p>';
        document.getElementById('proxyLogPaginationControls').innerHTML = '';
        return;
    }

    // Pass the specific params that triggered this load
    fetchAndDisplayProxyLogs(proxyLogParams); 
    document.getElementById('filterFavoritesToggle')?.addEventListener('change', handleProxyLogFavoriteFilterChange);
    document.getElementById('proxyLogSearchInput')?.addEventListener('input', debounce(handleProxyLogSearch, 300));
    document.getElementById('saveProxyLogLayoutBtn')?.addEventListener('click', () => {
        if (tableService) {
            tableService.saveCurrentTableLayout(tableKey, 'proxyLogTableHead');
        }
    });
    const deleteAllBtn = document.getElementById('deleteAllTargetLogsBtn');
    if (deleteAllBtn) {
        deleteAllBtn.addEventListener('click', handleDeleteAllTargetLogs);
    }
    const refreshBtn = document.getElementById('refreshProxyLogBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', handleRefreshProxyLog); // Listener remains the same
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
        resultsContentDiv.innerHTML = '';
        let currentJsAnalysisData = [];

        if (responseData.message) {
            const p = document.createElement('p');
            p.className = 'message-area info-message';
            p.innerHTML = escapeHtml(responseData.message);
            resultsContentDiv.appendChild(p);
        }
        if (responseData.results && Object.keys(responseData.results).length > 0) {
            for (const category in responseData.results) {
                if (responseData.results[category].length > 0) {
                    responseData.results[category].forEach(item => currentJsAnalysisData.push({ category, finding: item }));
                }
            }
        }
        stateService.updateState({ jsAnalysisDataCache: { [logIdStr]: currentJsAnalysisData } });
        if (currentJsAnalysisData.length > 0) {
            renderJsAnalysisTable(logIdStr);
        } else if (!responseData.message) {
            resultsContentDiv.innerHTML += `<p>No specific items extracted by the analysis tool.</p>`;
        }
    } catch (error) {
        resultsContentDiv.innerHTML = `<p class="error-message">Error analyzing log #${logIdStr}: ${escapeHtml(error.message)}</p>`;
    }
}

function renderJsAnalysisTable(logIdStr) {
    const resultsContentDiv = document.getElementById('jsAnalysisResultsContent');
    if (!resultsContentDiv) return;

    const appState = stateService.getState();
    const currentLogAnalysisData = appState.jsAnalysisDataCache[logIdStr];
    const currentSortState = appState.jsAnalysisSortState;

    if (!currentLogAnalysisData) {
        resultsContentDiv.innerHTML = "<p>No analysis data available for this log entry.</p>";
        return;
    }
    let existingMessageHTML = resultsContentDiv.querySelector('.message-area')?.outerHTML || '';
    if (currentLogAnalysisData.length === 0) {
        resultsContentDiv.innerHTML = existingMessageHTML + "<p>No analysis data to display.</p>";
        return;
    }

    const sortedData = [...currentLogAnalysisData].sort((a, b) => {
        const valA = a[currentSortState.sortBy];
        const valB = b[currentSortState.sortBy];
        let comparison = 0;
        if (valA > valB) comparison = 1;
        else if (valA < valB) comparison = -1;
        return currentSortState.sortOrder === 'ASC' ? comparison : comparison * -1;
    });

    let tableHTML = `<table><thead><tr>
        <th class="sortable" data-sort-key="category">Category</th>
        <th class="sortable" data-sort-key="finding">Finding</th>
        </tr></thead><tbody>`;
    sortedData.forEach(item => tableHTML += `<tr><td>${escapeHtml(item.category)}</td><td>${escapeHtml(item.finding)}</td></tr>`);
    tableHTML += `</tbody></table>`;
    resultsContentDiv.innerHTML = existingMessageHTML + tableHTML;

    resultsContentDiv.querySelectorAll('th.sortable').forEach(th => {
        th.classList.toggle('sorted-asc', currentSortState.sortBy === th.dataset.sortKey && currentSortState.sortOrder === 'ASC');
        th.classList.toggle('sorted-desc', currentSortState.sortBy === th.dataset.sortKey && currentSortState.sortOrder === 'DESC');
        th.addEventListener('click', (event) => handleJsAnalysisSort(event, logIdStr));
    });
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

function convertJsAnalysisToCSV(jsonData) {
    const headersConfig = [{ key: 'category', label: 'Category' }, { key: 'finding', label: 'Finding' }];
    const headerRow = headersConfig.map(h => escapeHtml(h.label)).join(',');
    const dataRows = jsonData.map(item => headersConfig.map(header => escapeHtml(item[header.key])).join(','));
    return [headerRow].concat(dataRows).join('\n');
}

function handleExportJsAnalysisToCSV(event) {
    const logIdStr = event.target.getAttribute('data-log-id');
    const appState = stateService.getState();
    const currentLogAnalysisData = appState.jsAnalysisDataCache[logIdStr];

    if (!currentLogAnalysisData || currentLogAnalysisData.length === 0) {
        uiService.showModalMessage("No Data", "No JavaScript analysis data available to export.");
        return;
    }
    uiService.showModalMessage("Exporting...", "Preparing CSV data...");
    const csvString = convertJsAnalysisToCSV(currentLogAnalysisData);
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
    
    try {
        const logEntry = await apiService.getProxyLogDetail(logId, navParams);
        let reqHeaders = {}; try { reqHeaders = JSON.parse(logEntry.request_headers || '{}'); } catch(e) { console.warn("Error parsing request headers JSON", e); }
        let resHeaders = {}; try { resHeaders = JSON.parse(logEntry.response_headers || '{}'); } catch(e) { console.warn("Error parsing response headers JSON", e); }

        viewContentContainer.innerHTML = `
            <div class="log-detail-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                <h1>Log Entry Detail: #${logEntry.id}
                    <button id="analyzeJsBtn" class="secondary small-button" data-log-id="${logEntry.id}" style="margin-left: 15px;">Analyze JS</button>
                    <span id="favoriteToggleBtn" class="favorite-toggle ${logEntry.is_favorite ? 'favorited' : ''}" data-log-id="${logEntry.id}" data-is-favorite="${logEntry.is_favorite}" title="Toggle Favorite" style="margin-left: 10px; font-size: 1.2em; vertical-align: middle;">${logEntry.is_favorite ? '★' : '☆'}</span>
                </h1>
                <div class="log-navigation">
                    ${logEntry.prev_log_id ? `<button id="prevLogBtn" class="secondary" data-log-id="${logEntry.prev_log_id}" title="Previous Log Entry">&laquo; Previous</button>` : ''}
                    ${logEntry.next_log_id ? `<button id="nextLogBtn" class="secondary" data-log-id="${logEntry.next_log_id}" title="Next Log Entry" style="margin-left: ${logEntry.prev_log_id ? '5px' : '0'};">Next &raquo;</button>` : ''}
                </div>
            </div>
            <div class="log-meta-info" style="margin-bottom: 15px; padding: 10px; background-color: #f9f9f9; border-radius: 4px;">
                <p><strong>Timestamp:</strong> ${new Date(logEntry.timestamp).toLocaleString()}</p>
                <p><strong>URL:</strong> ${escapeHtml(logEntry.request_url)}</p>
                <p><strong>Method:</strong> ${escapeHtml(logEntry.request_method)}</p>
                <p><strong>Status:</strong> ${logEntry.response_status_code || '-'}</p>
                <p><strong>Duration:</strong> ${logEntry.duration_ms || 0} ms</p>
                ${logEntry.target_id ? `<p><strong>Target ID:</strong> ${logEntry.target_id}</p>` : ''}
            </div>
            <div class="tabs">
                <button class="tab-button" data-tab="requestTab">Request</button>
                <button class="tab-button" data-tab="responseTab">Response</button>
                <button class="tab-button" data-tab="jsAnalysisTab">JS Analysis</button>
            </div>
            <div id="requestTab" class="tab-content"><h3>Request Details</h3><p><strong>HTTP Version:</strong> ${escapeHtml(logEntry.request_http_version)}</p><h4>Headers:</h4><pre class="headers-box">${formatHeaders(reqHeaders)}</pre><h4>Body:</h4><pre class="body-box">${formatBody(logEntry.request_body, reqHeaders['Content-Type']?.[0])}</pre></div>
            <div id="responseTab" class="tab-content"><h3>Response Details</h3><p><strong>HTTP Version:</strong> ${escapeHtml(logEntry.response_http_version)}</p><h4>Headers:</h4><pre class="headers-box">${formatHeaders(resHeaders)}</pre><h4>Body: (${logEntry.response_body_size} bytes)</h4><pre class="body-box">${formatBody(logEntry.response_body, logEntry.response_content_type)}</pre></div>
            <div id="jsAnalysisTab" class="tab-content"><h3>JavaScript Analysis Results</h3><div style="margin-bottom: 10px;"><button id="exportJsAnalysisCsvBtn" class="secondary small-button" data-log-id="${logEntry.id}">Export to CSV</button></div><div id="jsAnalysisResultsContent"><p>Click "Analyze JS" to perform analysis.</p></div></div>
            <div class="notes-section" style="margin-top: 20px;"><h3>Notes:</h3><textarea id="logEntryNotes" rows="5" style="width: 100%;">${escapeHtml(logEntry.notes || '')}</textarea><button id="saveLogEntryNotesBtn" class="primary" data-log-id="${logEntry.id}" style="margin-top: 10px;">Save Notes</button><div id="saveNotesMessage" class="message-area" style="margin-top: 5px;"></div></div>`;

        document.querySelectorAll('.tab-button').forEach(button => {
            button.addEventListener('click', () => {
                document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
                button.classList.add('active');
                document.getElementById(button.getAttribute('data-tab')).classList.add('active');
                // If JS Analysis tab is clicked, and no data, trigger analysis
                if (button.getAttribute('data-tab') === 'jsAnalysisTab' && !appState.jsAnalysisDataCache[String(logId)]) {
                    document.getElementById('analyzeJsBtn')?.click();
                }
            });
        });
        document.getElementById('analyzeJsBtn')?.addEventListener('click', handleAnalyzeJS);
        document.getElementById('exportJsAnalysisCsvBtn')?.addEventListener('click', handleExportJsAnalysisToCSV);
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
        if (appState.jsAnalysisDataCache[logIdString] && appState.jsAnalysisDataCache[logIdString].length > 0) {
            renderJsAnalysisTable(logIdString);
        }

        // Activate tab based on hash parameter
        const tabToActivate = requestedTab || 'requestTab';
        const tabButtonToActivate = document.querySelector(`.tab-button[data-tab="${tabToActivate}"]`);
        if (tabButtonToActivate) {
            tabButtonToActivate.click(); // This will also trigger analysis if it's the JS tab and no data
        } else {
            // Default to requestTab if specified tab is invalid
            document.querySelector('.tab-button[data-tab="requestTab"]')?.click();
        }
    } catch (error) {
        viewContentContainer.innerHTML = `<h1>Log Entry Detail</h1><p class="error-message">Error loading details for Log ID ${logId}: ${escapeHtml(error.message)}</p>`;
    }
}
