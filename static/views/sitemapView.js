import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
// let tableService; // Not directly used for the tree itself

// DOM element references
let viewContentContainer;
const sitemapState = {}; // For storing expanded/collapsed states by fullPath (e.g., sitemapState['/api/users'] = true)
let fullSitemapData = []; // To store the complete sitemap data for the current target
let targetExpansionLevel = 0; // 0 means only hosts (or first level if no hosts) are visible
const sitemapEndpointsVisibleState = {}; // For storing visibility of direct endpoints for a node

/**
 * Initializes the Sitemap View module with necessary services.
 * @param {Object} services - An object containing service instances.
 */
export function initSitemapView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    // tableService = services.tableService;
    console.log("[SitemapView] Initialized.");
}

/**
 * Loads the sitemap view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */

function toggleNodeExpansion(event) {
    const toggler = event.currentTarget;
    const nodeElement = toggler.closest('.sitemap-node');
    if (!nodeElement) return;

    const fullPath = nodeElement.dataset.fullPath;
    const isExpanded = nodeElement.classList.toggle('expanded');
    toggler.textContent = isExpanded ? '▼' : '►';
    sitemapState[fullPath] = isExpanded; // Store state

    const childrenContainer = nodeElement.querySelector('.sitemap-node-children');
    if (childrenContainer) {
        childrenContainer.style.display = isExpanded ? 'block' : 'none';
    }
}

async function handleSitemapSendToModifierClick(event) {
    const button = event.currentTarget;
    const logId = button.dataset.logId;

    if (logId && logId !== "0") {
        try {
            button.disabled = true;
            button.style.opacity = '0.5';
            const task = await apiService.addModifierTask({ http_traffic_log_id: parseInt(logId, 10) });
            uiService.showModalMessage("Sent to Modifier", `Task "${escapeHtml(task.name || `Task ${task.id}`)}" sent to Modifier. Navigating...`, true, 1500);
            window.location.hash = `#modifier?task_id=${task.id}`;
        } catch (error) {
            console.error("Error sending log to modifier:", error);
            uiService.showModalMessage("Error", `Failed to send to Modifier: ${escapeHtml(error.message)}`);
        } finally {
            button.disabled = false;
            button.style.opacity = '1';
        }
    } else {
        uiService.showModalMessage("Info", "This sitemap entry is not linked to a specific proxy log to send to Modifier.");
    }
}

