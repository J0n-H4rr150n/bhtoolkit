// static/apiService.js

let API_BASE = '/api'; // Default, can be overridden by initApiService

/**
 * Initializes the API service with a specific base URL.
 * @param {string} baseUrl - The base URL for all API calls.
 */
export function initApiService(baseUrl) {
    if (baseUrl) {
        API_BASE = baseUrl;
    }
    console.log("[ApiService] Initialized with API_BASE:", API_BASE);
}

/**
 * A helper function to handle fetch responses.
 * @param {Response} response - The fetch Response object.
 * @returns {Promise<Object>} - A promise that resolves with the JSON data or rejects with an error.
 */
async function handleResponse(response) {
    if (response.ok) {
        // Handle cases where response might be empty (e.g., 204 No Content)
        const contentType = response.headers.get("content-type");
        if (contentType && contentType.indexOf("application/json") !== -1) {
            const responseJson = await response.json();
            console.log("[ApiService] handleResponse (url, status, json):", response.url, response.status, JSON.parse(JSON.stringify(responseJson)));
            return responseJson;
        }
        return {}; // Return empty object for non-JSON or empty successful responses
    }
    // Try to parse error message from JSON response
    let errorMessage = `HTTP error ${response.status} ${response.statusText}`;
    try {
        const errorData = await response.json();
        errorMessage = errorData.message || errorMessage;
    } catch (e) {
        // If parsing JSON fails, use the original error message
    }
    throw new Error(errorMessage);
}

/**
 * Fetches the current target settings.
 * @returns {Promise<Object>}
 */
export async function getCurrentTargetSetting() {
    const response = await fetch(`${API_BASE}/settings/current-target`);
    return handleResponse(response);
}

/**
 * Sets or clears the current target.
 * @param {number|null} targetId - The ID of the target to set, or null to clear.
 * @returns {Promise<Object>}
 */
export async function setCurrentTargetSetting(targetId) {
    const response = await fetch(`${API_BASE}/settings/current-target`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target_id: targetId })
    });
    return handleResponse(response);
}

/**
 * Fetches table column width settings.
 * @returns {Promise<Object>}
 */
export async function getTableLayouts() {
    const response = await fetch(`${API_BASE}/settings/table-column-widths`);
    return handleResponse(response);
}

/**
 * Saves table column width settings.
 * @param {Object} layouts - The layout data to save.
 * @returns {Promise<Object>}
 */
export async function saveTableLayouts(layouts) {
    const response = await fetch(`${API_BASE}/settings/table-column-widths`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(layouts)
    });
    return handleResponse(response);
}

/**
 * Fetches UI settings.
 * @returns {Promise<Object>}
 */
export async function getUISettings() {
    const response = await fetch(`${API_BASE}/ui-settings`);
    return handleResponse(response);
}

/**
 * Saves UI settings.
 * @param {Object} settings - The UI settings to save (e.g., { showSynackSection: true }).
 * @returns {Promise<Object>}
 */
export async function saveUISettings(settings) {
    const response = await fetch(`${API_BASE}/ui-settings`, {
        method: 'PUT', // Or POST, depending on your backend implementation
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
    });
    return handleResponse(response);
}


/**
 * Fetches all platforms.
 * @returns {Promise<Array<Object>>}
 */
export async function getPlatforms() {
    const response = await fetch(`${API_BASE}/platforms`);
    return handleResponse(response);
}

/**
 * Adds a new platform.
 * @param {string} name - The name of the new platform.
 * @returns {Promise<Object>}
 */
export async function addPlatform(name) {
    const response = await fetch(`${API_BASE}/platforms`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name })
    });
    return handleResponse(response);
}

/**
 * Fetches details for a specific platform.
 * @param {number|string} platformId - The ID of the platform.
 * @returns {Promise<Object>}
 */
export async function getPlatformDetails(platformId) {
    const response = await fetch(`${API_BASE}/platforms/${platformId}`);
    return handleResponse(response);
}

