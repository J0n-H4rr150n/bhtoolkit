// static/views/visualizerView.js
import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;

// DOM element references
let viewContentContainer;

// Cytoscape instance variables to manage their lifecycle
let sitemapCy = null;
let pageSitemapCy = null;

/**
 * Initializes the Visualizer View module with necessary services.
 * @param {Object} services - An object containing service instances.
 */
export function initVisualizerView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    console.log("[VisualizerView] Initialized.");
}

/**
 * Loads the Visualizer view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadVisualizerView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadVisualizerView!");
        return;
    }

    if (!apiService || !uiService || !stateService) {
        console.error("VisualizerView not initialized. Call initVisualizerView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>VisualizerView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    let headerHTML = `<h1>Visualizer ${currentTargetId ? `for Target: <span class="highlight">${escapeHtml(currentTargetName)}</span> (ID: ${currentTargetId})` : '(No Target Selected)'}</h1>`;

    viewContentContainer.innerHTML = `
        ${headerHTML}
        <div id="visualizerControls" class="control-group">
            <button id="loadSitemapGraphBtn" class="button" ${!currentTargetId ? 'disabled' : ''}>Load Sitemap Graph</button>
            <button id="loadPageSitemapGraphBtn" class="button" style="margin-left: 10px;" ${!currentTargetId ? 'disabled' : ''}>Load Page Sitemap Graph</button>
            <button id="zoomInBtn" class="button" style="margin-left: 20px;" ${!currentTargetId ? 'disabled' : ''}>Zoom In (+)</button>
            <button id="zoomOutBtn" class="button" ${!currentTargetId ? 'disabled' : ''}>Zoom Out (-)</button>
            <button id="resetZoomBtn" class="button" ${!currentTargetId ? 'disabled' : ''}>Reset Zoom</button>
        </div>

        <div id="visualizerMessage" class="message-area" style="margin-bottom: 10px;"></div>

        <div class="tabs">
            <button class="tab-button active" data-tab="sitemapGraphTab">Sitemap Graph</button>
            <button class="tab-button" data-tab="pageSitemapGraphTab">Page Sitemap Graph</button>
        </div>

        <div id="sitemapGraphTab" class="tab-content active">
            <div class="control-group" style="margin-top:10px; margin-bottom:10px;">
                 <button id="loadSitemapGraphBtn" class="button" ${!currentTargetId ? 'disabled' : ''}>Load Sitemap Graph</button>
            </div>
            <div id="sitemapGraphContainer" class="graph-container" style="display: none; position: relative;">
                <div id="sitemapCy" style="width: 100%; height: 600px; border: 1px solid #ccc;"></div>
                <div id="sitemapTooltip" class="graph-tooltip" style="display:none; position:absolute; z-index: 1000;"></div>
            </div>
        </div>

        <div id="pageSitemapGraphTab" class="tab-content">
            <div class="control-group" style="margin-top:10px; margin-bottom:10px;">
                <button id="loadPageSitemapGraphBtn" class="button" ${!currentTargetId ? 'disabled' : ''}>Load Page Sitemap Graph</button>
            </div>
            <div id="pageSitemapGraphContainer" class="graph-container" style="display: none; position: relative;">
                <div id="pageSitemapCy" style="width: 100%; height: 600px; border: 1px solid #ccc;"></div>
                <div id="pageSitemapTooltip" class="graph-tooltip" style="display:none; position:absolute; z-index: 1000;"></div>
            </div>
        </div>
    `;

    const loadSitemapBtn = document.getElementById('loadSitemapGraphBtn');
    const loadPageSitemapBtn = document.getElementById('loadPageSitemapGraphBtn');
    const zoomInBtn = document.getElementById('zoomInBtn');
    const zoomOutBtn = document.getElementById('zoomOutBtn');
    const resetZoomBtn = document.getElementById('resetZoomBtn');

    if (loadSitemapBtn) {
        loadSitemapBtn.addEventListener('click', () => loadSitemapGraph(currentTargetId));
    }
    if (loadPageSitemapBtn) {
        loadPageSitemapBtn.addEventListener('click', () => loadPageSitemapGraph(currentTargetId));
    }
    // Add listeners for zoom buttons
    if (zoomInBtn) {
        zoomInBtn.addEventListener('click', handleZoomIn);
    }
    if (zoomOutBtn) {
        zoomOutBtn.addEventListener('click', handleZoomOut);
    }
    if (resetZoomBtn) {
        resetZoomBtn.addEventListener('click', handleResetZoom);
    }

    if (!currentTargetId) {
        document.getElementById('visualizerMessage').innerHTML = '<p>Please select a target to view visualizations.</p>';
    }

    // Ensure Cytoscape.js is loaded
    if (typeof cytoscape === 'undefined') {
        console.error("Cytoscape.js is not loaded. Please include the library script.");
        document.getElementById('visualizerMessage').innerHTML = '<p class="error-message">Cytoscape.js library not found. Cannot render graphs.</p>';
        if (loadSitemapBtn) loadSitemapBtn.disabled = true;
        if (loadPageSitemapBtn) loadPageSitemapBtn.disabled = true;
        if (zoomInBtn) zoomInBtn.disabled = true;
        if (zoomOutBtn) zoomOutBtn.disabled = true;
        if (loadPageSitemapBtn) loadPageSitemapBtn.disabled = true;
    }

    // Tab switching logic
    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.addEventListener('click', () => switchTab(button.dataset.tab));
    });
}

async function loadSitemapGraph(targetId) {
    if (!targetId) return;
    const container = document.getElementById('sitemapGraphContainer');
    const cyContainer = document.getElementById('sitemapCy');
    const messageArea = document.getElementById('visualizerMessage');
    const tooltip = document.getElementById('sitemapTooltip');

    if (!container || !cyContainer || !messageArea || !tooltip) return;

    container.style.display = 'block';
    messageArea.textContent = 'Loading Sitemap graph data...';
    messageArea.className = 'message-area';

    // Destroy previous instance if it exists
    if (sitemapCy) {
        sitemapCy.destroy();
        sitemapCy = null;
    }

    try {
        const graphData = await apiService.getSitemapGraphData(targetId);
        messageArea.textContent = ''; // Clear message on success
        sitemapCy = renderGraph(cyContainer, graphData, tooltip, 'sitemap');
    } catch (error) {
        console.error("Error loading Sitemap graph:", error);
        messageArea.textContent = `Error loading Sitemap graph: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
        cyContainer.innerHTML = `<p class="error-message">Failed to load graph data.</p>`;
    }
}

async function loadPageSitemapGraph(targetId) {
    if (!targetId) return;
    const container = document.getElementById('pageSitemapGraphContainer');
    const cyContainer = document.getElementById('pageSitemapCy');
    const messageArea = document.getElementById('visualizerMessage');
    const tooltip = document.getElementById('pageSitemapTooltip');

    if (!container || !cyContainer || !messageArea || !tooltip) return;

    container.style.display = 'block';
    messageArea.textContent = 'Loading Page Sitemap graph data...';
    messageArea.className = 'message-area';

    // Destroy previous instance if it exists
    if (pageSitemapCy) {
        pageSitemapCy.destroy();
        pageSitemapCy = null;
    }

    try {
        const graphData = await apiService.getPageSitemapGraphData(targetId);
        messageArea.textContent = ''; // Clear message on success
        pageSitemapCy = renderGraph(cyContainer, graphData, tooltip, 'pageSitemap');
    } catch (error) {
        console.error("Error loading Page Sitemap graph:", error);
        messageArea.textContent = `Error loading Page Sitemap graph: ${escapeHtml(error.message)}`;
        messageArea.className = 'message-area error-message';
        cyContainer.innerHTML = `<p class="error-message">Failed to load graph data.</p>`;
    }
}

// --- Zoom Controls ---

function getActiveCyInstance() {
    const sitemapTab = document.getElementById('sitemapGraphTab');
    const pageSitemapTab = document.getElementById('pageSitemapGraphTab');

    if (sitemapTab && sitemapTab.classList.contains('active') && sitemapCy) {
        return sitemapCy;
    }
    if (pageSitemapTab && pageSitemapTab.classList.contains('active') && pageSitemapCy) {
        return pageSitemapCy;
    }
    return null; // No active graph or instance not initialized
}

function handleZoomIn() {
    const cy = getActiveCyInstance();
    if (cy) {
        cy.zoom(cy.zoom() * 1.2); // Zoom in by 20%
    }
}

function handleZoomOut() {
    const cy = getActiveCyInstance();
    if (cy) {
        cy.zoom(cy.zoom() / 1.2); // Zoom out by 20%
    }
}

function switchTab(tabIdToActivate) {
    // Deactivate all tabs and content
    document.querySelectorAll('.tabs .tab-button').forEach(button => {
        button.classList.remove('active');
    });
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });

    // Activate the selected tab and content
    document.querySelector(`.tabs .tab-button[data-tab="${tabIdToActivate}"]`).classList.add('active');
    document.getElementById(tabIdToActivate).classList.add('active');
}

function handleResetZoom() {
    const cy = getActiveCyInstance();
    if (cy) {
        // Fit the graph back into the container with padding
        cy.fit(cy.elements(), 30); // Use the same padding as the initial layout
    }
}

function renderGraph(containerElement, graphData, tooltipElement, graphType) {
    if (typeof cytoscape === 'undefined') {
        console.error("Cytoscape.js is not available.");
        containerElement.innerHTML = '<p class="error-message">Cytoscape.js library not found.</p>';
        return null;
    }

    // Transform data into Cytoscape.js format
    const elements = [
        ...graphData.nodes.map(node => ({
            data: {
                id: node.id, // Ensure ID is directly under data
                label: node.label,
                type: node.type, // Use type for styling
                ...node.data // Include any extra data
            }
        })),
        ...graphData.edges.map(edge => ({
            data: {
                id: `${edge.source}-${edge.target}-${edge.label || ''}-${Math.random().toString(36).substr(2, 5)}`, // Ensure unique edge IDs
                source: edge.source,
                target: edge.target,
                label: edge.label, // Use label for edge text
                ...edge.data
            }
        }))
    ];

    const cy = cytoscape({
        container: containerElement,
        elements: elements,
        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#666',
                    'label': 'data(label)',
                    'text-valign': 'center',
                    'color': 'white',
                    'text-outline-width': 2,
                    'text-outline-color': '#888',
                    'font-size': '10px',
                    'width': 'mapData(weight, 40, 80, 20, 60)', // Example: dynamically size nodes
                    'height': 'mapData(weight, 40, 80, 20, 60)'
                }
            },
            {
                selector: 'node[type="folder"]',
                style: {
                    'background-color': '#3498db',
                    'text-outline-color': '#2980b9',
                    'shape': 'round-rectangle'
                }
            },
            {
                selector: 'node[type="endpoint"]',
                style: {
                    'background-color': '#2ecc71',
                    'text-outline-color': '#27ae60',
                    'shape': 'ellipse'
                }
            },
            {
                selector: 'node[type="page"]',
                style: {
                    'background-color': '#9b59b6',
                    'text-outline-color': '#8e44ad',
                    'shape': 'diamond'
                }
            },
            {
                selector: 'node[type="url"]',
                style: {
                    'background-color': '#f1c40f',
                    'text-outline-color': '#f39c12',
                    'color': '#333', // Darker text for light background
                    'shape': 'ellipse'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#ccc',
                    'target-arrow-color': '#ccc',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'label': 'data(label)',
                    'font-size': '8px',
                    'text-rotation': 'autorotate',
                    'color': '#555',
                    'text-outline-width': 1,
                    'text-outline-color': '#fff'
                }
            },
            {
                selector: 'edge[method="GET"]',
                style: { 'line-color': '#2ecc71', 'target-arrow-color': '#2ecc71' }
            },
            {
                selector: 'edge[method="POST"]',
                style: { 'line-color': '#e74c3c', 'target-arrow-color': '#e74c3c' }
            },
            {
                selector: 'edge[method="PUT"]',
                style: { 'line-color': '#f39c12', 'target-arrow-color': '#f39c12' }
            },
            {
                selector: 'edge[method="DELETE"]',
                style: { 'line-color': '#c0392b', 'target-arrow-color': '#c0392b' }
            },
            {
                selector: 'edge[source^="page-"]', // Edges originating from a page node
                style: { 'line-color': '#9b59b6', 'target-arrow-color': '#9b59b6' }
            }
        ],
        layout: {
            name: 'cose',
            animate: true,
            animationDuration: 500,
            padding: 30,
            nodeRepulsion: function( node ){ return 400000; },
            idealEdgeLength: function( edge ){ return 100; },
            edgeElasticity: function( edge ){ return 100; },
            gravity: 80,
            numIter: 1000,
            initialTemp: 200,
            coolingFactor: 0.95,
            minTemp: 1.0
        }
    });

    // Tooltip functionality
    cy.on('mouseover', 'node', function(event) {
        const node = event.target;
        const data = node.data();
        let content = `<strong>${escapeHtml(data.label)}</strong><br>`;
        if (data.type === 'endpoint') {
            content += `Method: ${escapeHtml(data.method)}<br>`;
            content += `Path: ${escapeHtml(data.path)}<br>`;
            if (data.statusCode) content += `Status: ${data.statusCode}<br>`;
            if (data.responseSize) content += `Size: ${data.responseSize} bytes<br>`;
        } else if (data.type === 'folder') {
            content += `Path: ${escapeHtml(data.fullPath)}`;
        } else if (data.type === 'page') {
            content += `Description: ${escapeHtml(data.description || 'N/A')}`;
        } else if (data.type === 'url') {
            content += `Method: ${escapeHtml(data.method)}<br>`;
            content += `Path: ${escapeHtml(data.path)}`;
        }

        tooltipElement.innerHTML = content;
        // Position the tooltip relative to the graph container, not the viewport
        const containerRect = containerElement.getBoundingClientRect();
        tooltipElement.style.left = (event.renderedPosition.x + 10 + containerElement.scrollLeft) + 'px';
        tooltipElement.style.top = (event.renderedPosition.y + 10 + containerElement.scrollTop) + 'px';
        tooltipElement.style.display = 'block';
    });

    cy.on('mouseout', 'node', function() {
        tooltipElement.style.display = 'none';
    });

    cy.on('tap', 'node', function(evt){
        const node = evt.target;
        const data = node.data();
        let details = `<strong>ID:</strong> ${escapeHtml(data.id)}<br><strong>Label:</strong> ${escapeHtml(data.label)}<br><strong>Type:</strong> ${escapeHtml(data.type)}`;

        if (data.type === 'endpoint') {
            details += `<br><strong>Method:</strong> ${escapeHtml(data.method)}`;
            details += `<br><strong>Path:</strong> ${escapeHtml(data.path)}`;
            // Check if logId exists and is not 0 (Int64 default)
            if (data.logId && data.logId !== 0) {
                 details += `<br><strong>Log ID:</strong> <a href="#proxy-log-detail?id=${data.logId}" target="_blank">${data.logId}</a>`;
            }
            if (data.statusCode) details += `<br><strong>Status:</strong> ${data.statusCode}`;
            if (data.responseSize) details += `<br><strong>Size:</strong> ${data.responseSize} bytes`;
            if (data.isFavorite !== undefined) details += `<br><strong>Favorite:</strong> ${data.isFavorite ? 'Yes' : 'No'}`;
        } else if (data.type === 'page') {
            details += `<br><strong>Page ID:</strong> ${data.pageId}`;
            details += `<br><strong>Description:</strong> ${escapeHtml(data.description || 'N/A')}`;
            details += `<br><strong>Started:</strong> ${new Date(data.startTime).toLocaleString()}`;
            // Check if endTime exists and is valid (sql.NullTime)
            if (data.endTime && data.endTime.Valid) {
                 details += `<br><strong>Ended:</strong> ${new Date(data.endTime.Time).toLocaleString()}`;
            } else {
                 details += `<br><strong>Ended:</strong> Still Recording`;
            }
        } else if (data.type === 'url') {
            details += `<br><strong>Method:</strong> ${escapeHtml(data.method)}`;
            details += `<br><strong>Path:</strong> ${escapeHtml(data.path)}`;
            // Check if exampleLogId exists and is not 0
            if (data.exampleLogId && data.exampleLogId !== 0) {
                 details += `<br><strong>Example Log ID:</strong> <a href="#proxy-log-detail?id=${data.exampleLogId}" target="_blank">${data.exampleLogId}</a>`;
            }
        }

        uiService.showModalMessage(`Node Details: ${escapeHtml(data.label)}`, details);

        // Handle Ctrl+Click (or Cmd+Click on Mac) to open log details
        if (evt.originalEvent.ctrlKey || evt.originalEvent.metaKey) {
            let logIdToOpen = null;
            if (data.type === 'endpoint' && data.logId && data.logId !== 0) {
                logIdToOpen = data.logId;
            } else if (data.type === 'url' && data.exampleLogId && data.exampleLogId !== 0) {
                logIdToOpen = data.exampleLogId;
            }

            if (logIdToOpen !== null) {
                // Construct the hash URL for the log detail view
                const logDetailHash = `#proxy-log-detail?id=${logIdToOpen}`;
                window.open(logDetailHash, '_blank'); // Open in a new tab
            }
        }
    });

    return cy; // Return the Cytoscape instance
}