function renderSitemapTree(nodes, parentElement, level = 0) {
    if (!nodes || nodes.length === 0) {
        if (level === 0 && parentElement.classList.contains('sitemap-tree-container')) {
             parentElement.innerHTML = '<p>Sitemap is empty or no data received.</p>';
        }
        return;
    }

    const ul = document.createElement('ul');
    ul.className = 'sitemap-tree';
    if (level > 0) { // Apply indentation for sub-levels
        // ul.style.paddingLeft = '20px'; // Handled by CSS now
    }

    nodes.forEach(node => {
        const li = document.createElement('li');
        li.className = 'sitemap-node';
        li.dataset.fullPath = node.full_path;

        let nodeHTML = '';
        const hasChildren = node.children && node.children.length > 0;
        const hasEndpoints = node.endpoints && node.endpoints.length > 0;

        const isExpanded = sitemapState[node.full_path] === true;
        const areEndpointsVisible = sitemapEndpointsVisibleState[node.full_path] === true; // For direct endpoints
        if (isExpanded) {
            li.classList.add('expanded');
        }

        // Determine node type for styling/icon. Assume backend adds `node.type = 'host'` for top-level domain/subdomain nodes.
        // If not, we can infer it if level === 0.
        const isHostNode = node.type === 'host' || (level === 0 && !node.type); // Infer if type not present at level 0

        nodeHTML += `<div class="sitemap-node-header ${isHostNode ? 'sitemap-host-node-header' : ''}">`;
        nodeHTML += `<span class="sitemap-node-toggle">${hasChildren ? (isExpanded ? '▼' : '►') : ''}</span>`;
        
        if (isHostNode) {
            nodeHTML += `<span class="sitemap-node-icon">🌐</span>`; // Globe icon for hosts
        } else {
            nodeHTML += `<span class="sitemap-node-icon">${hasChildren ? '📁' : '📄'}</span>`; // Folder or File icon for paths
        }
        nodeHTML += `<span class="sitemap-node-name">${escapeHtml(node.name)}</span>`;
        if (node.is_manually_added && node.manual_entry_id && !hasEndpoints && !hasChildren) {
            nodeHTML += ` <small class="manual-indicator" title="Manually added path">(manual path)</small>`;
        }
        // Add an indicator if endpoints are visible/hidden, only if there are endpoints
        if (hasEndpoints) {
            nodeHTML += ` <span class="sitemap-endpoints-toggle-indicator">${areEndpointsVisible ? '📂' : '📁'}</span>`;
        }
        nodeHTML += `</div>`;
        li.innerHTML = nodeHTML;

        if (hasEndpoints) {
            const endpointsDiv = document.createElement('div');
            endpointsDiv.className = 'sitemap-node-endpoints';
            endpointsDiv.style.display = areEndpointsVisible ? 'block' : 'none';
            let endpointsListHTML = '<ul>';
            node.endpoints.forEach(ep => {
                let title = `Log ID: ${ep.http_traffic_log_id?.Int64 || 'N/A'}\nStatus: ${ep.status_code?.Int64 || 'N/A'}\nSize: ${ep.response_size?.Int64 || 'N/A'} bytes`;
                if (ep.is_manually_added) {
                    const manualIdDisplay = (ep.manual_entry_id && ep.manual_entry_id.Valid) ? ep.manual_entry_id.Int64 : '-';
                    title = `Manually Added (ID: ${manualIdDisplay})\nNotes: ${ep.manual_entry_notes?.String || 'N/A'}`;
                }
                
                let logIdDisplay = '-';
                if (ep.http_traffic_log_id && ep.http_traffic_log_id.Valid) {
                    logIdDisplay = ep.http_traffic_log_id.Int64;
                }

                const statusCodeDisplay = ep.status_code && ep.status_code.Valid ? ep.status_code.Int64 : '-';
                const responseSizeDisplay = ep.response_size && ep.response_size.Valid ? ep.response_size.Int64 : '-';
                const isFavorite = ep.is_favorite && ep.is_favorite.Valid && ep.is_favorite.Bool;

                let endpointDetails = `[sc: ${statusCodeDisplay}; cl: ${responseSizeDisplay}; id: ${logIdDisplay}]`;
                let logDetailLink = '';

                if (ep.is_manually_added) {
                    const manualIdDisplay = (ep.manual_entry_id && ep.manual_entry_id.Valid) ? ep.manual_entry_id.Int64 : '-';
                    endpointDetails = `[Manual Entry ID: ${manualIdDisplay}]`;
                    // Link to original log if http_traffic_log_id is present and valid for a manual entry
                    if (ep.http_traffic_log_id && ep.http_traffic_log_id.Valid) { 
                         logDetailLink = `<a href="#proxy-log-detail?id=${ep.http_traffic_log_id.Int64}" class="sitemap-log-link" title="View Original Log (ID: ${ep.http_traffic_log_id.Int64})">👁️</a>`;
                    }
                } else if (ep.http_traffic_log_id && ep.http_traffic_log_id.Valid) { // For non-manual entries
                    logDetailLink = `<a href="#proxy-log-detail?id=${ep.http_traffic_log_id.Int64}" class="sitemap-log-link" title="View Log (ID: ${ep.http_traffic_log_id.Int64})">👁️</a>`;
                }

                const favoriteStar = `<span class="favorite-toggle sitemap-favorite-toggle ${isFavorite ? 'favorited' : ''}" data-log-id="${ep.http_traffic_log_id?.Int64 || ''}" data-is-favorite="${isFavorite}" title="Toggle Favorite" style="cursor: pointer; margin-right: 5px;">${isFavorite ? '★' : '☆'}</span>`;
                
                const actionButtonsHTML = `
                    <button class="action-button sitemap-send-to-modifier" data-log-id="${ep.http_traffic_log_id?.Int64 || ''}" title="Send to Modifier" style="${!(ep.http_traffic_log_id && ep.http_traffic_log_id.Valid) ? 'display:none;' : ''}">🔧</button>
                `;

                endpointsListHTML += `<li class="sitemap-endpoint ${ep.is_manually_added ? 'manual-endpoint' : ''}" title="${escapeHtmlAttribute(title)}">
                    <span class="sitemap-actions-cell">${favoriteStar}${logDetailLink}${actionButtonsHTML}</span><span class="endpoint-method method-${escapeHtmlAttribute(ep.method.toLowerCase())}">${escapeHtml(ep.method)}</span>${endpointDetails}<span class="endpoint-path">${escapeHtml(ep.path)}</span>
                    ${ep.is_manually_added ? `<span class="manual-indicator" title="${escapeHtmlAttribute(ep.manual_entry_notes?.String || "Manually added")}">(manual)</span>` : ''}
                </li>`;
            });
            endpointsListHTML += '</ul>';
            endpointsDiv.innerHTML = endpointsListHTML;
            li.appendChild(endpointsDiv);
        }

        if (hasChildren) {
            const childrenDiv = document.createElement('div');
            childrenDiv.className = 'sitemap-node-children';
            if (isHostNode) { // Add a specific class for children of a host node if needed for styling
                childrenDiv.classList.add('sitemap-host-children');
            }
            childrenDiv.style.display = isExpanded ? 'block' : 'none';
            renderSitemapTree(node.children, childrenDiv, level + 1);
            li.appendChild(childrenDiv);
        }
        ul.appendChild(li);
    });
    parentElement.appendChild(ul);

    ul.querySelectorAll('.sitemap-node-toggle').forEach(toggler => {
        if(toggler.textContent !== '') { // Only add listener if there's a toggle symbol
            toggler.removeEventListener('click', toggleNodeExpansion);
            toggler.addEventListener('click', toggleNodeExpansion);
        }
    });

    ul.querySelectorAll('.sitemap-node-header').forEach(header => {
        const nodeLi = header.closest('.sitemap-node');
        const nodeData = nodes.find(n => n.full_path === nodeLi.dataset.fullPath); // Find the node data
        if (nodeData && nodeData.endpoints && nodeData.endpoints.length > 0) { // Only if it has endpoints
            header.removeEventListener('click', handleToggleEndpointsVisibility);
            header.addEventListener('click', handleToggleEndpointsVisibility);
            header.classList.add('clickable-for-endpoints'); // For CSS cursor styling
        }
    });
    attachSitemapActionListeners(ul);
}