/**
 * Updates an existing platform.
 * @param {number|string} platformId - The ID of the platform to update.
 * @param {Object} data - The data to update (e.g., { name: newName }).
 * @returns {Promise<Object>}
 */
export async function updatePlatform(platformId, data) {
    const response = await fetch(`${API_BASE}/platforms/${platformId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    return handleResponse(response);
}

/**
 * Deletes a platform.
 * @param {number|string} platformId - The ID of the platform to delete.
 * @returns {Promise<Object>}
 */
export async function deletePlatform(platformId) {
    const response = await fetch(`${API_BASE}/platforms/${platformId}`, {
        method: 'DELETE'
    });
    return handleResponse(response);
}

/**
 * Fetches targets, optionally filtered by platform ID.
 * @param {number|string|null} platformIdFilter - Optional platform ID to filter by.
 * @returns {Promise<Array<Object>>}
 */
export async function getTargets(platformIdFilter = null) {
    let url = `${API_BASE}/targets`;
    if (platformIdFilter) {
        url += `?platform_id=${platformIdFilter}`;
    }
    const response = await fetch(url);
    return handleResponse(response);
}

/**
 * Adds a new target.
 * @param {Object} targetData - Data for the new target (platform_id, codename, slug, link, notes).
 * @returns {Promise<Object>}
 */
export async function addTarget(targetData) {
    const response = await fetch(`${API_BASE}/targets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(targetData)
    });
    return handleResponse(response);
}

/**
 * Fetches details for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>}
 */
export async function getTargetDetails(targetId) {
    const response = await fetch(`${API_BASE}/target/${targetId}`);
    return handleResponse(response);
}

/**
 * Updates an existing target.
 * @param {number|string} targetId - The ID of the target to update.
 * @param {Object} data - The data to update (e.g., { link, notes }).
 * @returns {Promise<Object>}
 */
export async function updateTarget(targetId, data) {
    const response = await fetch(`${API_BASE}/target/${targetId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    return handleResponse(response);
}

/**
 * Adds a scope rule.
 * @param {Object} ruleData - Data for the scope rule.
 * @returns {Promise<Object>}
 */
export async function addScopeRule(ruleData) {
    const response = await fetch(`${API_BASE}/scope-rules`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(ruleData)
    });
    return handleResponse(response);
}

/**
 * Deletes a scope rule.
 * @param {number|string} ruleId - The ID of the scope rule to delete.
 * @returns {Promise<Object>}
 */
export async function deleteScopeRule(ruleId) {
    const response = await fetch(`${API_BASE}/scope-rules/${ruleId}`, {
        method: 'DELETE'
    });
    return handleResponse(response);
}

/**
 * Fetches checklist items for a target with pagination, sorting, and filtering.
 * @param {number|string} targetId - The ID of the target.
 * @param {Object} params - Query parameters (page, limit, sort_by, sort_order, filter).
 * @returns {Promise<Object>} - API response containing checklist items and pagination info.
 */
export async function getChecklistItems(targetId, params = {}) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/target/${targetId}/checklist-items?${query}`);
    return handleResponse(response);
}

/**
 * Adds a new checklist item.
 * @param {Object} itemData - Data for the new checklist item.
 * @returns {Promise<Object>}
 */
export async function addChecklistItem(itemData) {
    const response = await fetch(`${API_BASE}/checklist-items`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(itemData)
    });
    return handleResponse(response);
}

/**
 * Updates an existing checklist item.
 * @param {number|string} itemId - The ID of the checklist item to update.
 * @param {Object} data - The data to update.
 * @returns {Promise<Object>}
 */
export async function updateChecklistItem(itemId, data) {
    const response = await fetch(`${API_BASE}/checklist-items/${itemId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    return handleResponse(response);
}

/**
 * Deletes a checklist item.
 * @param {number|string} itemId - The ID of the checklist item to delete.
 * @returns {Promise<Object>}
 */
export async function deleteChecklistItem(itemId) {
    const response = await fetch(`${API_BASE}/checklist-items/${itemId}`, {
        method: 'DELETE'
    });
    return handleResponse(response);
}

/**
 * Deletes all checklist items for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>}
 */
export async function deleteAllChecklistItemsForTarget(targetId) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/checklist-items/all`, { // New endpoint
        method: 'DELETE'
    });
    return handleResponse(response);
}

