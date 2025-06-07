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
            return response.json();
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
 * Fetches checklist items for a target.
 * @param {number|string} targetId - The ID of the target.
 * @returns {Promise<Array<Object>>}
 */
export async function getChecklistItems(targetId) {
    const response = await fetch(`${API_BASE}/target/${targetId}/checklist-items`);
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