function applyExpansionLevel() {
    const appState = stateService.getState();
    let dataToProcess = fullSitemapData;
    if (appState.selectedSitemapHost && fullSitemapData.length > 0) {
        dataToProcess = fullSitemapData.filter(node => node.name === appState.selectedSitemapHost);
    }

    // Recursive function to set expansion state based on targetExpansionLevel
    function setExpansion(nodes, currentDepth) {
        nodes.forEach(node => {
            if (node.children && node.children.length > 0) {
                // Expand if currentDepth is less than the targetExpansionLevel
                sitemapState[node.full_path] = (currentDepth < targetExpansionLevel);
                setExpansion(node.children, currentDepth + 1);
            }
            // Endpoint visibility (sitemapEndpointsVisibleState) is not managed by this function.
            // It's toggled by clicking the node header or "Expand/Collapse All".
        });
    }

    setExpansion(dataToProcess, 0); // Start with depth 0 for host nodes (or first level of paths)
}

/**
 * Displays a message in the sitemap message area with a close button and optional auto-hide.
 * @param {HTMLElement} messageAreaElement - The DOM element for the message area.
 * @param {string} text - The message text.
 * @param {string} className - The class to apply (e.g., 'success-message', 'error-message', 'info-message').
 * @param {number} autoHideTimeout - Milliseconds to auto-hide the message (0 for no auto-hide).
 */