/**
 * Fetches Synack targets.
 * @param {Object} params - Query parameters (page, limit, sort_by, sort_order, active_only).
 * @returns {Promise<Object>} - API response containing targets and pagination info.
 */
export async function getSynackTargets(params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/synack-targets?${query}`);
    return handleResponse(response);
}

/**
 * Promotes a Synack target to a regular target.
 * @param {Object} payload - Data for promotion (synack_target_id_str, platform_id, etc.).
 * @returns {Promise<Object>}
 */
export async function promoteSynackTarget(payload) {
    const response = await fetch(`${API_BASE}/targets/from-synack`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    return handleResponse(response);
}

/**
 * Fetches details for a specific Synack target (for breadcrumbs/titles).
 * @param {number|string} targetDbId - The database ID of the Synack target.
 * @returns {Promise<Object>}
 */
export async function getSynackTargetDetails(targetDbId) {
    const response = await fetch(`${API_BASE}/synack-targets/${targetDbId}`);
    return handleResponse(response);
}

/**
 * Fetches global Synack analytics.
 * @param {Object} params - Query parameters (page, limit, sort_by, sort_order).
 * @returns {Promise<Object>}
 */
export async function getGlobalSynackAnalytics(params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/synack-analytics/all?${query}`);
    return handleResponse(response);
}

/**
 * Fetches analytics (findings) for a specific Synack target.
 * @param {number|string} targetDbId - The database ID of the Synack target.
 * @param {Object} params - Query parameters (page, limit, sort_by, sort_order).
 * @returns {Promise<Object>}
 */
export async function getTargetSynackAnalytics(targetDbId, params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/synack-targets/${targetDbId}/analytics?${query}`);
    return handleResponse(response);
}

/**
 * Requests a refresh of findings for a Synack target.
 * @param {number|string} targetDbId - The database ID of the Synack target.
 * @returns {Promise<Object>}
 */
export async function refreshSynackFindings(targetDbId) {
    const response = await fetch(`${API_BASE}/synack-targets/${targetDbId}/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
    });
    return handleResponse(response);
}

/**
 * Fetches proxy log entries.
 * @param {Object} params - Query parameters (target_id, page, limit, sort_by, sort_order, filters...).
 * @returns {Promise<Object>}
 */
export async function getProxyLog(params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/traffic-log?${query}`);
    return handleResponse(response);
}

/**
 * Marks a proxy log entry as favorite or not.
 * @param {number|string} logId - The ID of the log entry.
 * @param {boolean} isFavorite - True to mark as favorite, false otherwise.
 * @returns {Promise<Object>}
 */
export async function setProxyLogFavorite(logId, isFavorite) {
    const response = await fetch(`${API_BASE}/traffic-log/entry/${logId}/favorite`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ is_favorite: isFavorite })
    });
    return handleResponse(response);
}

/**
 * Fetches details for a specific proxy log entry.
 * @param {number|string} logId - The ID of the log entry.
 * @param {Object} navParams - Optional parameters for fetching prev/next log IDs based on current filters.
 * @returns {Promise<Object>}
 */
export async function getProxyLogDetail(logId, navParams = {}) {
    const query = new URLSearchParams(navParams).toString();
    const response = await fetch(`${API_BASE}/traffic-log/entry/${logId}?${query}`);
    return handleResponse(response);
}

/**
 * Saves notes for a proxy log entry.
 * @param {number|string} logId - The ID of the log entry.
 * @param {string} notes - The notes to save.
 * @returns {Promise<Object>}
 */
export async function saveProxyLogNotes(logId, notes) {
    const response = await fetch(`${API_BASE}/traffic-log/entry/${logId}/notes`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ notes })
    });
    return handleResponse(response);
}

/**
 * Deletes all proxy log entries for a specific target.
 * @param {number|string} targetId - The ID of the target whose logs are to be deleted.
 * @returns {Promise<Object>}
 */
export async function deleteProxyLogsForTarget(targetId) {
    const response = await fetch(`${API_BASE}/traffic-log/target/${targetId}`, {
        method: 'DELETE'
    });
    return handleResponse(response);
}

/**
 * Analyzes JavaScript links from a proxy log entry.
 * @param {number} httpLogId - The ID of the HTTP log entry.
 * @returns {Promise<Object>}
 */
export async function analyzeJsLinks(httpLogId) {
    const response = await fetch(`${API_BASE}/analyze/jslinks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ http_log_id: httpLogId })
    });
    return handleResponse(response);
}

