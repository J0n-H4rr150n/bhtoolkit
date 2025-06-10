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
                let title = `Log ID: ${ep.http_traffic_log_id || 'N/A'}\nStatus: ${ep.status_code?.Int64 || ep.status_code?.String || 'N/A'}\nSize: ${ep.response_size?.Int64 || ep.response_size?.String || 'N/A'} bytes`;
                if (ep.is_manually_added) {
                    title = `Manually Added (ID: ${ep.manual_entry_id})\nNotes: ${ep.manual_entry_notes?.String || 'N/A'}`;
                }

                const getNullableValue = (sqlNullable, returnType = 'string') => {
                    if (sqlNullable?.Valid) {
                        if (typeof sqlNullable.Int64 === 'number') { // Covers 0 as well
                            return sqlNullable.Int64;
                        }
                        if (typeof sqlNullable.String === 'string') { // Covers empty string ""
                            return sqlNullable.String;
                        }
                        // If Int64 is 0, it's a valid value.
                        // If Valid is true but neither Int64 nor String has a typical value
                        return returnType === 'number' ? 0 : "";
                    }
                    return returnType === 'number' ? 0 : "";
                };

                let logDetailsDisplay = '';
                if (ep.http_traffic_log_id && ep.http_traffic_log_id !== 0) {
                    const statusCode = getNullableValue(ep.status_code, 'number');
                    const contentLength = getNullableValue(ep.response_size, 'number');
                    logDetailsDisplay = ` <span class="endpoint-log-id">[sc: ${statusCode}; cl: ${contentLength}; id: ${ep.http_traffic_log_id}]</span>`;
                }

                const actionButtonsHTML = `
                    <button class="action-button sitemap-view-log" data-log-id="${ep.http_traffic_log_id}" title="View Log Details" style="${!ep.http_traffic_log_id || ep.http_traffic_log_id === 0 ? 'display:none;' : ''}">👁️</button>
                    <button class="action-button sitemap-send-to-modifier" data-log-id="${ep.http_traffic_log_id}" title="Send to Modifier" style="${!ep.http_traffic_log_id || ep.http_traffic_log_id === 0 ? 'display:none;' : ''}">🔧</button>
                `;

                endpointsListHTML += `<li class="sitemap-endpoint ${ep.is_manually_added ? 'manual-endpoint' : ''}" title="${escapeHtmlAttribute(title)}">
                    <span class="sitemap-actions-cell">${actionButtonsHTML}</span><span class="endpoint-method method-${escapeHtmlAttribute(ep.method.toLowerCase())}">${escapeHtml(ep.method)}</span>${logDetailsDisplay}<span class="endpoint-path">${escapeHtml(ep.path)}</span>
                    ${ep.is_favorite?.Valid && ep.is_favorite.Bool ? '<span class="favorite-indicator">★</span>' : ''}
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
            toggler.removeEventListener('click', toggleNodeExpansion); // Prevent multiple listeners
            toggler.addEventListener('click', toggleNodeExpansion);
        }
    });

    // Add click listener to headers for toggling direct endpoints
    ul.querySelectorAll('.sitemap-node-header').forEach(header => {
        const nodeLi = header.closest('.sitemap-node');
        const nodeData = nodes.find(n => n.full_path === nodeLi.dataset.fullPath); // Find the node data
        if (nodeData && nodeData.endpoints && nodeData.endpoints.length > 0) { // Only if it has endpoints
            header.addEventListener('click', handleToggleEndpointsVisibility);
            header.classList.add('clickable-for-endpoints'); // For CSS cursor styling
        }
    });
    attachSitemapActionListeners(ul);
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

function attachSitemapActionListeners(parentElement) {
    parentElement.querySelectorAll('.sitemap-view-log').forEach(button => {
        button.addEventListener('click', (e) => {
            const logId = e.currentTarget.dataset.logId;
            if (logId && logId !== "0") {
                const detailHashPath = `#proxy-log-detail?id=${logId}`;
                if (e.ctrlKey || e.metaKey) { // Check for Ctrl (Windows/Linux) or Command (Mac) key
                    e.preventDefault(); // Prevent default click behavior if modifier is pressed
                    // Construct the full URL for the new tab
                    const baseUrl = window.location.origin + window.location.pathname.replace(/\/$/, '');
                    const fullUrl = baseUrl + detailHashPath;
                    window.open(fullUrl, '_blank'); // Open in new tab
                } else {
                    // Default action: navigate in the current tab using hash change
                    window.location.hash = detailHashPath;
                }
            } else {
                uiService.showModalMessage("Info", "This sitemap entry is not linked to a specific proxy log.");
            }
        });
    });
    parentElement.querySelectorAll('.sitemap-send-to-modifier').forEach(button => {
        button.addEventListener('click', async (e) => {
            const logId = e.currentTarget.dataset.logId;
             if (logId && logId !== "0") {
                try {
                    // Assuming addModifierTask can take http_traffic_log_id
                    const task = await apiService.addModifierTask({ http_traffic_log_id: parseInt(logId, 10) });
                    uiService.showModalMessage("Sent to Modifier", `Task "${escapeHtml(task.name || `Task ${task.id}`)}" sent to Modifier. Navigating...`, true, 1500);
                    window.location.hash = `#modifier?task_id=${task.id}`;
                } catch (error) {
                    console.error("Error sending log to modifier:", error);
                    uiService.showModalMessage("Error", `Failed to send to Modifier: ${error.message}`);
                }
            } else {
                uiService.showModalMessage("Info", "This sitemap entry is not linked to a specific proxy log to send to Modifier.");
            }
        });
    });
}

function toggleAllSitemapNodes(expand, treeContainer) {
    if (!treeContainer) treeContainer = document.getElementById('sitemapTreeContainer');
    if (!treeContainer) return;

    const allNodesHaveChildren = [];
    const allNodesWithEndpoints = [];

    // First, collect all nodes that have children and all nodes that have endpoints
    // This is a bit inefficient if the tree is huge, might need optimization later
    // For now, this ensures we have the full_path for all relevant nodes.
    function collectNodePaths(nodes, parentPath = '') {
        nodes.forEach(node => {
            if (node.children && node.children.length > 0) {
                allNodesHaveChildren.push(node.full_path);
                collectNodePaths(node.children, node.full_path);
            }
            if (node.endpoints && node.endpoints.length > 0) {
                allNodesWithEndpoints.push(node.full_path);
            }
        });
    }
    // Assuming sitemapTreeData is accessible or passed if needed, for now, we operate on rendered elements

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