function displaySitemapMessage(messageAreaElement, text, className, autoHideTimeout = 0) {
    if (!messageAreaElement) return;

    messageAreaElement.innerHTML = ''; // Clear previous content
    messageAreaElement.className = 'message-area'; // Reset class first

    const textSpan = document.createElement('span');
    textSpan.textContent = text; // Text content is set directly

    const closeButton = document.createElement('span');
    closeButton.innerHTML = '&times;'; // 'X' character
    closeButton.className = 'message-close-button';
    closeButton.title = 'Close message';
    
    let timeoutId = null;
    const clearMessage = () => {
        messageAreaElement.innerHTML = '';
        messageAreaElement.className = 'message-area';
        messageAreaElement.style.display = 'none';
        if (timeoutId) clearTimeout(timeoutId);
    };
    closeButton.onclick = clearMessage;

    messageAreaElement.appendChild(textSpan);
    messageAreaElement.appendChild(closeButton);
    messageAreaElement.classList.add(className);
    messageAreaElement.style.display = 'block';

    if (autoHideTimeout > 0) {
        timeoutId = setTimeout(clearMessage, autoHideTimeout);
    }
}

async function renderFilteredSitemapView() {
    viewContentContainer = document.getElementById('viewContentContainer'); // Ensure it's up-to-date
    const treeContainer = document.getElementById('sitemapTreeContainer');
    const messageArea = document.getElementById('sitemapMessage');

    if (!viewContentContainer || !treeContainer || !messageArea) {
        console.error("SitemapView: Required DOM elements not found for rendering.");
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId } = appState;

    if (!currentTargetId) {
        treeContainer.innerHTML = `<p>Please select a target to view its sitemap.</p>`;
        messageArea.textContent = '';
        displaySitemapMessage(messageArea, 'Please select a target to view its sitemap.', 'info-message');
        document.getElementById('refreshSitemapBtn').disabled = true;
        document.getElementById('expandAllSitemapBtn').disabled = true;
        document.getElementById('collapseAllSitemapBtn').disabled = true;
        const hostFilterDropdown = document.getElementById('sitemapHostFilter');
        if (hostFilterDropdown) {
            hostFilterDropdown.innerHTML = '<option value="">-- No Target --</option>';
            hostFilterDropdown.disabled = true;
        }
        return;
    }

    treeContainer.innerHTML = ''; // Clear previous content

    // Filter data based on selected host
    let sitemapTreeDataToRender = fullSitemapData;
    if (appState.selectedSitemapHost && fullSitemapData.length > 0) {
        sitemapTreeDataToRender = fullSitemapData.filter(hostNode => hostNode.name === appState.selectedSitemapHost);
    }

    if (sitemapTreeDataToRender.length > 0) {
        renderSitemapTree(sitemapTreeDataToRender, treeContainer);
        displaySitemapMessage(messageArea, 'Sitemap loaded successfully.', 'success-message', 3000);
        document.getElementById('expandAllSitemapBtn').disabled = false;
        document.getElementById('collapseAllSitemapBtn').disabled = false;
    } else {
        if (appState.selectedSitemapHost) {
            treeContainer.innerHTML = `<p>No sitemap data found for host: ${escapeHtml(appState.selectedSitemapHost)}.</p>`;
            displaySitemapMessage(messageArea, `No sitemap data found for host: ${escapeHtml(appState.selectedSitemapHost)}.`, 'info-message');
        } else {
            treeContainer.innerHTML = '<p>No sitemap data found for this target.</p>';
            displaySitemapMessage(messageArea, 'No sitemap data found for this target.', 'info-message');
        }
        document.getElementById('expandAllSitemapBtn').disabled = true;
        document.getElementById('collapseAllSitemapBtn').disabled = true;
    }
}


async function fetchAndPrepareSitemapData(targetId) {
    const treeContainer = document.getElementById('sitemapTreeContainer');
    const messageArea = document.getElementById('sitemapMessage');
    const hostFilterDropdown = document.getElementById('sitemapHostFilter');

    treeContainer.innerHTML = '<p>Fetching sitemap data...</p>';
    displaySitemapMessage(messageArea, 'Fetching sitemap data...', 'info-message');

    try {
        fullSitemapData = await apiService.getGeneratedSitemap(targetId);
        populateHostFilterDropdown(fullSitemapData, hostFilterDropdown);
        
        // Clear previous sitemap expansion states
        Object.keys(sitemapState).forEach(key => delete sitemapState[key]);
        // Clear previous endpoint visibility states
        Object.keys(sitemapEndpointsVisibleState).forEach(key => delete sitemapEndpointsVisibleState[key]);
        
        const currentAppState = stateService.getState(); // Get potentially updated state after dropdown population
        if (currentAppState.selectedSitemapHost && fullSitemapData.some(node => node.name === currentAppState.selectedSitemapHost)) {
            targetExpansionLevel = 100; // Expand all levels for the selected host
        } else {
            targetExpansionLevel = 0; // Default: only hosts (or first level of paths) visible
        }
        applyExpansionLevel(); // Apply this initial level
        await renderFilteredSitemapView(); // Render based on current filter (might be "All")
    } catch (error) {
        console.error("Error fetching sitemap data:", error);
        treeContainer.innerHTML = `<p class="error-message">Error loading sitemap: ${escapeHtml(error.message)}</p>`;
        displaySitemapMessage(messageArea, `Error loading sitemap: ${escapeHtml(error.message)}`, 'error-message');
        fullSitemapData = []; // Clear data on error
        if (hostFilterDropdown) hostFilterDropdown.innerHTML = '<option value="">-- Error --</option>';
    }
}