/**
 * Fetches all checklist templates.
 * @returns {Promise<Array<Object>>}
 */
export async function getChecklistTemplates() {
    const response = await fetch(`${API_BASE}/checklist-templates`);
    return handleResponse(response);
}

/**
 * Fetches items for a specific checklist template.
 * @param {number|string} templateId - The ID of the checklist template.
 * @param {Object} params - Query parameters (page, limit).
 * @returns {Promise<Object>}
 */
export async function getChecklistTemplateItems(templateId, params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/checklist-templates/${templateId}/items?${query}`);
    return handleResponse(response);
}

/**
 * Copies selected checklist template items to a target.
 * @param {Object} payload - Data for copying (target_id, items).
 * @returns {Promise<Object>}
 */
export async function copyChecklistTemplateItemsToTarget(payload) {
    const response = await fetch(`${API_BASE}/checklist-templates/copy-to-target`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    return handleResponse(response);
}

/**
 * Copies all items from a checklist template to a target's checklist.
 * @param {number|string} templateId - The ID of the checklist template.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - A promise that resolves with the backend's response (e.g., a success message and count of items copied).
 */
export async function copyAllTemplateItemsToTarget(templateId, targetId) {
    const response = await fetch(`${API_BASE}/checklist-templates/copy-all-to-target`, { // New endpoint
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ template_id: templateId, target_id: targetId })
    });
    return handleResponse(response);
}


/**
 * Adds a manual entry to the sitemap.
 * @param {string|number} httpLogId - The ID of the original HTTP log entry.
 * @param {string} folderPath - The user-defined folder path for the sitemap.
 * @param {string} notes - Optional notes for the sitemap entry.
 * @returns {Promise<Object>}
 */
export async function addSitemapManualEntry(httpLogId, folderPath, notes) {
    const response = await fetch(`${API_BASE}/sitemap/manual-entry`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ http_log_id: parseInt(httpLogId, 10), folder_path: folderPath, notes })
    });
    return handleResponse(response);
}

/**
 * Fetches manual sitemap entries for a given target.
 * @param {string|number} targetId - The ID of the target.
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of sitemap entries.
 */
export async function getSitemapManualEntries(targetId) {
    const response = await fetch(`${API_BASE}/sitemap/manual-entries?target_id=${targetId}`);
    return handleResponse(response);
}

/**
 * Fetches the generated sitemap tree for a given target.
 * @param {string|number} targetId - The ID of the target.
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of sitemap tree nodes.
 */
export async function getGeneratedSitemap(targetId) {
    const response = await fetch(`${API_BASE}/sitemap/generated?target_id=${targetId}`);
    return handleResponse(response);
}


export async function getProxyExclusionRules() {
    const response = await fetch(`${API_BASE}/settings/proxy-exclusions`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    return handleResponse(response);
}

export async function setProxyExclusionRules(rules) {
    const response = await fetch(`${API_BASE}/settings/proxy-exclusions`, {
        method: 'POST', // Or PUT, backend supports both
        headers: {
            'Content-Type': 'application/json',
            // Add any auth headers if needed
        },
        body: JSON.stringify(rules),
    });
    return handleResponse(response);
}

// --- Findings API ---

/**
 * Fetches all findings for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of findings.
 */
export async function getTargetFindings(targetId) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/findings`);
    return handleResponse(response);
}

