// static/stateService.js

let state = {
    currentTargetId: null,
    currentTargetName: 'None',
    currentChecklistTemplateId: null,
    jsAnalysisDataCache: {},
    jsAnalysisSortState: { sortBy: 'category', sortOrder: 'ASC' },
    globalTableLayouts: {},
    viewConfig: {
        synackTargets: { sortBy: 'last_seen_timestamp', sortOrder: 'DESC', filterIsActive: true, sortableColumns: ['synack_target_id_str', 'codename', 'name', 'findings_count', 'status', 'last_seen_timestamp']},
        proxyLog: { sortBy: 'timestamp', sortOrder: 'DESC', filterFavoritesOnly: false, sortableColumns: ['id', 'timestamp', 'method', 'url', 'status', 'type', 'size', 'duration'] },
        synackAnalyticsGlobal: { sortBy: 'target_codename', sortOrder: 'ASC', sortableColumns: ['target_codename', 'category_name', 'count'] },
        synackAnalyticsTarget: { sortBy: 'reported_at', sortOrder: 'DESC', sortableColumns: ['synack_finding_id', 'title', 'category_name', 'severity', 'status', 'reported_at', 'vulnerability_url'] },
    },
    paginationState: {
        synackTargets: { currentPage: 1, limit: 20, totalPages: 1, totalRecords: 0, sortBy: 'last_seen_timestamp', sortOrder: 'DESC' },
        proxyLog: {
            currentPage: 1, limit: 15, totalPages: 1, totalRecords: 0,
            sortBy: 'timestamp', sortOrder: 'DESC', filterFavoritesOnly: false,
            filterMethod: '', filterStatus: '', filterContentType: '', filterSearchText: ''
        },
        checklistTemplateItems: {
            currentPage: 1,
            limit: 20,
            totalPages: 1, totalRecords: 0
        },
        proxyLogTableLayout: {
            index: { default: '3%', id: 'col-proxylog-index' },
            timestamp: { default: '15%', id: 'col-proxylog-timestamp' },
            method: { default: '7%', id: 'col-proxylog-method' },
            url: { default: 'auto', id: 'col-proxylog-url' },
            status: { default: '7%', id: 'col-proxylog-status' },
            type: { default: '15%', id: 'col-proxylog-type' },
            size: { default: '7%', id: 'col-proxylog-size' },
            actions: { default: '8%', id: 'col-proxylog-actions' }
        },
        parameterizedUrlsView: { // New state for this view
            currentPage: 1,
            limit: 50, // Default limit
            totalPages: 1,
            totalRecords: 0,
            sortBy: 'discovered_at', // Default sort
            sortOrder: 'DESC',
            filterRequestMethod: '',
            filterPathSearch: '',
            filterParamKeysSearch: '',
        },
        parameterizedUrlsTableLayout: { // Default column widths/visibility
            id: { default: '50px', visible: true, id: 'col-paramurl-id' },
            method: { default: '80px', visible: true, id: 'col-paramurl-method' },
            path: { default: '3fr', visible: true, id: 'col-paramurl-path' },
            param_keys: { default: '2fr', visible: true, id: 'col-paramurl-paramkeys' },
            example_url: { default: '3fr', visible: true, id: 'col-paramurl-exampleurl' },
            discovered: { default: '150px', visible: true, id: 'col-paramurl-discovered' },
            last_seen: { default: '150px', visible: true, id: 'col-paramurl-lastseen' },
            actions: { default: '100px', visible: true, id: 'col-paramurl-actions' },
        },
        synackAnalytics: { currentPage: 1, limit: 20, totalPages: 1, totalRecords: 0, targetDbId: null, sortBy: 'category_name', sortOrder: 'ASC' },
    }
};

/**
 * Initializes the state with given values.
 * This is useful for setting up the state when the application loads,
 * for example, after fetching initial data from the backend.
 * @param {Object} initialValues - An object containing initial state values to merge.
 */