function populateHostFilterDropdown(sitemapData, dropdownElement) {
    if (!dropdownElement) return;

    const appState = stateService.getState();
    dropdownElement.innerHTML = '<option value="">All Hosts</option>'; // Default option

    if (sitemapData && sitemapData.length > 0) {
        const hostnames = [...new Set(sitemapData.map(node => node.name))].sort();
        hostnames.forEach(hostname => {
            const option = document.createElement('option');
            option.value = hostname;
            option.textContent = escapeHtml(hostname);
            dropdownElement.appendChild(option);
        });
        dropdownElement.disabled = false;
    } else {
        dropdownElement.disabled = true;
    }
    // Set selected value based on state
    if (appState.selectedSitemapHost && dropdownElement.querySelector(`option[value="${appState.selectedSitemapHost}"]`)) {
        dropdownElement.value = appState.selectedSitemapHost;
    } else {
        dropdownElement.value = ""; // Default to "All Hosts"
    }
}

async function handleHostFilterChange(event) {
    const selectedHost = event.target.value || null;
    stateService.updateState({ selectedSitemapHost: selectedHost });
    // Clear existing sitemap states when filter changes to avoid weird expansions
    Object.keys(sitemapState).forEach(key => delete sitemapState[key]);
    Object.keys(sitemapEndpointsVisibleState).forEach(key => delete sitemapEndpointsVisibleState[key]);
    
    if (selectedHost) {
        targetExpansionLevel = 100; // Effectively expand all folders for this host
    } else {
        targetExpansionLevel = 0; // Collapse to show only hosts when "All Hosts"
    }
    applyExpansionLevel(); // Apply the new level
    if (selectedHost && fullSitemapData.length > 0) {
        const hostNodeToExpand = fullSitemapData.find(node => node.name === selectedHost);
        if (hostNodeToExpand) {
            // Temporarily define or reuse setDefaultExpansionRecursive for the selected host
            // This function will mark all child folders of the selected host for expansion.
            function expandHostFolders(nodesToExpand) {
                nodesToExpand.forEach(node => {
                    if (node.children && node.children.length > 0) {
                        sitemapState[node.full_path] = true; // Mark this folder for expansion
                        expandHostFolders(node.children);    // Recurse for its children
                    }
                });
            }
            expandHostFolders([hostNodeToExpand]); // Pass as an array
        }
    }
    await renderFilteredSitemapView(); // Re-render with new expansion states
}

function handleExpandNextLevelSitemap() {
    targetExpansionLevel++;
    applyExpansionLevel();
    renderFilteredSitemapView();
}

function handleCollapseLastLevelSitemap() {
    targetExpansionLevel = Math.max(0, targetExpansionLevel - 1); // Ensure it doesn't go below 0
    applyExpansionLevel();
    renderFilteredSitemapView();
}

function getActiveSitemapTreeContainer() {
    // This function can be expanded if you have multiple sitemap trees in different tabs.
    return document.getElementById('sitemapTreeContainer');
}