/**
 * Fetches details for a specific finding.
 * @param {number|string} findingId - The ID of the finding.
 * @returns {Promise<Object>} - A promise that resolves with the finding details.
 */
export async function getFindingDetails(findingId) {
    const response = await fetch(`${API_BASE}/findings/${findingId}`);
    return handleResponse(response);
}

/**
 * Updates an existing finding.
 * @param {number|string} findingId - The ID of the finding to update.
 * @param {Object} findingData - The data to update the finding with.
 * @returns {Promise<Object>} - A promise that resolves with the updated finding object.
 */
export async function updateTargetFinding(findingId, findingData) {
    const response = await fetch(`${API_BASE}/findings/${findingId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(findingData),
    });
    return handleResponse(response);
}

/**
 * Deletes a finding.
 * @param {number|string} findingId - The ID of the finding to delete.
 * @returns {Promise<Object>} - A promise that resolves (usually with an empty object on success).
 */
export async function deleteTargetFinding(findingId) {
    const response = await fetch(`${API_BASE}/findings/${findingId}`, {
        method: 'DELETE',
    });
    return handleResponse(response); // Will be an empty object on 204 No Content
}

export async function createTargetFinding(findingData) {
    const response = await fetch(`${API_BASE}/findings`, { // Assuming /api/findings is your endpoint
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(findingData),
    });
    if (!response.ok) {
        const errorData = await response.json().catch(() => ({ message: response.statusText }));
        throw new Error(errorData.message || `HTTP error! status: ${response.status}`);
    }
    return await response.json(); // Or handle no content if API returns 201/204
}

/**
 * Triggers the backend analysis of parameterized URLs for a target.
 * @param {number|string} targetId - The ID of the target to analyze.
 * @returns {Promise<Object>} - A promise that resolves with the analysis summary.
 */
export async function analyzeTargetParameters(targetId) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/analyze-parameters`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
    });
    return handleResponse(response);
}

/**
 * Fetches stored parameterized URLs for a target with pagination, sorting, and filtering.
 * @param {Object} params - Query parameters (target_id, page, limit, sort_by, sort_order, filters...).
 * @returns {Promise<Object>} - A promise that resolves with the paginated list of parameterized URLs.
 */
export async function getParameterizedURLs(params) {
    const query = new URLSearchParams(params);
    // Ensure target_id is part of the query string
    if (!params.target_id) throw new Error("target_id is required for getParameterizedURLs");

    const response = await fetch(`${API_BASE}/parameterized-urls?${query.toString()}`);
    return handleResponse(response);
}

/**
 * Adds a new task to the Modifier.
 * @param {Object} taskData - Data for the new modifier task (e.g., { parameterized_url_id }).
 * @returns {Promise<Object>} - A promise that resolves with the created task details.
 */
export async function addModifierTask(taskData) {
    const response = await fetch(`${API_BASE}/modifier/tasks`, { // Endpoint to be created
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(taskData),
    });
    return handleResponse(response);
}

/**
 * Fetches all tasks currently in the Modifier.
 * @param {Object} params - Optional query parameters (e.g., for pagination, filtering by target_id).
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of modifier tasks.
 */
export async function getModifierTasks(params = {}) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/modifier/tasks?${query}`);
    return handleResponse(response);
}

/**
 * Fetches the full details for a specific modifier task.
 * @param {number|string} taskId - The ID of the modifier task.
 * @returns {Promise<Object>} - A promise that resolves with the task details.
 */
export async function getModifierTaskDetails(taskId) {
    const response = await fetch(`${API_BASE}/modifier/tasks/${taskId}`);
    return handleResponse(response);
}

/**
 * Executes a modified request via the backend.
 * @param {Object} requestData - The modified request details.
 *                               Expected: { taskId (optional), method, url, headers (string), body (string) }
 * @returns {Promise<Object>} - A promise that resolves with the response from the executed request.
 */
export async function executeModifiedRequest(requestData) {
    const response = await fetch(`${API_BASE}/modifier/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestData),
    });
    return handleResponse(response);
}

