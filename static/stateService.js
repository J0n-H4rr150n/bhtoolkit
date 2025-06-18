// static/stateService.js

let state = {
    currentTargetId: null,
    currentTargetName: 'None',
    currentChecklistTemplateId: null,
    jsAnalysisDataCache: {},
    jsAnalysisSortState: { sortBy: 'category', sortOrder: 'ASC' },
    jsAnalysisFilterCategory: '', // New: For JS Analysis category filter
    jsAnalysisSearchText: '',     // New: For JS Analysis search text
    commentAnalysisDataCache: {}, 
    selectedSitemapHost: null, // New: For sitemap host filter
    appSettings: { // Store application settings fetched from backend
        ui: {
            DefaultTheme: 'light', // Default theme from config
            ShowSynackSection: false,
        },
    },
    commentAnalysisSortState: { sortBy: 'lineNumber', sortOrder: 'ASC' }, // New: Sort state for comments
    globalTableLayouts: {},
    viewConfig: {
        synackTargets: { sortBy: 'last_seen_timestamp', sortOrder: 'DESC', filterIsActive: true, sortableColumns: ['synack_target_id_str', 'codename', 'name', 'findings_count', 'status', 'last_seen_timestamp']},
        proxyLog: { sortBy: 'id', sortOrder: 'DESC', filterFavoritesOnly: false, sortableColumns: ['id', 'timestamp', 'method', 'url', 'status', 'type', 'size', 'duration'], analysis_type: null },
        synackAnalyticsGlobal: { sortBy: 'target_codename', sortOrder: 'ASC', sortableColumns: ['target_codename', 'category_name', 'count'] },
        synackAnalyticsTarget: { sortBy: 'reported_at', sortOrder: 'DESC', sortableColumns: ['synack_finding_id', 'title', 'category_name', 'severity', 'status', 'reported_at', 'vulnerability_url'] },
    },
    paginationState: {
        synackTargets: { currentPage: 1, limit: 20, totalPages: 1, totalRecords: 0, sortBy: 'last_seen_timestamp', sortOrder: 'DESC' },
        proxyLog: {
            currentPage: 1, limit: 15, // This 'limit' will be the runtime page size
            totalPages: 1, totalRecords: 0,
            sortBy: 'id', sortOrder: 'DESC', filterFavoritesOnly: false,
            filterMethod: '', filterStatus: '', filterContentType: '', filterSearchText: '',
            analysis_type: null // Added to proxyLog pagination state
        },
        targetChecklistItems: { // New state for target checklist
            currentPage: 1,
            limit: 10, 
            totalPages: 1,
            totalRecords: 0,
            sortBy: 'created_at', // Default sort
            sortOrder: 'asc',
            filterQuery: '',
            showIncompleteOnly: true 
        },
        checklistTemplateItems: {
            currentPage: 1,
            limit: 20,
            totalPages: 1, totalRecords: 0
        },
        proxyLogTableLayout: {
            // Note: 'pageSize' will be stored at the root of the saved layout for proxyLogTable, not in columnConfig
            index: { default: '3%', id: 'col-proxylog-index', visible: true, label: '#' },
            timestamp: { default: '15%', id: 'col-proxylog-timestamp', visible: true, label: 'Timestamp' },
            method: { default: '7%', id: 'col-proxylog-method', visible: true, label: 'Method' },
            page_name: { default: '15%', id: 'col-proxylog-page_name', visible: true, label: 'Page Name' }, // Added page_name
            url: { default: 'auto', id: 'col-proxylog-url', visible: true, label: 'URL' },
            status: { default: '7%', id: 'col-proxylog-status', visible: true, label: 'Status' },
            type: { default: '15%', id: 'col-proxylog-type', visible: true, label: 'Content-Type' },
            size: { default: '7%', id: 'col-proxylog-size', visible: true, label: 'Size (B)' },
            actions: { default: '150px', id: 'col-proxylog-actions', visible: true, label: 'Actions', nonResizable: true, nonHideable: true } // Actions column specific flags
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
            actions: { default: '130px', visible: true, id: 'col-paramurl-actions' }, // Increased width
        },
        synackAnalytics: { currentPage: 1, limit: 20, totalPages: 1, totalRecords: 0, targetDbId: null, sortBy: 'category_name', sortOrder: 'ASC' },
        pageSitemapLogs: { // New state for logs within Page Sitemap view
            currentPage: 1,
            limit: 20, // Default number of logs per page
            totalPages: 1,
            totalRecords: 0,
            sortBy: 'timestamp', // Default sort column
            sortOrder: 'DESC'    // Default sort order
        },
        domainsView: { // New state for Domains view
            currentPage: 1,
            limit: 25,
            sortBy: 'domain_name',
            sortOrder: 'ASC',
            filterDomainName: '',
            filterSource: '',
            filterIsFavorite: false, // New filter state
            filterIsInScope: null, // null means 'all' (no filter), true for 'In Scope', false for 'Out of Scope'
            filterHttpxScanStatus: 'all', // New: "all", "scanned", "not_scanned"
            filterHTTPStatusCode: '', // New for dropdown
            filterHTTPServer: '',     // New for dropdown
            filterHTTPTech: '',       // New for dropdown
            totalPages: 0,
            totalRecords: 0,
        },
        domainsTableLayout: { // New layout for Domains table
            checkbox: { default: '3%', id: 'col-domain-checkbox', visible: true, label: '<input type="checkbox" id="selectAllDomainsCheckbox" title="Select/Deselect All Visible">', sortKey: null, nonResizable: true, nonHideable: true, isHtmlLabel: true },
            id: { default: '4%', id: 'col-domain-row-number', visible: true, label: '#', sortKey: 'id' }, // Restore sortKey to 'id'
            is_favorite: { default: '4%', id: 'col-domain-favorite', visible: false, label: '★', sortKey: 'is_favorite', nonResizable: true }, // Hidden, will be part of actions
            domain_name: { default: '25%', id: 'col-domain-name', visible: true, label: 'Domain Name', sortKey: 'domain_name' },
            source: { default: '10%', id: 'col-domain-source', visible: true, label: 'Source', sortKey: 'source' },
            http_status_code: { default: '7%', id: 'col-domain-status', visible: true, label: 'Status', sortKey: 'http_status_code' },
            http_content_length: { default: '7%', id: 'col-domain-length', visible: true, label: 'Length', sortKey: 'http_content_length' },
            http_title: { default: '15%', id: 'col-domain-title', visible: true, label: 'Title', sortKey: 'http_title' },
            http_tech: { default: '15%', id: 'col-domain-tech', visible: true, label: 'Tech', sortKey: 'http_tech' },
            http_server: { default: '10%', id: 'col-domain-server', visible: true, label: 'Server', sortKey: 'http_server' },
            is_in_scope: { default: '8%', id: 'col-domain-inscope', visible: false, label: 'In Scope?', sortKey: 'is_in_scope' },
            is_wildcard_scope: { default: '8%', id: 'col-domain-wildcard', visible: false, label: 'Wildcard?', sortKey: 'is_wildcard_scope'}, // Hidden
            notes: { default: '15%', id: 'col-domain-notes', visible: true, label: 'Notes', sortKey: 'notes' },
            last_httpx_result: { default: '12%', id: 'col-domain-httpx-scan', visible: true, label: 'Last HTTPX Result', sortKey: 'updated_at' }, // New column
            created_at: { default: 'auto', id: 'col-domain-created', visible: true, label: 'Created At', sortKey: 'created_at' },
            actions: { default: '150px', id: 'col-domain-actions', visible: true, label: 'Actions', nonResizable: true, nonHideable: true }
        }, // Note: The actual database ID is still available in the `domain.id` object property for actions.
        commentAnalysisTableLayout: { // New layout for the comments table
            lineNumber: { default: '80px', id: 'col-comment-linenum', visible: true, label: 'Line#', sortKey: 'lineNumber' },
            commentType: { default: '150px', id: 'col-comment-type', visible: true, label: 'Type', sortKey: 'commentType' }, // Specific px width
            commentText: { default: 'auto', id: 'col-comment-text', visible: true, label: 'Comment Text', sortKey: null }, // Not sortable by text directly
            contextBefore: { default: '25%', id: 'col-comment-ctxbefore', visible: true, label: 'Context Before', sortKey: null },
            contextAfter: { default: '25%', id: 'col-comment-ctxafter', visible: true, label: 'Context After', sortKey: null }
        },
        synackMissionsView: { // New state for Synack Missions list
            currentPage: 1,
            limit: 25,
            sortBy: 'created_at', // Default sort column in DB
            sortOrder: 'DESC',
            totalPages: 0,
            totalRecords: 0,
        },
        synackMissionsTableLayout: { // New layout for Synack Missions table
            // Ensure sortKey matches the actual DB column name or an alias the backend can use
            id: { label: 'Mission ID', default: '15%', visible: true, sortKey: 'synack_task_id', id: 'col-mission-list-id' },
            title: { label: 'Title', default: '35%', visible: true, sortKey: 'title', id: 'col-mission-list-title' },
            payout_amount: { label: 'Payout', default: '10%', visible: true, sortKey: 'payout_amount', id: 'col-mission-list-payout' },
            status: { label: 'Status', default: '15%', visible: true, sortKey: 'status', id: 'col-mission-list-status' },
            // Assuming 'updated_at' or 'created_at' for "Last Seen"
            updated_at: { label: 'Last Seen/Updated', default: '15%', visible: true, sortKey: 'updated_at', id: 'col-mission-list-lastseen' },
            claimed_by_toolkit_at: { label: 'Claimed By Toolkit', default: 'auto', visible: true, sortKey: 'claimed_by_toolkit_at', id: 'col-mission-list-claimed' },
        }
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
    // Add this block to correctly initialize appSettings
    if (initialValues.appSettings !== undefined) {
        state.appSettings = initialValues.appSettings; // Directly assign the fetched appSettings
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
    if (newStateUpdates.commentAnalysisDataCache) { // New: Handle comment analysis cache
        if (typeof newStateUpdates.commentAnalysisDataCache === 'object' && newStateUpdates.commentAnalysisDataCache !== null) {
            Object.keys(newStateUpdates.commentAnalysisDataCache).forEach(logIdKey => {
                state.commentAnalysisDataCache[logIdKey] = newStateUpdates.commentAnalysisDataCache[logIdKey];
            });
        }
        delete newStateUpdates.commentAnalysisDataCache;
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
 */
state.modifierTasks = []; // Array to hold tasks sent to the modifier
state.currentModifierTask = null; // Details of the task currently being worked on
state.modifierViewConfig = {
    // any specific view configurations for modifier page
};

/**
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