export function initState(initialValues = {}) {
    if (initialValues.currentTargetId !== undefined) {
        state.currentTargetId = initialValues.currentTargetId;
    }
    if (initialValues.currentTargetName !== undefined) {
        state.currentTargetName = initialValues.currentTargetName;
    }
    if (initialValues.globalTableLayouts !== undefined) {
        state.globalTableLayouts = { ...state.globalTableLayouts, ...initialValues.globalTableLayouts };
    }
    // You can extend this to initialize other specific parts of the state if needed
    // For paginationState and viewConfig, the defaults are already set.
    // If initialValues contains these, they will be merged by a subsequent updateState call if necessary,
    // or you can add specific merging logic here.
    console.log("[StateService] Initialized with:", initialValues, "Current state:", JSON.parse(JSON.stringify(state)));
}

/**
 * Returns a deep copy of the current application state.
 * @returns {Object} The current state.
 */
export function getState() {
    // Return a deep copy to prevent direct modification of the internal state object.
    // JSON.parse(JSON.stringify(state)) is a common way for simple deep copies.
    // For more complex objects (with functions, Dates, etc.), a more robust deep copy mechanism might be needed.
    return JSON.parse(JSON.stringify(state));
}

/**
 * Updates the application state by merging the given updates.
 * @param {Object} newStateUpdates - An object containing the state properties to update.
 */
export function updateState(newStateUpdates) {
    // A more sophisticated merge might be needed for nested objects
    // For example, for paginationState, you might want to merge deeply.
    if (newStateUpdates.paginationState) {
        for (const key in newStateUpdates.paginationState) {
            if (state.paginationState.hasOwnProperty(key)) {
                state.paginationState[key] = {
                    ...state.paginationState[key],
                    ...newStateUpdates.paginationState[key]
                };
            } else {
                 state.paginationState[key] = newStateUpdates.paginationState[key];
            }
        }
        delete newStateUpdates.paginationState; // Remove it so it's not shallow copied below
    }

    if (newStateUpdates.jsAnalysisDataCache) {
        // For cache, we might want to merge or replace based on keys
        // This example replaces the specific logId entry if provided, or merges the whole cache
        if (typeof newStateUpdates.jsAnalysisDataCache === 'object' && newStateUpdates.jsAnalysisDataCache !== null) {
            Object.keys(newStateUpdates.jsAnalysisDataCache).forEach(logIdKey => {
                state.jsAnalysisDataCache[logIdKey] = newStateUpdates.jsAnalysisDataCache[logIdKey];
            });
        }
        delete newStateUpdates.jsAnalysisDataCache;
    }
    
    if (newStateUpdates.globalTableLayouts) {
        state.globalTableLayouts = { ...state.globalTableLayouts, ...newStateUpdates.globalTableLayouts };
        delete newStateUpdates.globalTableLayouts;
    }


    // For other top-level properties, a simple spread should be fine.
    state = {
        ...state,
        ...newStateUpdates
    };
    console.log("[StateService] State updated. New state:", JSON.parse(JSON.stringify(state)));
}

// Example of specific getters if needed, though getState() is often sufficient.
/**
 * Gets the current target ID.
 * @returns {number|null} The current target ID.
 */
export function getCurrentTargetId() {
    return state.currentTargetId;
}

/**
 * Gets the current target name.
 * @returns {string} The current target name.
 */
export function getCurrentTargetName() {
    return state.currentTargetName;
}

/**
 * Gets the pagination state for a specific key.
 * @param {string} key - The key for the pagination state (e.g., 'proxyLog').
 * @returns {Object|undefined} The pagination state for the key, or undefined.
 */
export function getPaginationState(key) {
    return state.paginationState[key] ? JSON.parse(JSON.stringify(state.paginationState[key])) : undefined;
}

/**
 * Gets the view configuration for a specific key.
 * @param {string} key - The key for the view configuration.
 * @returns {Object|undefined} The view configuration for the key, or undefined.
 */
export function getViewConfig(key) {
    return state.viewConfig[key] ? JSON.parse(JSON.stringify(state.viewConfig[key])) : undefined;
}