/**
 * Updates a modifier task (e.g., its name).
 * @param {number|string} taskId - The ID of the task to update.
 * @param {Object} updateData - An object containing the data to update (e.g., { name: "New Name" }).
 * @returns {Promise<Object>} - A promise that resolves with the updated task details.
 */
export async function updateModifierTask(taskId, updateData) {
    const response = await fetch(`${API_BASE}/modifier/tasks/${taskId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updateData),
    });
    return handleResponse(response);
}

/**
 * Clones an existing modifier task.
 * @param {number|string} originalTaskId - The ID of the task to clone.
 * @returns {Promise<Object>} - A promise that resolves with the newly created (cloned) task details.
 */
export async function cloneModifierTask(originalTaskId) {
    const response = await fetch(`${API_BASE}/modifier/tasks/${originalTaskId}/clone`, {
        method: 'POST', // Using POST as it creates a new resource
    });
    return handleResponse(response);
}

/**
 * Updates the display order of modifier tasks.
 * @param {Object} taskOrders - A map where keys are task IDs (string) and values are their new display order (number).
 *                              Example: { "1": 0, "5": 1, "2": 2 }
 * @returns {Promise<Object>} - A promise that resolves with the backend's response message.
 */
export async function updateModifierTasksOrder(taskOrders) {
    const response = await fetch(`${API_BASE}/modifier/tasks/order`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(taskOrders),
    });
    return handleResponse(response);
}

// --- Page Sitemap API ---

/**
 * Starts recording a new page.
 * @param {Object} pageData - Data for the new page (e.g., { target_id, name, description }).
 * @returns {Promise<Object>} - A promise that resolves with the created page details.
 */
export async function createPageSitemapEntry(pageData) {
    const response = await fetch(`${API_BASE}/pages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(pageData),
    });
    return handleResponse(response);
}

/**
 * Stops recording for a specific page and associates logs.
 * @param {Object} stopData - Data to stop recording (e.g., { page_id, target_id, start_timestamp }).
 * @returns {Promise<Object>} - A promise that resolves with the backend's response message.
 */
