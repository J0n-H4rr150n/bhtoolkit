// static/views/modifierView.js
import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

let apiService;
let uiService;
let stateService;
let tableService; // If needed for tables within modifier

let viewContentContainer;

// Module-level state for the "Add no-cache header" toggle
let autoAddNoCacheHeader = localStorage.getItem('modifierAddNoCacheHeader') === 'true';

export function initModifierView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService; // Assign if used
    console.log("[ModifierView] Initialized.");
}
// Helper function to format headers for display in a textarea
function localFormatHeaders(headersObj) {
    if (!headersObj || Object.keys(headersObj).length === 0) return '(No Headers)';
    return Object.entries(headersObj)
        .map(([key, value]) => `${escapeHtml(key)}: ${escapeHtml(Array.isArray(value) ? value.join(', ') : value)}`)
        .join('\n');
}

// Helper function to format (decode and potentially pretty-print) a response body
function localFormatBody(base64Body, contentType = '') {
    //if (!base64Body) return '(Empty body)'; // Consistent with proxyLogView
    if (!base64Body) return '';
    try {
        const textContent = atob(base64Body); // Base64 decode
        const lowerContentType = contentType.toLowerCase();

        if (lowerContentType.includes('json')) {
            try {
                // Pretty print JSON
                return JSON.stringify(JSON.parse(textContent), null, 2);
            } catch (e) {
                // Fallback for malformed JSON: display as escaped text
                return escapeHtml(textContent.replace(/[\x00-\x1F\x7F-\x9F]/g, '.'));
            }
        } else if (
            lowerContentType.includes('javascript') ||
            lowerContentType.includes('text') || // Catches text/plain, text/html, text/css etc.
            lowerContentType.includes('xml') ||
            lowerContentType.includes('svg') // SVG is XML-based and often text
        ) {
            // For JavaScript, HTML, CSS, XML, plain text, display as escaped text
            return escapeHtml(textContent.replace(/[\x00-\x1F\x7F-\x9F]/g, '.'));
        }
        // For other content types (e.g., images, binary), show a placeholder
        // Since we've already decoded, showing a placeholder for non-text is better.
        return `(Binary or non-displayable content type: ${escapeHtml(contentType)})`;
    } catch (e) {
        // If atob fails (not valid Base64)
        console.error("Error decoding base64 body in ModifierView:", e);
        return ``;
        //return `(Error decoding body: Not valid Base64 or an unexpected error occurred. Raw data (first 200 chars): ${escapeHtml(String(base64Body).substring(0,200))})`;
    }
}

// Helper function to auto-adjust textarea height
function autoAdjustTextareaHeight(textareaElement) {
    if (!textareaElement) return;
    textareaElement.style.height = 'auto'; // Reset height to recalculate
    // Add a small buffer (e.g., 2px) to prevent scrollbar from appearing unnecessarily in some browsers
    const scrollHeight = textareaElement.scrollHeight;
    textareaElement.style.height = `${scrollHeight + 2}px`;
}

export function loadModifierView(mainViewContainer, params = {}) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadModifierView!");
        return;
    }
    console.log("[ModifierView] loadModifierView called with params:", params);
    const appState = stateService.getState(); // Get current state
    const { currentTargetId, currentTargetName } = appState;


    viewContentContainer.innerHTML = `
        <div class="modifier-layout">
            <div id="modifierTasksPanel" class="modifier-sidebar">
                <div class="modifier-sidebar-header">
                    <h2>Tasks ${currentTargetId ? `for <span class="highlight">${escapeHtml(currentTargetName)}</span>` : ''}</h2>
                    <button id="modifierSidebarToggle" title="Toggle Task List">‹</button>
                </div>
                <div id="modifierGlobalActions" style="padding: 10px; border-bottom: 1px solid #ccc;">
                    <button id="deleteAllModifierTasksBtn" class="secondary small-button" 
                            ${!currentTargetId ? 'disabled title="No current target set"' : `title="Delete all tasks for target ${escapeHtml(currentTargetName)}"`}>
                        Delete All Tasks for Target
                    </button>
                </div>
                <div id="modifierTaskList" class="task-list">
                    <p>Loading tasks...</p>
                </div>
            </div>
            <div id="modifierResizer" class="modifier-resizer"></div>
            <div id="modifierMainPanel" class="modifier-main-content">
                <div id="modifierWorkspace">
                    <h2>Workspace</h2>
                    <p>Select a task from the list to view and modify its details.</p>
                </div>
            </div>
        </div>
    `;

    setupModifierLayoutControls();
    document.getElementById('deleteAllModifierTasksBtn')?.addEventListener('click', handleDeleteAllModifierTasksForTarget);

    // Fetch tasks first, then if a specific task_id is provided, load and highlight it.
    fetchAndDisplayModifierTasks().then(() => {
        if (params.task_id) {
            loadModifierTaskIntoWorkspace(params.task_id);
            // Attempt to highlight after tasks are displayed and specific task loaded
            highlightModifierTaskInList(params.task_id);
        }
    });
}

