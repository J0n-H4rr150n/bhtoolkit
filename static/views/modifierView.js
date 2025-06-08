// static/views/modifierView.js
import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

let apiService;
let uiService;
let stateService;
let tableService; // If needed for tables within modifier

let viewContentContainer;

export function initModifierView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService; // Assign if used
    console.log("[ModifierView] Initialized.");
}

export function loadModifierView(mainViewContainer, params = {}) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadModifierView!");
        return;
    }
    console.log("[ModifierView] loadModifierView called with params:", params);

    viewContentContainer.innerHTML = `
        <div class="modifier-layout">
            <div id="modifierTasksPanel" class="modifier-sidebar">
                <div class="modifier-sidebar-header">
                    <h2>Tasks</h2>
                    <button id="modifierSidebarToggle" title="Toggle Task List">‹</button>
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

        // Basic display of request details
        // Headers are stored as JSON string, body as Base64 string
        let requestHeadersFormatted = '(No Headers)';
        if (task.base_request_headers) {
            try {
                const headersObj = JSON.parse(task.base_request_headers);
                requestHeadersFormatted = Object.entries(headersObj)
                    .map(([key, value]) => `${escapeHtml(key)}: ${escapeHtml(Array.isArray(value) ? value.join(', ') : value)}`)
                    .join('\n');
            } catch (e) {
                console.warn("Could not parse base_request_headers JSON:", e);
                requestHeadersFormatted = escapeHtml(task.base_request_headers); // Show raw if not parsable
            }
        }

        let requestBodyDecoded = '(Empty Body)';
        if (task.base_request_body) {
            try {
                requestBodyDecoded = atob(task.base_request_body);
                // Attempt to pretty-print if JSON
                try {
                    const jsonObj = JSON.parse(requestBodyDecoded);
                    requestBodyDecoded = JSON.stringify(jsonObj, null, 2);
                } catch (e) { /* Not JSON, or malformed, leave as decoded text */ }
            } catch (e) {
                console.warn("Could not decode base64 base_request_body:", e);
                requestBodyDecoded = 'Error decoding body (not valid Base64)';
            }
        }


        workspaceDiv.innerHTML = `
            <div class="modifier-task-header">
                <h2 id="modifierTaskNameDisplay" data-task-id="${task.id}">${escapeHtml(task.name || `Task ${task.id}`)}</h2>
                <div id="modifierTaskNameEditControls" style="display: inline-block; margin-left: 10px;">
                    <button id="editModifierTaskNameBtn" class="action-button inline-edit-button" title="Edit Task Name" style="margin-right: 5px;">✏️</button>
                    <button id="cloneModifierTaskBtn" class="action-button inline-edit-button" title="Clone Task">🐑</button> 
                </div>
                <div id="modifierTaskNameInputContainer" style="display: none; margin-bottom:10px;">
                    <input type="text" id="modifierTaskNameInput" class="modifier-input" value="${escapeHtmlAttribute(task.name || `Task ${task.id}`)}" style="width: 500px; margin-right: 5px;">
                    <button id="saveModifierTaskNameBtn" class="primary small-button">Save</button>
                    <button id="cancelModifierTaskNameBtn" class="secondary small-button">Cancel</button>
                </div>
            </div>
            <p><strong>Base Request:</strong></p>
            <div id="modifierRequestStatusMessage" class="status-message-area" style="margin-bottom: 10px; display: none;"></div>

            <div class="modifier-tabs">
                <button class="modifier-tab-button active" data-tab-id="modifierRequestTab">Request</button>
                <button class="modifier-tab-button" data-tab-id="modifierResponseTab">Response</button>
            </div>

            <div id="modifierRequestTab" class="modifier-tab-content active">
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
                        <button id="sendModifiedRequestBtn" class="primary">Send Request</button>
                    </div>
                </div>
            </div>

            <div id="modifierResponseTab" class="modifier-tab-content">
                <div class="response-section">
                    <h3>Response:</h3>
                    <div class="form-group">
                        <label for="modResponseStatus">Status:</label>
                        <input type="text" id="modResponseStatus" class="modifier-input" readonly placeholder="e.g., 200 OK">
                    </div>
                    <div class="form-group">
                        <label for="modResponseHeaders">Response Headers:</label>
                        <textarea id="modResponseHeaders" class="modifier-textarea" rows="8" readonly placeholder="Response headers will appear here..."></textarea>
                    </div>
                    <div class="form-group">
                        <label for="modResponseBody">Response Body:</label>
                        <textarea id="modResponseBody" class="modifier-textarea" rows="10" readonly placeholder="Response body will appear here..."></textarea>
                    </div>
                </div>
            </div>
        `;
        document.getElementById('sendModifiedRequestBtn')?.addEventListener('click', () => handleSendModifiedRequest(task.id));
        document.getElementById('editModifierTaskNameBtn')?.addEventListener('click', () => toggleTaskNameEdit(true, task.id, task.name));
        document.getElementById('saveModifierTaskNameBtn')?.addEventListener('click', () => handleSaveTaskName(task.id));
        document.getElementById('cancelModifierTaskNameBtn')?.addEventListener('click', () => toggleTaskNameEdit(false, task.id, task.name));
        document.getElementById('cloneModifierTaskBtn')?.addEventListener('click', () => handleCloneModifierTask(task.id));
        document.querySelectorAll('.modifier-tab-button').forEach(button => {
            button.addEventListener('click', () => setActiveModifierTab(button.dataset.tabId));
        });

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
    const mainPanel = document.getElementById('modifierMainPanel'); // Not directly resized, but good to have
    const toggleButton = document.getElementById('modifierSidebarToggle'); 

    // --- Resizing Logic ---
    let isResizing = false;
    const storedWidth = localStorage.getItem('modifierSidebarWidth');
    if (storedWidth) {
        tasksPanel.style.width = storedWidth;
    }

    if (resizer && tasksPanel && mainPanel) { // Ensure mainPanel is also available
        resizer.addEventListener('mousedown', (e) => {
            isResizing = true;
            document.body.style.cursor = 'col-resize'; // Change cursor for visual feedback
            document.body.style.userSelect = 'none'; // Prevent text selection during resize

            const startX = e.clientX;
            const startWidth = tasksPanel.offsetWidth;

            const doDrag = (moveEvent) => {
                if (!isResizing) return;
                const newWidth = startWidth + (moveEvent.clientX - startX);
                if (newWidth > 150 && newWidth < (window.innerWidth * 0.7)) { // Min and Max width constraints
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

    // --- Toggle Logic ---
    const storedCollapsed = localStorage.getItem('modifierSidebarCollapsed') === 'true';
    if (storedCollapsed) {
        tasksPanel.classList.add('collapsed');
        if (mainPanel) mainPanel.classList.add('sidebar-collapsed-sibling'); // Add class on load
        if (toggleButton) toggleButton.textContent = '›';
        if(resizer) resizer.style.display = 'none';
    }

    if (toggleButton && tasksPanel && mainPanel) { // Ensure mainPanel is available
        toggleButton.addEventListener('click', () => {
            const isCollapsed = tasksPanel.classList.toggle('collapsed');
            // Toggle a class on the main panel to adjust its padding
            mainPanel.classList.toggle('sidebar-collapsed-sibling', isCollapsed);

            toggleButton.textContent = isCollapsed ? '›' : '‹';
            localStorage.setItem('modifierSidebarCollapsed', isCollapsed);
            if(resizer) resizer.style.display = isCollapsed ? 'none' : 'flex'; // Use flex for resizer display
        });
    }
}

async function handleSendModifiedRequest(taskId) {
    const method = document.getElementById('modMethod').value;
    const url = document.getElementById('modURL').value;
    const headers = document.getElementById('modHeaders').value; // Will need parsing
    const body = document.getElementById('modBody').value; // Will need encoding if not plain text
    const requestStatusMessageEl = document.getElementById('modifierRequestStatusMessage');
    const sendButton = document.getElementById('sendModifiedRequestBtn');

    const responseStatusEl = document.getElementById('modResponseStatus');
    const responseHeadersEl = document.getElementById('modResponseHeaders');
    const responseBodyEl = document.getElementById('modResponseBody');

    // Clear previous response
    if(responseStatusEl) responseStatusEl.value = '';
    if(responseHeadersEl) responseHeadersEl.value = '';
    if(responseBodyEl) responseBodyEl.value = '';
    if(requestStatusMessageEl) {
        requestStatusMessageEl.textContent = '';
        requestStatusMessageEl.style.display = 'none';
    }

    if(sendButton) sendButton.disabled = true;
    if(requestStatusMessageEl) { requestStatusMessageEl.textContent = 'Sending...'; requestStatusMessageEl.className = 'status-message-area info'; requestStatusMessageEl.style.display = 'block'; }


    try {
        const responseData = await apiService.executeModifiedRequest({
            task_id: taskId, // Optional, for future versioning
            method: method,
            url: url,
            headers: headers, // Send as string, backend will parse
            body: body        // Send as plain text
        });

        if(responseStatusEl) responseStatusEl.value = responseData.status_text || responseData.status_code || 'N/A';
        
        if(responseHeadersEl) {
            let headersFormatted = '(No Headers)';
            if (responseData.headers) {
                headersFormatted = Object.entries(responseData.headers)
                    .map(([key, value]) => `${escapeHtml(key)}: ${escapeHtml(Array.isArray(value) ? value.join(', ') : value)}`)
                    .join('\n');
            }
            responseHeadersEl.value = headersFormatted;
        }

        if(responseBodyEl) {
            let bodyDecoded = '(Empty Body)';
            if (responseData.body) { // Assuming backend sends body base64 encoded
                try { bodyDecoded = atob(responseData.body); } catch (e) { bodyDecoded = "Error decoding body (not valid Base64)"; }
            }
            responseBodyEl.value = bodyDecoded;
        }
        if(requestStatusMessageEl) {
            requestStatusMessageEl.textContent = 'Response received successfully.';
            requestStatusMessageEl.className = 'status-message-area success';
            // Optionally hide after a delay: setTimeout(() => { requestStatusMessageEl.style.display = 'none'; }, 3000);
            setActiveModifierTab('modifierResponseTab'); // Switch to response tab
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
        nameDisplay.style.display = 'inline-block'; // Or 'block' depending on desired layout
        editControls.style.display = 'inline-block';
        inputContainer.style.display = 'none';
        // Optionally reset input value if needed, though it's re-set on edit start
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
        await fetchAndDisplayModifierTasks(); // Refresh task list on the left
        highlightModifierTaskInList(taskId); // Re-highlight
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

        await fetchAndDisplayModifierTasks(); // Refresh task list
        loadModifierTaskIntoWorkspace(clonedTask.id); // Load the new cloned task
        // highlightModifierTaskInList will be called by loadModifierTaskIntoWorkspace
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
        taskOrders[id] = index; // Backend expects map of { "taskID_string": order_int }
    });

    if (Object.keys(taskOrders).length === 0) return; // No tasks to order

    try {
        await apiService.updateModifierTasksOrder(taskOrders);
        uiService.showModalMessage("Success", "Task order saved.", true, 1500);
        // Optionally, re-fetch to confirm, though optimistic update is usually fine for order
        // await fetchAndDisplayModifierTasks();
    } catch (error) {
        console.error("Error saving task order:", error);
        uiService.showModalMessage("Error", `Failed to save task order: ${escapeHtml(error.message)}`);
    }
}