export async function stopPageSitemapRecording(stopData) {
    const response = await fetch(`${API_BASE}/pages/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(stopData),
    });
    return handleResponse(response);
}

/**
 * Fetches all recorded pages for a given target.
 * @param {string|number} targetId - The ID of the target.
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of page objects.
 */
export async function getPagesForTarget(targetId) {
    const response = await fetch(`${API_BASE}/pages?target_id=${targetId}`);
    return handleResponse(response);
}

/**
 * Fetches all HTTP logs associated with a specific recorded page.
 * @param {string|number} pageId - The ID of the page.
 * @param {Object} params - Optional query parameters (e.g., page, limit, sort_by, sort_order).
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of HTTP log objects.
 */
export async function getLogsForPageSitemapEntry(pageId, params = {}) {
    const queryParams = new URLSearchParams({ page_id: pageId, ...params }).toString();
    const response = await fetch(`${API_BASE}/pages/logs?${queryParams}`);
    return handleResponse(response);
}

/**
 * Updates the display order of recorded pages.
 * @param {Object} pageOrders - A map where keys are page IDs (string) and values are their new display order (number).
 * @returns {Promise<Object>} - A promise that resolves with the backend's response message.
 */
export async function updatePageSitemapOrder(pageOrders) {
    const response = await fetch(`${API_BASE}/pages/order`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(pageOrders),
    });
    return handleResponse(response);
}

/**
 * Deletes a specific recorded page sitemap entry.
 * @param {string|number} pageId - The ID of the page sitemap entry to delete.
 * @returns {Promise<Object>} - A promise that resolves (usually with an empty object or success message on success).
 */
export async function deletePageSitemapEntry(pageId) {
    const response = await fetch(`${API_BASE}/pages/${pageId}`, { // Note the URL structure
        method: 'DELETE',
    });
    return handleResponse(response); // Expects 200 OK or 204 No Content on success
}

/**
 * Updates details of a specific recorded page sitemap entry (e.g., name, description).
 * @param {string|number} pageId - The ID of the page sitemap entry to update.
 * @param {Object} updateData - An object containing the data to update (e.g., { name: "New Name" }).
 * @returns {Promise<Object>} - A promise that resolves with the updated page details.
 */
export async function updatePageSitemapEntryDetails(pageId, updateData) {
    const response = await fetch(`${API_BASE}/pages/${pageId}`, { // Assuming PUT to /api/pages/{id}
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updateData),
    });
    return handleResponse(response);
}

/**
 * Deletes a specific modifier task.
 * @param {string|number} taskId - The ID of the modifier task to delete.
 * @returns {Promise<Object>} - A promise that resolves (usually with an empty object or success message on success).
 */
export async function deleteModifierTask(taskId) {
    const response = await fetch(`${API_BASE}/modifier/tasks/${taskId}`, { // URL for deleting a specific task
        method: 'DELETE',
    });
    return handleResponse(response); // Expects 200 OK or 204 No Content on success
}

/**
 * Deletes all modifier tasks for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - A promise that resolves with the backend's response.
 */
export async function deleteModifierTasksForTarget(targetId) {
    const response = await fetch(`${API_BASE}/modifier/tasks/target/${targetId}`, {
        method: 'DELETE',
    });
    return handleResponse(response);
}

/**
 * Deletes all modifier tasks for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - A promise that resolves with the backend's response.
 */
export async function deleteAllModifierTasksForTarget(targetId) {
    const response = await fetch(`${API_BASE}/modifier/tasks/target/${targetId}`, {
        method: 'DELETE',
    });
    return handleResponse(response);
}

/**
 * Finds comments in a proxy log entry's response body.
 * @param {number} httpLogId - The ID of the HTTP log entry.
 * @returns {Promise<Array<Object>>} - A promise that resolves with an array of comment findings.
 */
export async function findComments(httpLogId) {
	const response = await fetch(`${API_BASE}/traffic-log/analyze/comments`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ http_log_id: httpLogId })
	});
	return handleResponse(response);
}

// --- Visualizer API ---

/**
 * Fetches sitemap graph data for a given target.
 * @param {string|number} targetId - The ID of the target.
 * @returns {Promise<Object>} - A promise that resolves with the graph data (nodes and edges).
 */
export async function getSitemapGraphData(targetId) {
    const response = await fetch(`${API_BASE}/visualizer/sitemap-graph?target_id=${targetId}`);
    return handleResponse(response);
}

/**
 * Fetches page sitemap graph data for a given target.
 * @param {string|number} targetId - The ID of the target.
 * @returns {Promise<Object>} - A promise that resolves with the graph data (nodes and edges).
 */
export async function getPageSitemapGraphData(targetId) {
    const response = await fetch(`${API_BASE}/visualizer/page-sitemap-graph?target_id=${targetId}`);
    return handleResponse(response);
}

/**
 * Sends a list of URLs to the backend to be requested through the proxy.
 * @param {Object} payload - Data containing target_id and an array of URLs.
 *                         Expected: { target_id: number, urls: string[] }
 * @returns {Promise<Object>} - A promise that resolves with the backend's response.
 */
export async function sendPathsToProxy(payload) {
    const response = await fetch(`${API_BASE}/proxy/send-requests`, { // New endpoint
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    return handleResponse(response);
}

/**
 * Fetches the application version from the backend.
 * @returns {Promise<Object>} - A promise that resolves with an object like { version: "1.0.0" }.
 */
export async function getVersion() {
    const response = await fetch(`${API_BASE}/version`);
    return handleResponse(response);
}

/**
 * Fetches domains for a specific target with pagination, sorting, and filtering.
 * @param {number|string} targetId - The ID of the target.
 * @param {Object} params - Query parameters (page, limit, sort_by, sort_order, domain_name_search, source_search, is_in_scope).
 * @returns {Promise<Object>} - API response containing domains and pagination info.
 */
export async function getDomains(targetId, params = {}) {
    if (!targetId) {
        return Promise.reject(new Error("Target ID is required to fetch domains."));
    }
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${API_BASE}/targets/${targetId}/domains?${query}`);
    return handleResponse(response);
}