async function handleSitemapFavoriteToggle(event) {
    const button = event.currentTarget;
    const logId = button.getAttribute('data-log-id');
    const isCurrentlyFavorite = button.getAttribute('data-is-favorite') === 'true';
    const newFavoriteState = !isCurrentlyFavorite;

    if (!logId || logId === "0" || logId === "") {
        uiService.showModalMessage("Info", "Cannot toggle favorite: No associated log ID.");
        return;
    }

    try {
        await apiService.setProxyLogFavorite(logId, newFavoriteState); // Use existing API service function
        button.innerHTML = newFavoriteState ? '★' : '☆';
        button.classList.toggle('favorited', newFavoriteState);
        button.setAttribute('data-is-favorite', newFavoriteState.toString());
    } catch (favError) {
        console.error("Error toggling favorite from Sitemap:", favError);
        uiService.showModalMessage("Error", `Failed to update favorite status for log ${logId}: ${favError.message}`);
    }
}

// Define handleSitemapLogLinkClick if it's specific to this module and not imported
function handleSitemapLogLinkClick(event) {
    if (event.ctrlKey || event.metaKey) {
        event.preventDefault();
        const fullUrl = event.currentTarget.href;
        window.open(fullUrl, '_blank');
    }
    // Default action (navigate in current tab) is handled by href if not prevented
}

function attachSitemapActionListeners(parentElement) {
    parentElement.querySelectorAll('.sitemap-log-link').forEach(button => {
        button.removeEventListener('click', handleSitemapLogLinkClick);
        button.addEventListener('click', handleSitemapLogLinkClick);
    });

    parentElement.querySelectorAll('.sitemap-send-to-modifier').forEach(button => {
        button.removeEventListener('click', handleSitemapSendToModifierClick);
        button.addEventListener('click', handleSitemapSendToModifierClick);
    });

    parentElement.querySelectorAll('.sitemap-favorite-toggle').forEach(starBtn => {
        starBtn.removeEventListener('click', handleSitemapFavoriteToggle);
        starBtn.addEventListener('click', handleSitemapFavoriteToggle);
    });
}

