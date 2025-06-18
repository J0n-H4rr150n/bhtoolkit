// static/router.js

let viewContentContainer;
let sidebarItems;
let updateBreadcrumbs;
let showModalConfirm;
let showModalMessage;
let API_BASE_URL;

let getState = () => ({});
let setState = () => {};

let viewLoaders = {};

let getPlatformDetailsFunc;
let cancelTargetEditFunc;
let cancelChecklistItemEditFunc;


/**
 * Fetches platform details. This is a wrapper around the actual function passed during init.
 * @param {number|string} platformId The ID of the platform.
 * @returns {Promise<Object|null>} Platform details or null.
 */
async function getPlatformDetails(platformId) {
    if (!platformId) return null;
    if (!getPlatformDetailsFunc) {
        console.error("getPlatformDetailsFunc not initialized in router.");
        return null;
    }
    return getPlatformDetailsFunc(platformId);
}

/**
 * Loads the main view based on the viewId and parameters.
 * @param {string} viewId The identifier for the view to load.
 * @param {Object} params Parameters extracted from the URL hash.
 */
async function loadView(viewId, params = {}) {
    const appState = getState();


    if (viewId === "undefined" || typeof viewId === 'undefined' || viewId === null || viewId === "") {
        viewId = 'platforms';
        params = {};
    }

    if (cancelTargetEditFunc) cancelTargetEditFunc();
    if (cancelChecklistItemEditFunc) cancelChecklistItemEditFunc();

    document.querySelectorAll('.sidebar-item').forEach(item => item.classList.remove('active'));
    const activeItem = document.querySelector(`.sidebar-item[data-view="${viewId}"]`);
    if (activeItem) {
        activeItem.classList.add('active');
    }

    let breadcrumbSegments = [];
    let pageTitle = "Toolkit";

    switch (viewId) {
        case 'platforms':
            pageTitle = "Platforms";
            breadcrumbSegments = [{ name: "Platforms" }];
            break;
        case 'targets':
            pageTitle = "Targets";
            if (params.platform_id) {
                const platform = await getPlatformDetails(params.platform_id);
                const platformName = platform ? platform.name : `ID ${params.platform_id}`;
                pageTitle = `Targets for ${platformName}`;
                breadcrumbSegments = [
                    { name: "Platforms", hash: "#platforms" },
                    { name: platformName }
                ];
            } else {
                 breadcrumbSegments = [{ name: "All Targets" }];
            }
            break;
        case 'current-target':
            pageTitle = "Current Target";
            breadcrumbSegments = [{ name: "Target Details" }];
            break;
        case 'synack-targets':
            pageTitle = "Synack Targets";
            breadcrumbSegments = [{ name: "Synack Targets" }];
            break;
        case 'synack-analytics':
            pageTitle = "Synack Target Analytics";
            breadcrumbSegments = [{ name: "Synack Targets", hash: "#synack-targets" }, { name: "Analytics" }];
            break;
        case 'synack-missions':
            pageTitle = "Synack Missions";
            breadcrumbSegments = [{ name: "Synack Targets", hash: "#synack-targets" }, { name: "Missions" }];
            break;
        case 'proxy-log':
            pageTitle = "Proxy Log";
            breadcrumbSegments = [{ name: "Proxy Log" }];
            break;
        case 'proxy-log-detail':
            pageTitle = "Log Entry Detail";
            breadcrumbSegments = [{ name: "Proxy Log", hash: "#proxy-log" }, { name: "Detail" }];
            break;
        case 'sitemap':
            pageTitle = "Sitemap";
            breadcrumbSegments = [{ name: "Sitemap" }];
            break;
        case 'domains': // Changed from discovered-urls
            pageTitle = "Domains";
            breadcrumbSegments = [{ name: "Domains" }];
            break;
        case 'visualizer':
            pageTitle = "Visualizer";
            breadcrumbSegments = [{ name: "Visualizer" }];
            break;
        case 'settings':
            pageTitle = "Settings";
            breadcrumbSegments = [{ name: "Settings" }];
            break;
        case 'checklist-templates':
            pageTitle = "Checklist Templates";
            breadcrumbSegments = [{ name: "Checklist Templates" }];
            break;
        case 'modifier':
            pageTitle = "Request Modifier";
            breadcrumbSegments = [{ name: "Modifier" }];
            break;
        case 'page-sitemap': // New case for Page Sitemap
            pageTitle = "Page Sitemap";
            breadcrumbSegments = [{ name: "Page Sitemap" }];
            break;
        case 'domain-detail': // New case for Domain Detail view
            pageTitle = params.id ? `Domain Detail #${params.id}` : "Domain Detail";
            breadcrumbSegments = [{ name: "Domains", hash: "#domains" }, { name: `Detail ${params.id ? `(#${params.id})` : ''}` }];
            break;
        default:
            if (viewContentContainer) viewContentContainer.innerHTML = `<h1>Page Not Found</h1><p>The view '${viewId}' is not implemented or the path is incorrect. Please select an option from the sidebar.</p>`;
            if (updateBreadcrumbs) updateBreadcrumbs([{ name: "Page Not Found" }]);
            return;
    }
    if (updateBreadcrumbs) updateBreadcrumbs(breadcrumbSegments);
    document.title = `${pageTitle} - Toolkit`;

    if (!viewContentContainer) {
        console.error("viewContentContainer not found in router.js!");
        return;
    }
    viewContentContainer.innerHTML = `<h1>Loading ${pageTitle}...</h1>`;

    let newPaginationState = { ...appState.paginationState };
    let newGlobalStateUpdates = {};

    switch (viewId) {
        case 'platforms':
            if (viewLoaders.loadPlatformsViewModule) {
                viewLoaders.loadPlatformsViewModule(
                    viewContentContainer,
                    API_BASE_URL,
                    showModalConfirm,
                    showModalMessage,
                    (event) => viewLoaders.handleAddPlatformModule(event, API_BASE_URL, showModalMessage, () => viewLoaders.fetchAndDisplayPlatformsModule(API_BASE_URL, showModalConfirm, showModalMessage))
                );
            } else console.error("loadPlatformsViewModule not found in viewLoaders");
            break;
        case 'targets':
            if (viewLoaders.loadTargetsView) viewLoaders.loadTargetsView(params.platform_id);
            else console.error("loadTargetsView not found in viewLoaders");
            break;
        case 'current-target':
            if (viewLoaders.loadCurrentTargetView) viewLoaders.loadCurrentTargetView(params.id, params.tab); // Pass params.tab
            else console.error("loadCurrentTargetView not found in viewLoaders");
            break;
        case 'synack-targets':
            newPaginationState.synackTargets = {
                ...newPaginationState.synackTargets,
                currentPage: params.page || 1,
                sortBy: params.sort_by || appState.viewConfig.synackTargets.sortBy,
                sortOrder: params.sort_order || appState.viewConfig.synackTargets.sortOrder
            };
            setState({ paginationState: newPaginationState });
            if (viewLoaders.loadSynackTargetsView) viewLoaders.loadSynackTargetsView();
            else console.error("loadSynackTargetsView not found in viewLoaders");
            break;
        case 'synack-analytics':
            const targetDbIdForAnalytics = params.target_db_id || null;
            let analyticsSortBy, analyticsSortOrder;
            if (targetDbIdForAnalytics) {
                analyticsSortBy = params.sort_by || appState.viewConfig.synackAnalyticsTarget.sortBy;
                analyticsSortOrder = params.sort_order || appState.viewConfig.synackAnalyticsTarget.sortOrder;
            } else {
                analyticsSortBy = params.sort_by || appState.viewConfig.synackAnalyticsGlobal.sortBy;
                analyticsSortOrder = params.sort_order || appState.viewConfig.synackAnalyticsGlobal.sortOrder;
            }
            newPaginationState.synackAnalytics = {
                ...newPaginationState.synackAnalytics,
                targetDbId: targetDbIdForAnalytics,
                currentPage: params.page || 1,
                sortBy: analyticsSortBy,
                sortOrder: analyticsSortOrder
            };
            setState({ paginationState: newPaginationState });
            if (viewLoaders.loadSynackAnalyticsView) viewLoaders.loadSynackAnalyticsView();
            else console.error("loadSynackAnalyticsView not found in viewLoaders");
            break;
        case 'synack-missions':
            if (viewLoaders.loadSynackMissionsView) viewLoaders.loadSynackMissionsView();
            else console.error("loadSynackMissionsView not found in viewLoaders");
            break;
        case 'proxy-log':
            let newProxyLogSortBy = (params.sort_by && params.sort_by !== 'null' && params.sort_by !== 'undefined') ? params.sort_by : 'id'; // Default to 'id'
            let newProxyLogSortOrder = (params.sort_order && params.sort_order !== 'null' && params.sort_order !== 'undefined') ? params.sort_order.toUpperCase() : 'DESC';
            console.log('[Router] loadView for "proxy-log", received params from handleHashChange:', params);
            // Ensure sort order is valid, default to DESC
            if (newProxyLogSortOrder !== 'ASC' && newProxyLogSortOrder !== 'DESC') {
                newProxyLogSortOrder = 'DESC';
            }

            newPaginationState.proxyLog = {
                ...newPaginationState.proxyLog,
                currentPage: params.page || 1,
                sortBy: newProxyLogSortBy,
                sortOrder: newProxyLogSortOrder,
                filterFavoritesOnly: params.favorites_only === 'true',
                filterMethod: params.method || '', // Use parsed 'method' from hash
                filterStatus: params.status || '', // Use parsed 'status' from hash
                filterContentType: params.type || '', // Use parsed 'type' from hash
                filterSearchText: params.search || '', 
                showIncompleteOnly: params.show_incomplete_only === 'true', // Add this for checklist
                analysis_type: params.analysis_type || null // Ensure analysis_type is carried over
            };
            // The block below that resets filters if no '?' was in the hash might be problematic.
            if (!window.location.hash.includes('?')) {
                 newPaginationState.proxyLog.filterMethod = '';
                 newPaginationState.proxyLog.filterStatus = '';
                 newPaginationState.proxyLog.filterContentType = '';
                 newPaginationState.proxyLog.filterSearchText = '';
                 newPaginationState.proxyLog.filterFavoritesOnly = false;
                 // newPaginationState.proxyLog.showIncompleteOnly = true; // Reset for proxy log if needed, or handle separately
                 newPaginationState.proxyLog.analysis_type = null; // Reset analysis_type if no query params
            }
            setState({ paginationState: newPaginationState });
            console.log('[Router] loadView for "proxy-log", about to call viewLoader.loadProxyLogView with params:', newPaginationState.proxyLog);
            // Pass the just-calculated proxyLog state directly
            if (viewLoaders.loadProxyLogView) viewLoaders.loadProxyLogView(newPaginationState.proxyLog);
            else console.error("loadProxyLogView not found in viewLoaders");
            break;
        case 'current-target': // Assuming checklist is part of current-target view
            newPaginationState.targetChecklistItems = { // Ensure this state object exists
                ...appState.paginationState.targetChecklistItems, // Spread existing targetChecklistItems state
                currentPage: params.page || 1,
                sortBy: params.sort_by || appState.paginationState.targetChecklistItems.sortBy,
                sortOrder: params.sort_order || appState.paginationState.targetChecklistItems.sortOrder,
                filterQuery: params.filter || '',
                showIncompleteOnly: params.show_incomplete_only === 'true' // Default to true if param not present or false
            };
            setState({ paginationState: newPaginationState });
            if (viewLoaders.loadCurrentTargetView) viewLoaders.loadCurrentTargetView(params.id, params.tab);
            break;
        case 'proxy-log-detail':
            if (viewLoaders.loadProxyLogDetailView) viewLoaders.loadProxyLogDetailView(params.id);
            else console.error("loadProxyLogDetailView not found in viewLoaders");
            break;
        case 'sitemap':
            if (viewLoaders.loadSitemapView) viewLoaders.loadSitemapView();
            else console.error("loadSitemapView not found in viewLoaders");
            break;
        case 'domains':
            // Ensure domainsView pagination state exists and is up-to-date
            const currentDomainsState = appState.paginationState.domainsView || {};
            newPaginationState.domainsView = {
                ...currentDomainsState, // Spread existing or default
                currentPage: params.page || 1,
                sortBy: params.sort_by || currentDomainsState.sortBy || 'domain_name',
                sortOrder: params.sort_order ? params.sort_order.toUpperCase() : (currentDomainsState.sortOrder || 'ASC'),
                filterDomainName: params.domain_name_search || '',
                filterSource: params.source_search || '',
                filterIsInScope: params.is_in_scope !== undefined ? (params.is_in_scope === 'true') : null,
                limit: params.limit ? parseInt(params.limit, 10) : (currentDomainsState.limit || 25)
            };
             if (!window.location.hash.includes('?')) { // Reset if no query params
                 newPaginationState.domainsView.currentPage = 1;
                 newPaginationState.domainsView.filterDomainName = '';
                 newPaginationState.domainsView.filterSource = '';
                 newPaginationState.domainsView.filterIsInScope = null;
                 // Keep sortBy, sortOrder, limit as they might be user preferences not tied to specific filters
            }
            setState({ paginationState: newPaginationState });
            if (viewLoaders.loadDomainsView) viewLoaders.loadDomainsView(); // It will use the updated state
            else console.error("loadDomainsView not found in viewLoaders");
            break;
        case 'checklist-templates':
            newPaginationState.checklistTemplateItems = {
                ...newPaginationState.checklistTemplateItems,
                currentPage: params.page || 1
            };
            newGlobalStateUpdates.paginationState = newPaginationState;
            if (params.template_id !== undefined) {
                newGlobalStateUpdates.currentChecklistTemplateId = params.template_id ? params.template_id : null;
            }
            setState(newGlobalStateUpdates);
            if (viewLoaders.loadChecklistTemplatesView) viewLoaders.loadChecklistTemplatesView();
            else console.error("loadChecklistTemplatesView not found in viewLoaders");
            break;
        case 'settings': 
            if (viewLoaders.loadSettingsView) viewLoaders.loadSettingsView();
            else console.error("loadSettingsView not found in viewLoaders");
            break;
        case 'modifier':
            if (viewLoaders.loadModifierView) viewLoaders.loadModifierView(params);
            else console.error("loadModifierView not found in viewLoaders");
            break;
        case 'page-sitemap':
            if (viewLoaders.loadPageSitemapView) viewLoaders.loadPageSitemapView();
            else console.error("loadPageSitemapView not found in viewLoaders");
            break;
        case 'visualizer':
            if (viewLoaders.loadVisualizerView) viewLoaders.loadVisualizerView();
            else console.error("loadVisualizerView not found in viewLoaders");
             break;
        case 'domain-detail':
            if (viewLoaders.loadDomainDetailView) viewLoaders.loadDomainDetailView(viewContentContainer, params.id);
            else console.error("loadDomainDetailView not found in viewLoaders");
            break;
    }
}

/**
 * Handles changes to the URL hash, parsing it and calling loadView.
 */
function handleHashChange() {
    console.log('[Router] handleHashChange CALLED. Current window.location.hash:', window.location.hash);
    const hash = window.location.hash.substring(1); // Remove leading '#'
    let viewId = 'platforms';
    let params = {};
    let queryString = '';

    if (hash) {
        const indexOfQM = hash.indexOf('?');
        const indexOfAmp = hash.indexOf('&'); // Check for '&' as the first separator too

        let queryStartIndex = -1;

        // Determine the actual start of the query string
        if (indexOfQM !== -1 && (indexOfAmp === -1 || indexOfQM < indexOfAmp)) {
            queryStartIndex = indexOfQM;
        } else if (indexOfAmp !== -1) {
            queryStartIndex = indexOfAmp;
        }

        if (queryStartIndex !== -1) {
            viewId = hash.substring(0, queryStartIndex);
            queryString = hash.substring(queryStartIndex + 1); // Get the part after '?' or '&'
        } else {
            viewId = hash; // No query parameters
        }
        viewId = viewId || 'platforms'; // Ensure viewId is not empty if hash was just '?' or '&'

        if (queryString) {
            const queryParams = new URLSearchParams(queryString);
            console.log('[Router] handleHashChange: Raw query string from hash:', queryString);
            
            for (const [key, value] of queryParams.entries()) {
                console.log(`[Router] handleHashChange: Parsing queryParam - Key: "${key}", Value: "${value}"`);
                if (key === 'platform_id' || key === 'page' || key === 'id' || key === 'target_db_id' || key === 'template_id') {
                     params[key] = parseInt(value, 10);
                     if (isNaN(params[key])) {
                        console.warn(`[Router] handleHashChange: Failed to parse integer for ${key}: ${value}. Setting to undefined.`);
                        params[key] = undefined; 
                     }
                } else if (key === 'method') {
                    params.method = value; // Explicitly assign 'method'
                    console.log(`[Router] handleHashChange: Explicitly set params.method to "${value}"`);
                } else if (key === 'status') {
                    params.status = value;
                } else if (key === 'type') {
                    params.type = value;
                } else if (key === 'search') {
                    params.search = value;
                } else if (key === 'sort_by') {
                    params.sort_by = value;
                } else if (key === 'sort_order') {
                    params.sort_order = value.toUpperCase(); // Keep toUpperCase here
                } else if (key === 'favorites_only') {
                    params.favorites_only = value; // Keep as string, loadView handles conversion
                } else if (key === 'show_incomplete_only') { // New param for checklist
                    params.show_incomplete_only = value; 
                } else {
                    params[key] = value; // Catch-all for any other parameters
                    // Specifically capture 'tab' if it's not handled above
                    if (key === 'tab') params.tab = value;
                }
            }
        }
    }
    // Log a deep copy of the params object to avoid issues with console logging live objects
    console.log('[Router] handleHashChange: Final "params" object being passed to loadView:', JSON.parse(JSON.stringify(params)));
    loadView(viewId, params);
}

/**
 * Initializes the router.
 * @param {Object} dependencies Object containing necessary functions and DOM elements.
 *                         Expected: viewContentContainer, sidebarItems, updateBreadcrumbs,
 *                         showModalConfirm, showModalMessage, API_BASE_URL,
 *                         getState, setState, viewLoaders (obj), getPlatformDetailsFunc,
 *                         cancelTargetEditFunc, cancelChecklistItemEditFunc.
 */
export function initRouter(dependencies) {
    viewContentContainer = dependencies.viewContentContainer;
    sidebarItems = dependencies.sidebarItems;
    updateBreadcrumbs = dependencies.updateBreadcrumbs;
    showModalConfirm = dependencies.showModalConfirm;
    showModalMessage = dependencies.showModalMessage;
    API_BASE_URL = dependencies.API_BASE_URL;

    getState = dependencies.getState;
    setState = dependencies.setState;
    viewLoaders = dependencies.viewLoaders;
    getPlatformDetailsFunc = dependencies.getPlatformDetailsFunc;
    cancelTargetEditFunc = dependencies.cancelTargetEditFunc;
    cancelChecklistItemEditFunc = dependencies.cancelChecklistItemEditFunc;

    if (sidebarItems) {
        sidebarItems.forEach(item => {
            item.addEventListener('click', function(event) {
                event.preventDefault();
                const newViewId = this.getAttribute('data-view');
                if (newViewId) {
                    window.location.hash = `#${newViewId}`;
                } else {
                    console.warn("[DEBUG] router.js: Sidebar item clicked, but data-view attribute is missing or empty.", this);
                }
            });
        });
    } else {
        console.error("sidebarItems not provided to initRouter. Sidebar navigation will not work.");
    }

    window.addEventListener('hashchange', handleHashChange);
    // Process the initial hash when the router is initialized
    handleHashChange(); 
    console.log('[Router] Router initialized and initial hash processed.');
}