async function fetchAndDisplayModifierTasks() {
    // This function now returns a promise that resolves when tasks are displayed.
    const taskListDiv = document.getElementById('modifierTaskList');
    if (!taskListDiv) return;

    try {
        const tasks = await apiService.getModifierTasks(); // Add params if filtering/pagination needed
        if (tasks && tasks.length > 0) {
            // Ensure tasks are sorted by display_order from the backend
            // The backend GetModifierTasks already sorts by display_order ASC, created_at DESC
            let listHTML = '<ul id="modifierSortableTaskList">';
            tasks.forEach(task => {
                listHTML += `<li draggable="true" class="modifier-task-item" data-task-id="${task.id}" title="Base: ${escapeHtmlAttribute(task.base_request_method)} ${escapeHtmlAttribute(task.base_request_url)}">
                                ${escapeHtml(task.name || `Task ${task.id}`)}
                             </li>`;
            });
            listHTML += '</ul>';
            taskListDiv.innerHTML = listHTML;

            const sortableList = document.getElementById('modifierSortableTaskList');

            sortableList.querySelectorAll('li.modifier-task-item').forEach(item => {
                item.addEventListener('click', (e) => {
                    const taskId = e.currentTarget.dataset.taskId;
                    loadModifierTaskIntoWorkspace(taskId);
                    sortableList.querySelectorAll('li').forEach(li => li.classList.remove('active'));
                    e.currentTarget.classList.add('active');
                });

                // Drag and Drop listeners
                item.addEventListener('dragstart', handleDragStart);
                item.addEventListener('dragend', handleDragEnd);
            });
            sortableList.addEventListener('dragover', handleDragOver);
            sortableList.addEventListener('drop', handleDrop);
        } else {
            taskListDiv.innerHTML = '<p>No tasks sent to Modifier yet.</p>';
        }
    } catch (error) {
        console.error("Error fetching modifier tasks:", error);
        taskListDiv.innerHTML = `<p class="error-message">Error loading tasks: ${escapeHtml(error.message)}</p>`;
    }
}

let draggedItem = null;

function handleDragStart(event) {
    draggedItem = event.target;
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', event.target.dataset.taskId);
    // Add a slight delay to allow the browser to render the drag image
    setTimeout(() => {
        event.target.classList.add('dragging');
    }, 0);
}

function handleDragEnd(event) {
    event.target.classList.remove('dragging');
    draggedItem = null;
    // Remove any drop indicators
    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());
}

function handleDragOver(event) {
    event.preventDefault(); // Necessary to allow dropping
    event.dataTransfer.dropEffect = 'move';

    const container = event.currentTarget; // This should be the UL
    const afterElement = getDragAfterElement(container, event.clientY);

    // Clear previous indicators
    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());

    if (draggedItem && draggedItem !== afterElement && (!afterElement || draggedItem !== afterElement.previousSibling)) {
        const indicator = document.createElement('div');
        indicator.classList.add('drop-indicator');
        if (afterElement == null) {
            container.appendChild(indicator);
        } else {
            container.insertBefore(indicator, afterElement);
        }
    }
}

