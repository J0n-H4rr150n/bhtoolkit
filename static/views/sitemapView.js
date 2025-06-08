import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService; // If tables are used

// DOM element references
let viewContentContainer;

/**
 * Initializes the Sitemap View module with necessary services.
 * @param {Object} services - An object containing service instances.
 */
export function initSitemapView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService; // Assign if needed
    console.log("[SitemapView] Initialized.");
}

/**
 * Loads the sitemap view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
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
    viewContentContainer.innerHTML = headerHTML;

    if (!currentTargetId) {
        viewContentContainer.innerHTML += `<p>Please select a target to view its sitemap.</p>`;
        return;
    }

    viewContentContainer.innerHTML += `<div id="sitemapEntriesContainer"><p>Loading sitemap entries...</p></div>`;
    const entriesContainer = document.getElementById('sitemapEntriesContainer');

    try {
        const entries = await apiService.getSitemapManualEntries(currentTargetId);
        if (entries && entries.length > 0) {
            let entriesHTML = `
                <h3>Manually Added Entries</h3>
                <table>
                    <thead>
                        <tr>
                            <th>Folder Path</th>
                            <th>Method</th>
                            <th>Request Path</th>
                            <th>Notes</th>
                            <th>Added</th>
                        </tr>
                    </thead>
                    <tbody>`;
            entries.forEach(entry => {
                entriesHTML += `
                    <tr>
                        <td>${escapeHtml(entry.folder_path)}</td>
                        <td>${escapeHtml(entry.request_method)}</td>
                        <td>${escapeHtml(entry.request_path)}</td>
                        <td>${escapeHtml(entry.notes.String || '-')}</td>
                        <td>${new Date(entry.created_at).toLocaleDateString()}</td>
                    </tr>`;
            });
            entriesHTML += `</tbody></table>`;
            entriesContainer.innerHTML = entriesHTML;
        } else {
            entriesContainer.innerHTML = '<p>No manual sitemap entries found for this target.</p>';
        }
    } catch (error) {
        console.error("Error fetching sitemap entries:", error);
        entriesContainer.innerHTML = `<p class="error-message">Error loading sitemap entries: ${escapeHtml(error.message)}</p>`;
    }
}