/**
 * Creates a new domain.
 * @param {Object} domainData - Data for the new domain.
 *                              Expected: { target_id, domain_name, source (optional), is_in_scope (optional), notes (optional) }
 * @returns {Promise<Object>} - API response containing the created domain.
 */
export async function createDomain(domainData) {
    if (!domainData || !domainData.target_id || !domainData.domain_name) {
        return Promise.reject(new Error("Target ID and Domain Name are required to create a domain."));
    }
    const response = await fetch(`${API_BASE}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(domainData)
    });
    return handleResponse(response);
}

/**
 * Deletes a domain.
 * @param {number|string} domainId - The ID of the domain to delete.
 * @returns {Promise<Object>} - API response (usually empty on success).
 */
export async function deleteDomain(domainId) {
    const response = await fetch(`${API_BASE}/domains/${domainId}`, {
        method: 'DELETE'
    });
    return handleResponse(response); // Expects 204 No Content on success
}

/**
 * Updates an existing domain.
 * @param {number|string} domainId - The ID of the domain to update.
 * @param {Object} domainData - Data to update for the domain.
 *                              Expected: { source (optional), is_in_scope (optional), notes (optional) }
 * @returns {Promise<Object>} - API response containing the updated domain.
 */
export async function updateDomain(domainId, domainData) {
    const response = await fetch(`${API_BASE}/domains/${domainId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(domainData)
    });
    return handleResponse(response);
}

/**
 * Initiates subdomain discovery for a target.
 * @param {number|string} targetId - The ID of the target.
 * @param {Object} discoveryOptions - Options for the discovery process.
 *                                    Expected: { domain, recursive (optional), sources (optional) }
 * @returns {Promise<Object>} - API response indicating initiation.
 */
export async function discoverSubdomains(targetId, discoveryOptions) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/domains/discover`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(discoveryOptions)
    });
    return handleResponse(response); // Expects 202 Accepted on success
}

/**
 * Fetches the current status of subfinder tasks for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - API response containing subfinder status.
 */
export async function getSubfinderStatus(targetId) {
    if (!targetId) {
        return Promise.reject(new Error("Target ID is required to fetch subfinder status."));
    }
    const response = await fetch(`${API_BASE}/subfinder/status?target_id=${targetId}`);
    return handleResponse(response);
}

/**
 * Imports in-scope domains for a target from its scope rules.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - API response containing import summary (e.g., imported_count, skipped_count).
 */
export async function importInScopeDomains(targetId) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/domains/import-scope`, {
        method: 'POST', // Using POST as it creates/updates domain entries
        headers: { 'Content-Type': 'application/json' },
    });
    return handleResponse(response);
}

/**
 * Deletes all domains for a specific target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Object>} - API response (e.g., { deleted_count: X }).
 */
export async function deleteAllDomainsForTarget(targetId) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/domains/all`, { // New endpoint
        method: 'DELETE',
    });
    return handleResponse(response); // Expects 200 OK with a count or 204 No Content
}

/**
 * Sets the favorite status for a domain.
 * @param {number|string} domainId - The ID of the domain.
 * @param {boolean} isFavorite - True to mark as favorite, false otherwise.
 * @returns {Promise<Object>} - API response.
 */
export async function setDomainFavorite(domainId, isFavorite) {
    const response = await fetch(`${API_BASE}/domains/${domainId}/favorite`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ is_favorite: isFavorite })
    });
    return handleResponse(response);
}

/**
 * Marks all domains matching the given filters as favorite for a target.
 * @param {number|string} targetId - The ID of the target.
 * @param {Object} filters - An object containing filter criteria.
 *                           Expected: { domain_name_search, source_search, is_in_scope }
 * @returns {Promise<Object>} - API response, e.g., { updated_count: X }.
 */
export async function favoriteAllFilteredDomains(targetId, filters) {
    const response = await fetch(`${API_BASE}/targets/${targetId}/domains/favorite-filtered`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(filters)
    });
    return handleResponse(response);
}
