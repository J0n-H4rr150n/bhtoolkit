import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
// let tableService; // Not directly used for the tree itself

// DOM element references
let viewContentContainer;
const sitemapState = {}; // For storing expanded/collapsed states by fullPath (e.g., sitemapState['/api/users'] = true)
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

        nodeHTML += `<div class="sitemap-node-header">`;
        nodeHTML += `<span class="sitemap-node-toggle">${hasChildren ? (isExpanded ? '▼' : '►') : ''}</span>`;
        nodeHTML += `<span class="sitemap-node-icon">${hasChildren ? '📁' : '📄'}</span>`; // Folder or File icon
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
            endpointsDiv.style.display = areEndpointsVisible ? 'block' : 'none'; // Control initial visibility
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

async function fetchAndRenderSitemap(targetId, treeContainer, messageArea) {
    try {
        const sitemapTreeData = await apiService.getGeneratedSitemap(targetId); // New API call
        treeContainer.innerHTML = ''; // Clear previous content (like "Loading...")

        // Clear previous sitemap expansion states
        Object.keys(sitemapState).forEach(key => delete sitemapState[key]);
        // Clear previous endpoint visibility states
        Object.keys(sitemapEndpointsVisibleState).forEach(key => delete sitemapEndpointsVisibleState[key]);


        function setDefaultExpansionRecursive(nodesToExpand) {
            nodesToExpand.forEach(node => {
                if (node.children && node.children.length > 0) {
                    sitemapState[node.full_path] = true; // Mark this folder for expansion
                    setDefaultExpansionRecursive(node.children); // Recurse for its children
                }
            });
        }

        if (sitemapTreeData && sitemapTreeData.length > 0) {
            setDefaultExpansionRecursive(sitemapTreeData); // Set all folders to be expanded by default
            renderSitemapTree(sitemapTreeData, treeContainer);
        } else {
            treeContainer.innerHTML = '<p>No sitemap data found for this target. The proxy log might be empty.</p>';
            document.getElementById('expandAllSitemapBtn').disabled = true;
            document.getElementById('collapseAllSitemapBtn').disabled = true;
        }
        messageArea.textContent = 'Sitemap loaded successfully.';
        messageArea.className = 'message-area success-message';
        setTimeout(() => {
            messageArea.textContent = '';
            messageArea.className = 'message-area';
        }, 3000); // Message disappears after 3 seconds
    } catch (error) {
        console.error("Error fetching or rendering sitemap:", error);
        treeContainer.innerHTML = `<p class="error-message">Error loading sitemap: ${escapeHtml(error.message)}</p>`;
        messageArea.textContent = `Error: ${escapeHtml(error.message)}`;
        document.getElementById('expandAllSitemapBtn').disabled = true;
        document.getElementById('collapseAllSitemapBtn').disabled = true;
        messageArea.className = 'message-area error-message';
    }
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
        <div style="margin-bottom: 15px;">
            <button id="refreshSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Refresh Sitemap</button>
            <button id="expandAllSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''} style="margin-left: 10px;">Expand All</button>
            <button id="collapseAllSitemapBtn" class="secondary small-button" ${!currentTargetId ? 'disabled' : ''}>Collapse All</button>
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

    if (refreshButton) {
        refreshButton.addEventListener('click', async () => {
            if (currentTargetId) {
                treeContainer.innerHTML = '<p>Refreshing sitemap...</p>';
                messageArea.textContent = '';
                await fetchAndRenderSitemap(currentTargetId, treeContainer, messageArea);
            }
        });
    }

    if (expandAllButton) {
        expandAllButton.addEventListener('click', () => {
            if (currentTargetId) toggleAllSitemapNodes(true, treeContainer);
        });
    }

    if (collapseAllButton) {
        collapseAllButton.addEventListener('click', () => {
            if (currentTargetId) toggleAllSitemapNodes(false, treeContainer);
        });
    }

    if (!currentTargetId) {
        treeContainer.innerHTML = `<p>Please select a target to view its sitemap.</p>`;
        return;
    }
    await fetchAndRenderSitemap(currentTargetId, treeContainer, messageArea);
}

function toggleAllSitemapNodes(expand, treeContainer) {
    if (!treeContainer) treeContainer = document.getElementById('sitemapTreeContainer');
    if (!treeContainer) return;

    const allNodesHaveChildren = [];
    const allNodesWithEndpoints = [];

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
                if (toggler) toggler.textContent = '▼';
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
            if (toggler && childrenContainer) toggler.textContent = '►';
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