export async function loadSitemapView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadSitemapView!");
        return;
    }

    if (!apiService || !uiService || !stateService) {
        console.error("SitemapView not initialized. Call initSitemapView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>SitemapView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    let headerHTML = `<h1>Sitemap ${currentTargetId ? `for Target: ${escapeHtml(currentTargetName)} (ID: ${currentTargetId})` : '(No Target Selected)'}</h1>`;
    
    viewContentContainer.innerHTML = `
        ${headerHTML}
        <div style="margin-bottom: 15px; display: flex; align-items: center; gap: 10px;">
            <button id="refreshSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Refresh Sitemap</button>
            <button id="expandAllSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Expand All</button>
            <button id="collapseAllSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Collapse All</button>
            <button id="expandNextLevelSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Expand Next Level</button>
            <button id="collapseLastLevelSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Collapse Last Level</button>
            <div class="form-group" style="margin-left: auto; margin-bottom: 0;"> <!-- Host Filter Dropdown -->
                <label for="sitemapHostFilter" style="margin-right: 5px; font-weight: normal;">Filter by Host:</label>
                <select id="sitemapHostFilter" style="min-width: 200px;">
                    <option value="">All Hosts</option>
                </select>
            </div>
        </div>
        <div id="sitemapMessage" class="message-area" style="margin-bottom: 10px;"></div>
        <div id="sitemapTreeContainer" class="sitemap-tree-container">
            <p>Loading sitemap...</p>
        </div>
    `;
    
    const treeContainer = document.getElementById('sitemapTreeContainer');
    const messageArea = document.getElementById('sitemapMessage');
    const refreshButton = document.getElementById('refreshSitemapBtn');
    const expandAllButton = document.getElementById('expandAllSitemapBtn');
    const collapseAllButton = document.getElementById('collapseAllSitemapBtn');
    const expandNextLevelButton = document.getElementById('expandNextLevelSitemapBtn');
    const collapseLastLevelButton = document.getElementById('collapseLastLevelSitemapBtn');
    const hostFilterDropdown = document.getElementById('sitemapHostFilter');

    if (refreshButton) {
        refreshButton.addEventListener('click', async () => {
            if (currentTargetId) {
                treeContainer.innerHTML = '<p>Refreshing sitemap...</p>';
                messageArea.textContent = '';
                await fetchAndPrepareSitemapData(currentTargetId);
            }
        });
    }

    if (expandAllButton) {
        expandAllButton.addEventListener('click', () => {
            if (currentTargetId) toggleAllSitemapNodes(true, getActiveSitemapTreeContainer());
        });
    }

    if (collapseAllButton) {
        collapseAllButton.addEventListener('click', () => {
            if (currentTargetId) toggleAllSitemapNodes(false, getActiveSitemapTreeContainer());
        });
    }

    if (expandNextLevelButton) {
        expandNextLevelButton.addEventListener('click', handleExpandNextLevelSitemap);
    }

    if (collapseLastLevelButton) {
        collapseLastLevelButton.addEventListener('click', handleCollapseLastLevelSitemap);
    }

    if (hostFilterDropdown) {
        hostFilterDropdown.addEventListener('change', handleHostFilterChange);
    }

    // Initialize targetExpansionLevel based on current state (e.g., selected host)
    // This is now handled within fetchAndPrepareSitemapData
    if (!currentTargetId) {
        treeContainer.innerHTML = `<p>Please select a target to view its sitemap.</p>`;
        if (hostFilterDropdown) {
            hostFilterDropdown.innerHTML = '<option value="">-- No Target --</option>';
            hostFilterDropdown.disabled = true;
        }
        return;
    }
    await fetchAndPrepareSitemapData(currentTargetId);
}

function toggleAllSitemapNodes(expand, treeContainer) {
    if (!treeContainer) return;

    // Update targetExpansionLevel based on "Expand All" or "Collapse All"
    targetExpansionLevel = expand ? 100 : 0; // 100 as a large number for "all levels"

    treeContainer.querySelectorAll('.sitemap-node').forEach(nodeElement => {
        const fullPath = nodeElement.dataset.fullPath;
        const toggler = nodeElement.querySelector('.sitemap-node-toggle');
        const childrenContainer = nodeElement.querySelector('.sitemap-node-children');
        const endpointsContainer = nodeElement.querySelector('.sitemap-node-endpoints');
        const endpointsIndicator = nodeElement.querySelector('.sitemap-endpoints-toggle-indicator');

        if (expand) {
            // Expand folders
            if (childrenContainer) { // Only act if it's a folder node
                nodeElement.classList.add('expanded');
                if (toggler && toggler.textContent !== '') toggler.textContent = '▼';
                sitemapState[fullPath] = true;
                childrenContainer.style.display = 'block';
            }
            // Expand endpoints for *all* nodes that have them
            if (endpointsContainer) {
                sitemapEndpointsVisibleState[fullPath] = true;
                endpointsContainer.style.display = 'block';
                if (endpointsIndicator) endpointsIndicator.textContent = '📂';
            }
        } else {
            // Collapse folders
            nodeElement.classList.remove('expanded');
            if (toggler && childrenContainer && toggler.textContent !== '') toggler.textContent = '►';
            sitemapState[fullPath] = false;
            if (childrenContainer) childrenContainer.style.display = 'none';

            // Collapse endpoints
            if (endpointsContainer) {
                sitemapEndpointsVisibleState[fullPath] = false;
                endpointsContainer.style.display = 'none';
                if (endpointsIndicator) endpointsIndicator.textContent = '📁';
            }
        }
    });
}

function handleToggleEndpointsVisibility(event) {
    // Prevent toggling sub-folders if the click was on the sub-folder toggle itself
    if (event.target.classList.contains('sitemap-node-toggle')) {
        return;
    }

    const headerElement = event.currentTarget; // This is the .sitemap-node-header
    const nodeElement = headerElement.closest('.sitemap-node');
    if (!nodeElement) return;

    const fullPath = nodeElement.dataset.fullPath;
    const endpointsContainer = nodeElement.querySelector('.sitemap-node-endpoints');
    const endpointsIndicator = headerElement.querySelector('.sitemap-endpoints-toggle-indicator');

    if (!endpointsContainer) return; // Should not happen if listener was added correctly

    const currentlyVisible = sitemapEndpointsVisibleState[fullPath] === true;
    const makeVisible = !currentlyVisible;

    sitemapEndpointsVisibleState[fullPath] = makeVisible;
    endpointsContainer.style.display = makeVisible ? 'block' : 'none';
    if (endpointsIndicator) {
        endpointsIndicator.textContent = makeVisible ? '📂' : '📁';
    }
}