function getDragAfterElement(container, y) {
    const draggableElements = [...container.querySelectorAll('li.modifier-task-item:not(.dragging)')];

    return draggableElements.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = y - box.top - box.height / 2;
        if (offset < 0 && offset > closest.offset) {
            return { offset: offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
}

async function handleDrop(event) {
    event.preventDefault();
    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());
    if (!draggedItem) return;

    const container = event.currentTarget; // The UL
    const afterElement = getDragAfterElement(container, event.clientY);

    if (afterElement == null) {
        container.appendChild(draggedItem);
    } else {
        container.insertBefore(draggedItem, afterElement);
    }
    draggedItem.classList.remove('dragging'); // Ensure class is removed
    await saveModifierTasksOrder();
}

function highlightModifierTaskInList(taskId) {
    const taskListDiv = document.getElementById('modifierTaskList');
    if (!taskListDiv) return;

    taskListDiv.querySelectorAll('li').forEach(li => li.classList.remove('active'));
    const taskToActivate = taskListDiv.querySelector(`li[data-task-id="${taskId}"]`);
    if (taskToActivate) taskToActivate.classList.add('active');
}

async function loadModifierTaskIntoWorkspace(taskId) {
    const workspaceDiv = document.getElementById('modifierWorkspace');
    if (!workspaceDiv) return;

    workspaceDiv.innerHTML = `<p>Loading task ID: ${escapeHtml(taskId)}...</p>`;

    try {
        const task = await apiService.getModifierTaskDetails(taskId);
        stateService.updateState({ currentModifierTask: task }); // Store in state

        // --- Prepare Request Part ---
        let requestHeadersFormatted = '(No Headers)';
        if (task.base_request_headers && task.base_request_headers.Valid && task.base_request_headers.String) {
            try {
                const headersObj = JSON.parse(task.base_request_headers.String);
                requestHeadersFormatted = localFormatHeaders(headersObj);
            } catch (e) {
                console.warn("Could not parse base_request_headers JSON:", e);
                requestHeadersFormatted = escapeHtml(task.base_request_headers.String);
            }
        }

        let requestBodyDecoded = '';
        if (task.base_request_body && task.base_request_body.Valid && task.base_request_body.String) {
            try {
                const decoded = atob(task.base_request_body.String);
                try {
                    const jsonObj = JSON.parse(decoded);
                    requestBodyDecoded = JSON.stringify(jsonObj, null, 2);
                } catch (e) {
                    requestBodyDecoded = decoded; // Keep as decoded text
                }
            } catch (e) {
                console.warn("Could not decode base64 base_request_body:", e);
                //requestBodyDecoded = '(Empty Body)';
                requestBodyDecoded = '';
            }
        }

        // --- Prepare Response Part (Defaults) ---
        let initialResponseStatus = '';
        let initialResponseHeaders = '(No Response Headers)';
        let initialResponseBody = '(No Response Body)';
        let activateResponseTab = false;

        if (task.last_executed_log_id && task.last_executed_log_id.Valid) {
            try {
                const lastExecutedLog = await apiService.getProxyLogDetail(task.last_executed_log_id.Int64);
                
                const reasonPhrase = (lastExecutedLog.response_reason_phrase && lastExecutedLog.response_reason_phrase.Valid) 
                                     ? lastExecutedLog.response_reason_phrase.String 
                                     : '';
                initialResponseStatus = lastExecutedLog.response_status_code ? `${lastExecutedLog.response_status_code} ${reasonPhrase}`.trim() : 'N/A';

                let resHeadersObj = {};
                // Correctly handle the sql.NullString structure for response_headers
                if (lastExecutedLog.response_headers && lastExecutedLog.response_headers.Valid && lastExecutedLog.response_headers.String) {
                    try {
                        resHeadersObj = JSON.parse(lastExecutedLog.response_headers.String);
                    } catch (e) {
                        console.warn("Error parsing response headers JSON string from last executed log", e, "Original string:", lastExecutedLog.response_headers.String);
                        resHeadersObj = {}; // Fallback
                    }
                } else {
                    console.log("[ModifierView] Response headers from last executed log are not valid or not present.");
                    // resHeadersObj remains {}
                }
                initialResponseHeaders = localFormatHeaders(resHeadersObj);
                
                // Extract string from sql.NullString for content type
                const responseContentTypeFromLog = (lastExecutedLog.response_content_type && lastExecutedLog.response_content_type.Valid)
                                                   ? lastExecutedLog.response_content_type.String
                                                   : '';
                initialResponseBody = localFormatBody(lastExecutedLog.response_body, responseContentTypeFromLog);
                activateResponseTab = true;


            } catch (logFetchError) {
                 console.error(`Error fetching last executed log ID ${task.last_executed_log_id.Int64}:`, logFetchError);
                 initialResponseStatus = `Error loading last response: ${escapeHtml(logFetchError.message)}`;
            }
        }

        let sourceInfoHTML = '';
        if (task.source_log_id && task.source_log_id.Valid && task.source_log_id.Int64 !== 0) {
            const logLink = `#proxy-log-detail?id=${task.source_log_id.Int64}`;
            sourceInfoHTML = `<p style="margin-bottom: 5px;"><strong>Source Log ID:</strong> <a href="${logLink}" title="Ctrl+Click to open source log in new tab">${task.source_log_id.Int64}</a>`;
            if (task.source_param_url_id && task.source_param_url_id.Valid && task.source_param_url_id.Int64 !== 0) {
                sourceInfoHTML += ` (Example for PURL ID: ${task.source_param_url_id.Int64})`;
            }
            sourceInfoHTML += `</p>`;
        } else if (task.source_param_url_id && task.source_param_url_id.Valid && task.source_param_url_id.Int64 !== 0) {
            sourceInfoHTML = `<p style="margin-bottom: 5px;"><strong>Source:</strong> Parameterized URL ID: ${task.source_param_url_id.Int64} (No direct example log linked)</p>`;
        }

        // --- Construct and Set Workspace HTML ---
        workspaceDiv.innerHTML = `
            <div class="modifier-task-header">
                <h2 id="modifierTaskNameDisplay" data-task-id="${task.id}">${escapeHtml(task.name || `Task ${task.id}`)}</h2>
                <div id="modifierTaskNameEditControls" style="display: inline-block; margin-left: 10px;">
                    <button id="editModifierTaskNameBtn" class="action-button inline-edit-button" title="Edit Task Name" style="margin-right: 5px;">✏️</button>
                    <button id="cloneModifierTaskBtn" class="action-button inline-edit-button" title="Clone Task" style="margin-right: 5px;">🐑</button> 
                    <button id="deleteModifierTaskBtn" class="action-button inline-edit-button" title="Delete Task">🗑️</button>
                </div>
                <div id="modifierTaskNameInputContainer" style="display: none; margin-bottom:10px;">
                    <input type="text" id="modifierTaskNameInput" class="modifier-input" value="${escapeHtmlAttribute(task.name || `Task ${task.id}`)}" style="width: 500px; margin-right: 5px;">
                    <button id="saveModifierTaskNameBtn" class="primary small-button">Save</button>
                    <button id="cancelModifierTaskNameBtn" class="secondary small-button">Cancel</button>
                </div>
            </div>
            ${sourceInfoHTML} 
            <div id="modifierRequestStatusMessage" class="status-message-area" style="margin-bottom: 10px; display: none;"></div>

            <div class="modifier-tabs">
                <button class="modifier-tab-button ${!activateResponseTab ? 'active' : ''}" data-tab-id="modifierRequestTab">Request</button>
                <button class="modifier-tab-button ${activateResponseTab ? 'active' : ''}" data-tab-id="modifierResponseTab">Response</button>
            </div>

            <div id="modifierRequestTab" class="modifier-tab-content ${!activateResponseTab ? 'active' : ''}">
                <div class="request-details">
                    <div class="modifier-section request-section">
                        <div class="form-group">
                            <label for="modMethod">Method:</label>
                            <input type="text" id="modMethod" class="modifier-input" value="${escapeHtmlAttribute(task.base_request_method)}">
                        </div>
                        <div class="form-group">
                            <label for="modURL">URL:</label>
                            <input type="text" id="modURL" class="modifier-input" value="${escapeHtmlAttribute(task.base_request_url)}">
                        </div>
                        <div class="form-group">
                            <label for="modHeaders">Headers:</label>
                            <textarea id="modHeaders" class="modifier-textarea" rows="8">${requestHeadersFormatted}</textarea>
                        </div>
                        <div class="form-group">
                            <label for="modBody">Body:</label>
                            <textarea id="modBody" class="modifier-textarea" rows="10">${escapeHtml(requestBodyDecoded)}</textarea>
                        </div>
                        <div class="form-group" style="display: flex; align-items: center; margin-bottom: 10px; margin-top: 10px;">
                            <input type="checkbox" id="modAddNoCacheHeaderToggle" style="margin-right: 8px;" ${autoAddNoCacheHeader ? 'checked' : ''}>
                            <label for="modAddNoCacheHeaderToggle" style="font-weight:normal;">Ensure "Cache-Control: no-cache" header is sent</label>
                        </div>
                        <button id="sendModifiedRequestBtn" class="primary" style="margin-top: 5px;">Send Request</button>
                    </div>
                </div>
            </div>

            <div id="modifierResponseTab" class="modifier-tab-content ${activateResponseTab ? 'active' : ''}">
                <div class="response-section">
                    <h3>Response:</h3>
                    <div class="form-group" style="margin-bottom: 10px;">
                        <label for="modResponseStatus">Status:</label>
                        <input type="text" id="modResponseStatus" class="modifier-input" readonly placeholder="e.g., 200 OK" value="${escapeHtmlAttribute(initialResponseStatus)}">
                    </div>
                    <div class="form-group">
                        <label for="modResponseHeaders">Response Headers:</label>
                        <textarea id="modResponseHeaders" class="modifier-textarea" rows="12" readonly placeholder="Response headers will appear here...">${initialResponseHeaders}</textarea>
                    </div>
                    <div class="form-group">
                        <label for="modResponseBody">Response Body:</label>
                        <textarea id="modResponseBody" class="modifier-textarea" rows="10" readonly placeholder="Response body will appear here...">${initialResponseBody}</textarea>
                    </div>
                </div>
            </div>
        `;

        // --- Add Event Listeners ---
        document.getElementById('sendModifiedRequestBtn')?.addEventListener('click', () => handleSendModifiedRequest(task.id));
        document.getElementById('editModifierTaskNameBtn')?.addEventListener('click', () => toggleTaskNameEdit(true, task.id, task.name));
        document.getElementById('saveModifierTaskNameBtn')?.addEventListener('click', () => handleSaveTaskName(task.id));
        document.getElementById('cancelModifierTaskNameBtn')?.addEventListener('click', () => toggleTaskNameEdit(false, task.id, task.name));
        document.getElementById('cloneModifierTaskBtn')?.addEventListener('click', () => handleCloneModifierTask(task.id));
        document.getElementById('deleteModifierTaskBtn')?.addEventListener('click', () => handleDeleteModifierTask(task.id, task.name || `Task ${task.id}`));
        document.querySelectorAll('.modifier-tab-button').forEach(button => {
            button.addEventListener('click', () => setActiveModifierTab(button.dataset.tabId));
        });

        const noCacheToggleElement = document.getElementById('modAddNoCacheHeaderToggle');
        if (noCacheToggleElement) {
            noCacheToggleElement.addEventListener('change', (event) => {
                autoAddNoCacheHeader = event.target.checked;
                localStorage.setItem('modifierAddNoCacheHeader', autoAddNoCacheHeader);
            });
        }
        
        // Adjust height for response body if it was populated
        const responseBodyTextarea = document.getElementById('modResponseBody');
        autoAdjustTextareaHeight(responseBodyTextarea);

        // Set active tab based on whether a response was loaded
        if (activateResponseTab) {
            setActiveModifierTab('modifierResponseTab');
        } else {
            setActiveModifierTab('modifierRequestTab'); // Default to request tab
        }

        highlightModifierTaskInList(taskId); // Also highlight when a task is loaded by clicking
    } catch (error) {
        console.error(`Error loading modifier task ${taskId} into workspace:`, error);
        workspaceDiv.innerHTML = `<p class="error-message">Error loading task details: ${escapeHtml(error.message)}</p>`;
    }
}

function setActiveModifierTab(tabIdToActivate) {
    // Deactivate all tab buttons and content
    document.querySelectorAll('.modifier-tab-button').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelectorAll('.modifier-tab-content').forEach(content => {
        content.classList.remove('active');
    });

    // Activate the selected tab button and content
    document.querySelector(`.modifier-tab-button[data-tab-id="${tabIdToActivate}"]`)?.classList.add('active');
    document.getElementById(tabIdToActivate)?.classList.add('active');
}

function setupModifierLayoutControls() {
    const tasksPanel = document.getElementById('modifierTasksPanel');
    const resizer = document.getElementById('modifierResizer');
    const mainPanel = document.getElementById('modifierMainPanel'); 
    const toggleButton = document.getElementById('modifierSidebarToggle'); 

    let isResizing = false;
    const storedWidth = localStorage.getItem('modifierSidebarWidth');
    if (storedWidth && tasksPanel) {
        tasksPanel.style.width = storedWidth;
    }

    if (resizer && tasksPanel && mainPanel) { 
        resizer.addEventListener('mousedown', (e) => {
            isResizing = true;
            document.body.style.cursor = 'col-resize'; 
            document.body.style.userSelect = 'none'; 

            const startX = e.clientX;
            const startWidth = tasksPanel.offsetWidth;

            const doDrag = (moveEvent) => {
                if (!isResizing) return;
                const newWidth = startWidth + (moveEvent.clientX - startX);
                if (newWidth > 150 && newWidth < (window.innerWidth * 0.7)) { 
                    tasksPanel.style.width = `${newWidth}px`;
                }
            };

            const stopDrag = () => {
                if (!isResizing) return;
                isResizing = false;
                document.body.style.cursor = 'default';
                document.body.style.userSelect = 'auto';
                localStorage.setItem('modifierSidebarWidth', tasksPanel.style.width);
                document.removeEventListener('mousemove', doDrag);
                document.removeEventListener('mouseup', stopDrag);
            };

            document.addEventListener('mousemove', doDrag);
            document.addEventListener('mouseup', stopDrag);
        });
    }

    const storedCollapsed = localStorage.getItem('modifierSidebarCollapsed') === 'true';
    if (tasksPanel && storedCollapsed) {
        tasksPanel.classList.add('collapsed');
        if (mainPanel) mainPanel.classList.add('sidebar-collapsed-sibling'); 
        if (toggleButton) toggleButton.textContent = '›';
        if(resizer) resizer.style.display = 'none';
    }

    if (toggleButton && tasksPanel && mainPanel) { 
        toggleButton.addEventListener('click', () => {
            const isCollapsed = tasksPanel.classList.toggle('collapsed');
            mainPanel.classList.toggle('sidebar-collapsed-sibling', isCollapsed);
            toggleButton.textContent = isCollapsed ? '›' : '‹';
            localStorage.setItem('modifierSidebarCollapsed', isCollapsed);
            if(resizer) resizer.style.display = isCollapsed ? 'none' : 'flex'; 
        });
    }
}

async function handleSendModifiedRequest(taskId) {
    const method = document.getElementById('modMethod').value;
    const url = document.getElementById('modURL').value;
    let headersStringForSending = document.getElementById('modHeaders').value; 
    const body = document.getElementById('modBody').value; 
    const requestStatusMessageEl = document.getElementById('modifierRequestStatusMessage');
    const sendButton = document.getElementById('sendModifiedRequestBtn');

    const responseStatusEl = document.getElementById('modResponseStatus');
    const responseHeadersEl = document.getElementById('modResponseHeaders');
    const responseBodyEl = document.getElementById('modResponseBody');
    const currentTask = stateService.getState().currentModifierTask; 

    if(responseStatusEl) responseStatusEl.value = '';
    if(responseHeadersEl) responseHeadersEl.value = '';
    if(responseBodyEl) responseBodyEl.value = '';
    if(requestStatusMessageEl) {
        requestStatusMessageEl.textContent = '';
        requestStatusMessageEl.style.display = 'none';
    }

    if(sendButton) sendButton.disabled = true;
    if(requestStatusMessageEl) { requestStatusMessageEl.textContent = 'Sending...'; requestStatusMessageEl.className = 'status-message-area info'; requestStatusMessageEl.style.display = 'block'; }

    if (autoAddNoCacheHeader) {
        let headersLines = headersStringForSending.split('\n').map(line => line.trimEnd());
        let cacheControlIndex = -1;
        let existingCacheControlLine = '';

        for (let i = 0; i < headersLines.length; i++) {
            if (headersLines[i].toLowerCase().startsWith('cache-control:')) {
                cacheControlIndex = i;
                existingCacheControlLine = headersLines[i];
                break;
            }
        }

        const noCacheDirective = 'no-cache';
        if (cacheControlIndex !== -1) { // Cache-Control header exists
            // Check if 'no-cache' (as a whole word) is already present
            const directives = existingCacheControlLine.substring(existingCacheControlLine.indexOf(':') + 1).split(',').map(d => d.trim().toLowerCase());
            if (!directives.includes(noCacheDirective)) {
                headersLines[cacheControlIndex] = existingCacheControlLine + (existingCacheControlLine.endsWith(':') ? '' : ', ') + noCacheDirective;
            }
        } else { // No Cache-Control header, add one
            // Add to the end, ensuring it's not an empty line if headersStringForSending was empty
            if (headersLines.length === 1 && headersLines[0].trim() === '') {
                headersLines[0] = `Cache-Control: ${noCacheDirective}`;
            } else {
                headersLines.push(`Cache-Control: ${noCacheDirective}`);
            }
        }
        headersStringForSending = headersLines.filter(line => line.trim() !== '').join('\n'); // Filter out any potentially fully empty lines
    }

    try {
        const responseData = await apiService.executeModifiedRequest({
            task_id: currentTask?.id, 
            method: method,
            url: url,
            headers: headersStringForSending, 
            body: body        
        });

        if (responseData.error) { // Check for backend-reported error
            if(requestStatusMessageEl) {
                requestStatusMessageEl.textContent = `Error from server: ${escapeHtml(responseData.error)}`;
                requestStatusMessageEl.className = 'status-message-area error';
                requestStatusMessageEl.style.display = 'block';
            }
            // Still try to display headers/status if available
            if(responseStatusEl) responseStatusEl.value = responseData.status_text || (responseData.status_code ? String(responseData.status_code) : 'Error');
            if(responseHeadersEl) responseHeadersEl.value = localFormatHeaders(responseData.headers || {});
            // Display the error in the body area as well, or indicate body is unavailable
            if(responseBodyEl) responseBodyEl.value = `(Response body not available due to error: ${escapeHtml(responseData.error)})`;
            setActiveModifierTab('modifierResponseTab'); // Switch to response tab to show error context
            return; // Stop further processing of the response body
        }

        console.log("[ModifierView] handleSendModifiedRequest - raw responseData received from apiService:", JSON.parse(JSON.stringify(responseData)));

        if(responseStatusEl) responseStatusEl.value = responseData.status_text || responseData.status_code || 'N/A';
        
        if(responseHeadersEl) {
            responseHeadersEl.value = localFormatHeaders(responseData.headers);
        }
        
        if(responseBodyEl) {
            let responseContentType = '';
            if (responseData.headers) {
                for (const key in responseData.headers) {
                    if (key.toLowerCase() === 'content-type') {
                        responseContentType = (responseData.headers[key] && responseData.headers[key][0]) ? responseData.headers[key][0].toLowerCase() : '';
                        break;
                    }
                }
            }
            // Log the body string and content type before formatting
            console.log("[ModifierView] handleSendModifiedRequest - responseData.body (length, first 100 chars):", responseData.body?.length, responseData.body?.substring(0,100));
            console.log("[ModifierView] handleSendModifiedRequest - responseContentType for localFormatBody:", responseContentType);
            responseBodyEl.value = localFormatBody(responseData.body, responseContentType);
            autoAdjustTextareaHeight(responseBodyEl); // Adjust height after setting content
        }

        if(requestStatusMessageEl) {
            requestStatusMessageEl.textContent = 'Response received successfully.';
            requestStatusMessageEl.className = 'status-message-area success';
            setActiveModifierTab('modifierResponseTab'); 
        }
        // After successfully sending and getting a response, reload the task to get the updated last_executed_log_id
        if (currentTask?.id) {
            await loadModifierTaskIntoWorkspace(currentTask.id); // This will re-fetch and re-render
        }


    } catch (error) {
        console.error("Error sending modified request:", error);
        if(responseStatusEl) responseStatusEl.value = `Error: ${error.message}`;
        if(requestStatusMessageEl) { requestStatusMessageEl.textContent = `Error: ${escapeHtml(error.message)}`; requestStatusMessageEl.className = 'status-message-area error'; requestStatusMessageEl.style.display = 'block'; }
    } finally {
        if(sendButton) sendButton.disabled = false;
    }
}

function toggleTaskNameEdit(isEditing, taskId, currentName) {
    const nameDisplay = document.getElementById('modifierTaskNameDisplay');
    const editControls = document.getElementById('modifierTaskNameEditControls');
    const inputContainer = document.getElementById('modifierTaskNameInputContainer');
    const nameInput = document.getElementById('modifierTaskNameInput');

    if (!nameDisplay || !editControls || !inputContainer || !nameInput) return;

    if (isEditing) {
        nameDisplay.style.display = 'none';
        editControls.style.display = 'none';
        inputContainer.style.display = 'block';
        nameInput.value = currentName || `Task ${taskId}`;
        nameInput.focus();
        nameInput.select();
    } else {
        nameDisplay.style.display = 'inline-block'; 
        editControls.style.display = 'inline-block';
        inputContainer.style.display = 'none';
    }
}

async function handleSaveTaskName(taskId) {
    const nameInput = document.getElementById('modifierTaskNameInput');
    const newName = nameInput.value.trim();

    if (!newName) {
        uiService.showModalMessage("Validation Error", "Task name cannot be empty.");
        return;
    }

    const originalButtonText = document.getElementById('saveModifierTaskNameBtn')?.textContent;
    if (document.getElementById('saveModifierTaskNameBtn')) document.getElementById('saveModifierTaskNameBtn').textContent = 'Saving...';
    if (document.getElementById('saveModifierTaskNameBtn')) document.getElementById('saveModifierTaskNameBtn').disabled = true;

    try {
        const updatedTask = await apiService.updateModifierTask(taskId, { name: newName });

        document.getElementById('modifierTaskNameDisplay').textContent = escapeHtml(updatedTask.name);
        toggleTaskNameEdit(false, taskId, updatedTask.name);
        await fetchAndDisplayModifierTasks(); 
        highlightModifierTaskInList(taskId); 
        uiService.showModalMessage("Success", `Task name updated to "${escapeHtml(updatedTask.name)}".`);
    } catch (error) {
        console.error("Error saving task name:", error);
        uiService.showModalMessage("Error", `Failed to save task name: ${escapeHtml(error.message)}`);
    } finally {
        if (document.getElementById('saveModifierTaskNameBtn') && originalButtonText) {
            document.getElementById('saveModifierTaskNameBtn').textContent = originalButtonText;
            document.getElementById('saveModifierTaskNameBtn').disabled = false;
        }
    }
}

async function handleCloneModifierTask(originalTaskId) {
    console.log(`Attempting to clone task ID ${originalTaskId}`);
    uiService.showModalMessage("Cloning...", `Cloning task ${originalTaskId}...`, true, 1500);

    try {
        const clonedTask = await apiService.cloneModifierTask(originalTaskId);

        await fetchAndDisplayModifierTasks(); 
        loadModifierTaskIntoWorkspace(clonedTask.id); 
        uiService.showModalMessage("Success", `Task cloned successfully as "${escapeHtml(clonedTask.name)}".`);
    } catch (error) {
        console.error("Error cloning task:", error);
        uiService.showModalMessage("Error", `Failed to clone task: ${escapeHtml(error.message)}`);
    }
}

async function saveModifierTasksOrder() {
    const taskList = document.getElementById('modifierSortableTaskList');
    if (!taskList) return;

    const orderedTaskIds = [...taskList.querySelectorAll('li.modifier-task-item')]
        .map(item => item.dataset.taskId);

    const taskOrders = {};
    orderedTaskIds.forEach((id, index) => {
        taskOrders[id] = index; 
    });

    if (Object.keys(taskOrders).length === 0) return; 

    try {
        await apiService.updateModifierTasksOrder(taskOrders);
        await fetchAndDisplayModifierTasks(); 
    } catch (error) {
        console.error("Error saving task order:", error);
        uiService.showModalMessage("Error", `Failed to save task order: ${escapeHtml(error.message)}`);
    }
}

async function handleDeleteModifierTask(taskId, taskName) {
    if (!taskId) {
        uiService.showModalMessage("Error", "No task ID provided for deletion.");
        return;
    }

    uiService.showModalConfirm(
        "Confirm Delete Task",
        `Are you sure you want to delete the Modifier task "${escapeHtml(taskName)}"? This action cannot be undone.`,
        async () => {
            try {
                await apiService.deleteModifierTask(taskId); 
                uiService.showModalMessage("Success", `Task "${escapeHtml(taskName)}" deleted successfully.`, true, 2000);

                const workspaceDiv = document.getElementById('modifierWorkspace');
                if (workspaceDiv) {
                    workspaceDiv.innerHTML = '<h2>Workspace</h2><p>Select a task from the list to view and modify its details.</p>';
                }
                await fetchAndDisplayModifierTasks(); 
            } catch (error) {
                console.error("Error deleting modifier task:", error);
                uiService.showModalMessage("Error", `Failed to delete task: ${escapeHtml(error.message)}`);
            }
        }
    );
}

async function handleDeleteAllModifierTasksForTarget() {
    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    if (!currentTargetId) {
        uiService.showModalMessage("Error", "No current target selected to delete tasks for.");
        return;
    }

    uiService.showModalConfirm(
        "Confirm Delete All Tasks",
        `Are you sure you want to delete ALL Modifier tasks for target "${escapeHtml(currentTargetName)}" (ID: ${currentTargetId})? This action cannot be undone.`,
        async () => {
            try {
                // Assuming apiService will have a method like `deleteAllModifierTasksForTarget`
                // This would call a new backend endpoint, e.g., DELETE /api/modifier/tasks/target/{target_id}
                await apiService.deleteAllModifierTasksForTarget(currentTargetId);
                uiService.showModalMessage("Success", `All Modifier tasks for target "${escapeHtml(currentTargetName)}" have been deleted.`);
                
                // Clear the workspace and refresh the task list
                document.getElementById('modifierWorkspace').innerHTML = '<h2>Workspace</h2><p>Select a task from the list to view and modify its details.</p>';
                await fetchAndDisplayModifierTasks();
            } catch (error) {
                console.error("Error deleting all modifier tasks for target:", error);
                uiService.showModalMessage("Error", `Failed to delete tasks: ${escapeHtml(error.message)}`);
            }
        }
    );
}
