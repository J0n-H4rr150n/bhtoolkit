import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;

// DOM element references
let viewContentContainer;

// State specific to this view
let activeRecording = {
    pageId: null,
    targetId: null,
    startTimestamp: null, // JS timestamp (milliseconds)
    pageName: ''
};
let editingPageId = null; // To track which page item is currently being edited

/**
 * Initializes the Page Sitemap View module with necessary services.
 * @param {Object} services - An object containing service instances.
 */
export function initPageSitemapView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    console.log("[PageSitemapView] Initialized.");
}

/**
 * Loads the Page Sitemap view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadPageSitemapView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadPageSitemapView!");
        return;
    }

    if (!apiService || !uiService || !stateService) {
        console.error("PageSitemapView not initialized. Call initPageSitemapView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>PageSitemapView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const { currentTargetId, currentTargetName } = appState;

    let headerHTML = `<h1>Page Sitemap ${currentTargetId ? `for Target: ${escapeHtml(currentTargetName)} (ID: ${currentTargetId})` : '(No Target Selected)'}</h1>`;

    viewContentContainer.innerHTML = `
        ${headerHTML}
        <div id="pageSitemapControls" style="margin-bottom: 20px;">
            <button id="startPageRecordingBtn" class="primary" ${!currentTargetId ? 'disabled' : ''}>Start Recording Page</button>
            <button id="stopPageRecordingBtn" class="danger" style="display: none;">Stop Recording</button>
            <span id="activeRecordingStatus" style="margin-left: 15px; font-style: italic;"></span>
        </div>

        <div id="pageSitemapMessage" class="message-area" style="margin-bottom: 10px;"></div>

        <div class="page-sitemap-layout" style="display: flex; gap: 20px;">
            <div id="recordedPagesListContainer" style="flex: 1; max-width: 400px; border-right: 1px solid #ccc; padding-right: 20px;">
                <h2>Recorded Pages</h2>
                <div id="recordedPagesList">
                    <p>Loading recorded pages...</p>
                </div>
            </div>
            <div id="pageLogsContainer" style="flex: 2;">
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <h2>Logs for Selected Page</h2>
                    <button id="deleteSelectedPageBtn" class="action-button" title="Delete Selected Page" style="display: none; font-size: 1.2em;">🗑️</button>
                </div>
                <div id="selectedPageName" style="margin-bottom: 10px; font-weight: bold;"></div>
                <div id="pageLogsList">
                    <p>Select a page from the list to view its associated HTTP logs.</p>
                </div>
            </div>
        </div>
    `;

    const startRecordingBtn = document.getElementById('startPageRecordingBtn');
    const stopRecordingBtn = document.getElementById('stopPageRecordingBtn');
    // const activeRecordingStatusEl = document.getElementById('activeRecordingStatus'); // We'll use updateRecordingStatusUI

    if (startRecordingBtn) {
        startRecordingBtn.addEventListener('click', handleStartPageRecording);
    }
    if (stopRecordingBtn) {
        stopRecordingBtn.addEventListener('click', handleStopPageRecording);
    }

    if (currentTargetId) {
        if (!activeRecording.pageId || activeRecording.targetId !== currentTargetId) {
            fetchAndDisplayRecordedPages(currentTargetId);
        }
        updateRecordingStatusUI(); // Update UI based on any potentially active recording state
    } else {
        document.getElementById('recordedPagesList').innerHTML = '<p>Please select a target to manage page sitemaps.</p>';
    }
}

async function fetchAndDisplayRecordedPages(targetId) {
    const recordedPagesListDiv = document.getElementById('recordedPagesList');
    if (!recordedPagesListDiv) return;

    recordedPagesListDiv.innerHTML = '<p>Loading recorded pages...</p>';
    try {
        const pages = await apiService.getPagesForTarget(targetId);
        if (pages && pages.length > 0) {
            let listHTML = '<ul id="pageSitemapSortableList">'; // Add an ID for the sortable list
            pages.forEach(page => {
                const description = page.description?.Valid ? page.description.String : '';
                const endTime = page.end_timestamp?.Valid ? new Date(page.end_timestamp.Time).toLocaleString() : 'Still Recording...';
                listHTML += `
                    <li draggable="true" class="page-sitemap-item" data-page-id="${page.id}" data-page-name="${escapeHtmlAttribute(page.name)}" 
                        title="Started: ${new Date(page.start_timestamp).toLocaleString()}\nEnded: ${endTime}\nDescription: ${escapeHtmlAttribute(description)}">
                        <span class="page-name-display">${escapeHtml(page.name)}</span>
                        <input type="text" class="page-name-edit-input" value="${escapeHtmlAttribute(page.name)}" style="display:none; width: 70%;">
                        <button class="action-button edit-page-name-btn" title="Edit Page Name" style="margin-left: 5px;">✏️</button>
                        <br><small>ID: ${page.id} | Started: ${new Date(page.start_timestamp).toLocaleDateString()}</small>
                    </li>`;
            });
            listHTML += '</ul>';
            recordedPagesListDiv.innerHTML = listHTML;

            const sortableList = document.getElementById('pageSitemapSortableList');

            sortableList.querySelectorAll('.page-sitemap-item').forEach(item => {
                item.addEventListener('click', (e) => {
                    // Prevent click from firing if a drag operation just ended on this element
                    if (item.classList.contains('no-click-after-drag')) {
                        item.classList.remove('no-click-after-drag');
                        return;
                    }
                    const pageId = e.currentTarget.dataset.pageId;
                    const pageName = e.currentTarget.dataset.pageName;
                    // Highlight selected item
                    sortableList.querySelectorAll('.page-sitemap-item').forEach(li => li.classList.remove('active'));
                    e.currentTarget.classList.add('active');
                    document.getElementById('deleteSelectedPageBtn').style.display = 'inline-block'; // Show delete button
                    fetchAndDisplayLogsForPage(pageId, pageName);
                });
                item.addEventListener('dragstart', handlePageDragStart);
                item.addEventListener('dragend', handlePageDragEnd);

                const editBtn = item.querySelector('.edit-page-name-btn');
                if (editBtn) {
                    editBtn.addEventListener('click', (e) => handleEditPageName(e, item));
                }
            });
            sortableList.addEventListener('dragover', handlePageDragOver);
            sortableList.addEventListener('drop', handlePageDrop);
        } else {
            recordedPagesListDiv.innerHTML = '<p>No pages recorded for this target yet.</p>';
        }
        // Ensure delete button is hidden if no page is selected initially or list is empty
        if (!recordedPagesListDiv.querySelector('.page-sitemap-item.active')) {
            document.getElementById('deleteSelectedPageBtn').style.display = 'none';
        }
    } catch (error) {
        console.error("Error fetching recorded pages:", error);
        recordedPagesListDiv.innerHTML = `<p class="error-message">Error loading recorded pages: ${escapeHtml(error.message)}</p>`;
    }
}

async function handleStartPageRecording() {
    const appState = stateService.getState();
    if (!appState.currentTargetId) {
        uiService.showModalMessage("Error", "A target must be selected to start recording a page.");
        return;
    }

    if (activeRecording.pageId) {
        uiService.showModalMessage("Info", `A page recording ("${escapeHtml(activeRecording.pageName)}") is already in progress.`);
        return;
    }

    // Use a custom modal structure for input fields
    const modalContentHTML = `
        <div class="form-group">
            <label for="pageSitemapNameInput">Page Name (Required):</label>
            <input type="text" id="pageSitemapNameInput" class="modifier-input" style="width:100%;" required>
        </div>
        <div class="form-group">
            <label for="pageSitemapDescInput">Description (Optional):</label>
            <textarea id="pageSitemapDescInput" class="modifier-textarea" rows="3" style="width:100%;"></textarea>
        </div>
    `;

    uiService.showModalConfirm("Start New Page Recording", modalContentHTML, async () => {
        const pageNameInput = document.getElementById('pageSitemapNameInput');
        const pageDescInput = document.getElementById('pageSitemapDescInput');

        const pageName = pageNameInput.value.trim();
        const pageDescription = pageDescInput.value.trim();

        if (!pageName) {
            uiService.showModalMessage("Validation Error", "Page Name is required.", true, 2000, true); // isTemp, autoClose, isError
            // Re-show confirm modal or handle error better
            return false; // Prevent modal from closing if validation fails
        }

        try {
            const pageData = {
                target_id: appState.currentTargetId,
                name: pageName,
                description: pageDescription,
            };
            const createdPage = await apiService.createPageSitemapEntry(pageData);
            
            activeRecording.pageId = createdPage.id;
            activeRecording.targetId = createdPage.target_id;
            activeRecording.startTimestamp = new Date(createdPage.start_timestamp).getTime(); // Use backend's start_timestamp
            activeRecording.pageName = createdPage.name;

            updateRecordingStatusUI();
            uiService.showModalMessage("Recording Started", `Page recording for "${escapeHtml(createdPage.name)}" has started.`, true, 2000);
        } catch (error) {
            console.error("Error starting page recording:", error);
            uiService.showModalMessage("Error", `Failed to start page recording: ${escapeHtml(error.message)}`);
        }
    }, () => { /* User cancelled */ }, "Start Recording", "Cancel", true); // Pass true for customContent
}

async function handleStopPageRecording() {
    if (!activeRecording.pageId) {
        uiService.showModalMessage("Info", "No active page recording to stop.");
        return;
    }

    const { pageId, targetId, startTimestamp, pageName } = activeRecording;

    try {
        const stopData = {
            page_id: pageId,
            target_id: targetId,
            start_timestamp: Math.floor(startTimestamp / 1000) // Backend expects Unix timestamp in seconds
        };
        const response = await apiService.stopPageSitemapRecording(stopData);

        uiService.showModalMessage("Recording Stopped", `Page recording for "${escapeHtml(pageName)}" stopped. ${response.logs_associated} logs associated.`, true, 3000);

        // Clear active recording state
        activeRecording.pageId = null;
        activeRecording.targetId = null;
        activeRecording.startTimestamp = null;
        activeRecording.pageName = '';

        updateRecordingStatusUI();
        // Refresh the list of pages for the current target
        const appState = stateService.getState();
        if (appState.currentTargetId) {
            fetchAndDisplayRecordedPages(appState.currentTargetId);
        }
        // Clear selected page details and hide delete button
        document.getElementById('selectedPageName').textContent = '';
        document.getElementById('pageLogsList').innerHTML = '<p>Select a page from the list to view its associated HTTP logs.</p>';
        document.getElementById('deleteSelectedPageBtn').style.display = 'none';
    } catch (error) {
        console.error("Error stopping page recording:", error);
        uiService.showModalMessage("Error", `Failed to stop page recording: ${escapeHtml(error.message)}`);
    }
}

function updateRecordingStatusUI() {
    const startBtn = document.getElementById('startPageRecordingBtn');
    const stopBtn = document.getElementById('stopPageRecordingBtn');
    const statusEl = document.getElementById('activeRecordingStatus');

    if (!startBtn || !stopBtn || !statusEl) return;

    if (activeRecording.pageId) {
        startBtn.style.display = 'none';
        stopBtn.style.display = 'inline-block';
        statusEl.textContent = `Recording: "${escapeHtml(activeRecording.pageName)}" (Target: ${activeRecording.targetId})...`;
    } else {
        startBtn.style.display = 'inline-block';
        stopBtn.style.display = 'none';
        statusEl.textContent = '';
    }
}

async function fetchAndDisplayLogsForPage(pageId, pageName) {
    const selectedPageNameDiv = document.getElementById('selectedPageName');
    const pageLogsListDiv = document.getElementById('pageLogsList');

    if (!selectedPageNameDiv || !pageLogsListDiv) {
        console.error("Required divs for displaying page logs not found.");
        return;
    }

    selectedPageNameDiv.textContent = `Displaying logs for: ${escapeHtml(pageName)} (ID: ${pageId})`;
    pageLogsListDiv.innerHTML = `<p>Loading logs for page ID ${pageId}...</p>`;

    try {
        const logs = await apiService.getLogsForPageSitemapEntry(pageId);
        if (logs && logs.length > 0) {
            // For now, a simple table. Could be enhanced with sorting, filtering, resizable columns later.
            let tableHTML = `
                <table class="page-sitemap-logs-table">
                    <thead>
                        <tr>
                            <th>#</th>
                            <th>Timestamp</th>
                            <th>Method</th>
                            <th>URL</th>
                            <th>Status</th>
                            <th>Size (B)</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>`;
            logs.forEach((log, index) => {
                const ts = log.timestamp ? new Date(log.timestamp).toLocaleString() : 'N/A';
                const requestMethod = log.request_method && log.request_method.Valid ? log.request_method.String : 'N/A';
                const requestURL = log.request_url && log.request_url.Valid ? log.request_url.String : 'N/A';
                tableHTML += `
                    <tr>
                        <td>${index + 1}</td>
                        <td>${ts}</td>
                        <td>${escapeHtml(requestMethod)}</td>
                        <td class="proxy-log-url-cell" title="${escapeHtmlAttribute(requestURL)}">${escapeHtml(requestURL)}</td>
                        <td>${log.response_status_code || '-'}</td>
                        <td>${log.response_body_size || 0}</td>
                        <td class="actions-cell">
                            <span class="favorite-toggle page-sitemap-log-favorite-toggle ${log.is_favorite ? 'favorited' : ''}" data-log-id="${log.id}" data-is-favorite="${log.is_favorite ? 'true' : 'false'}" title="Toggle Favorite" style="cursor: pointer; margin-right: 8px; font-size: 1.2em; vertical-align: middle;">${log.is_favorite ? '★' : '☆'}</span>
                            <button class="action-button view-log-detail" data-log-id="${log.id}" title="View Full Log Detail">👁️</button>
                        </td>
                    </tr>`;
            });
            tableHTML += `</tbody></table>`;
            pageLogsListDiv.innerHTML = tableHTML;

            pageLogsListDiv.querySelectorAll('.view-log-detail').forEach(button => {
                button.addEventListener('click', (e) => {
                    const logId = e.currentTarget.dataset.logId;
                    if (logId) {
                        const detailHashPath = `#proxy-log-detail?id=${logId}`;
                        if (e.ctrlKey || e.metaKey) { // Check for Ctrl (Windows/Linux) or Command (Mac) key
                            e.preventDefault(); // Prevent default click behavior if modifier is pressed
                            const baseUrl = window.location.origin + window.location.pathname.replace(/\/$/, '');
                            const fullUrl = baseUrl + detailHashPath;
                            window.open(fullUrl, '_blank'); // Open in new tab
                        } else {
                            // Default action: navigate in the current tab using hash change
                            window.location.hash = detailHashPath;
                        }
                    }
                });
            });

            pageLogsListDiv.querySelectorAll('.page-sitemap-log-favorite-toggle').forEach(starBtn => {
                starBtn.addEventListener('click', handlePageSitemapLogFavoriteToggle);
            });
        } else {
            pageLogsListDiv.innerHTML = '<p>No HTTP logs associated with this page recording.</p>';
        }
    } catch (error) {
        console.error(`Error fetching logs for page ID ${pageId}:`, error);
        pageLogsListDiv.innerHTML = `<p class="error-message">Error loading logs: ${escapeHtml(error.message)}</p>`;
    }
}

document.addEventListener('DOMContentLoaded', () => {
    // Delegate event listener for delete button if it's added dynamically or ensure it's always in DOM
    // For now, we'll add it directly if the button is part of the initial loadPageSitemapView HTML
});

// --- Drag and Drop for Page Sitemap List ---
let draggedPageItem = null;

function handlePageDragStart(event) {
    draggedPageItem = event.target;
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', event.target.dataset.pageId);
    setTimeout(() => {
        event.target.classList.add('dragging');
    }, 0);
}

function handlePageDragEnd(event) {
    if (draggedPageItem) { // Check if draggedPageItem is not null
        draggedPageItem.classList.remove('dragging');
        // Add a temporary class to prevent click event right after drag
        draggedPageItem.classList.add('no-click-after-drag');
        setTimeout(() => {
            if (draggedPageItem) draggedPageItem.classList.remove('no-click-after-drag');
        }, 100); // Short delay to absorb potential click
    }
    draggedPageItem = null;
    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());
}

function handlePageDragOver(event) {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    const container = event.currentTarget; // The UL
    const afterElement = getPageDragAfterElement(container, event.clientY);

    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());

    if (draggedPageItem && draggedPageItem !== afterElement && (!afterElement || draggedPageItem !== afterElement.previousSibling)) {
        const indicator = document.createElement('div');
        indicator.classList.add('drop-indicator'); // Use same class as modifier for now
        if (afterElement == null) {
            container.appendChild(indicator);
        } else {
            container.insertBefore(indicator, afterElement);
        }
    }
}

function getPageDragAfterElement(container, y) {
    const draggableElements = [...container.querySelectorAll('li.page-sitemap-item:not(.dragging)')];
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

async function handlePageDrop(event) {
    event.preventDefault();
    document.querySelectorAll('.drop-indicator').forEach(ind => ind.remove());
    if (!draggedPageItem) return;

    const container = event.currentTarget; // The UL
    const afterElement = getPageDragAfterElement(container, event.clientY);

    if (afterElement == null) {
        container.appendChild(draggedPageItem);
    } else {
        container.insertBefore(draggedPageItem, afterElement);
    }
    await savePageSitemapOrder();
}

async function savePageSitemapOrder() {
    const pageList = document.getElementById('pageSitemapSortableList');
    if (!pageList) return;

    const orderedPageIds = [...pageList.querySelectorAll('li.page-sitemap-item')].map(item => item.dataset.pageId);
    const pageOrders = {};
    orderedPageIds.forEach((id, index) => {
        pageOrders[id] = index; // Backend expects map of { "pageID_string": order_int }
    });

    if (Object.keys(pageOrders).length === 0) return;

    try {
        await apiService.updatePageSitemapOrder(pageOrders);
        // uiService.showModalMessage("Success", "Page order saved.", true, 1500); // Removed success popup
    } catch (error) {
        console.error("Error saving page order:", error);
        uiService.showModalMessage("Error", `Failed to save page order: ${escapeHtml(error.message)}`);
        // Optionally, re-fetch to revert to previous order if save fails
        // const appState = stateService.getState();
        // if (appState.currentTargetId) fetchAndDisplayRecordedPages(appState.currentTargetId);
    }
}

async function handleDeleteSelectedPage() {
    const selectedPageItem = document.querySelector('#recordedPagesList .page-sitemap-item.active');
    if (!selectedPageItem) {
        uiService.showModalMessage("Info", "No page selected to delete.");
        return;
    }

    const pageId = selectedPageItem.dataset.pageId;
    const pageName = selectedPageItem.dataset.pageName;

    uiService.showModalConfirm(
        "Confirm Delete Page",
        `Are you sure you want to delete the recorded page "${escapeHtml(pageName)}" (ID: ${pageId}) and all its associated logs? This action cannot be undone.`,
        async () => {
            try {
                await apiService.deletePageSitemapEntry(pageId); // Uncommented and using the actual API call
                uiService.showModalMessage("Success", `Page "${escapeHtml(pageName)}" deleted successfully.`, true, 2000);

                // Refresh list and clear details
                const appState = stateService.getState();
                if (appState.currentTargetId) {
                    fetchAndDisplayRecordedPages(appState.currentTargetId);
                }
                document.getElementById('selectedPageName').textContent = '';
                document.getElementById('pageLogsList').innerHTML = '<p>Select a page from the list to view its associated HTTP logs.</p>';
                document.getElementById('deleteSelectedPageBtn').style.display = 'none';
            } catch (error) {
                console.error("Error deleting page:", error);
                uiService.showModalMessage("Error", `Failed to delete page: ${escapeHtml(error.message)}`);
            }
        }
    );
}

// Add event listener for the delete button in loadPageSitemapView or ensure it's always present
// For simplicity, if it's always in the initial HTML structure of the view:
document.addEventListener('click', function(event) {
    if (event.target && event.target.id === 'deleteSelectedPageBtn') {
        handleDeleteSelectedPage();
    }
});

function handleEditPageName(event, listItem) {
    event.stopPropagation(); // Prevent the li click handler from firing
    const pageId = listItem.dataset.pageId;

    if (editingPageId === pageId) return; // Already editing this item
    if (editingPageId) { // If another item is being edited, cancel it
        cancelPageNameEdit(editingPageId);
    }
    editingPageId = pageId;

    const nameDisplay = listItem.querySelector('.page-name-display');
    const nameInput = listItem.querySelector('.page-name-edit-input');
    const editButton = listItem.querySelector('.edit-page-name-btn');

    nameDisplay.style.display = 'none';
    nameInput.style.display = 'inline-block';
    nameInput.value = nameDisplay.textContent; // Ensure it has the current text
    nameInput.focus();
    nameInput.select();

    editButton.style.display = 'none'; // Hide pencil

    // Add Save and Cancel buttons
    const saveBtn = document.createElement('button');
    saveBtn.className = 'action-button save-page-name-btn';
    saveBtn.title = 'Save Name';
    saveBtn.innerHTML = '✔️';
    saveBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        handleSavePageNameEdit(pageId, listItem);
    });

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'action-button cancel-page-name-btn';
    cancelBtn.title = 'Cancel Edit';
    cancelBtn.innerHTML = '❌';
    cancelBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        cancelPageNameEdit(pageId);
    });

    // Insert buttons after the input field
    nameInput.parentNode.insertBefore(saveBtn, nameInput.nextSibling);
    nameInput.parentNode.insertBefore(cancelBtn, saveBtn.nextSibling);

    nameInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            handleSavePageNameEdit(pageId, listItem);
        } else if (e.key === 'Escape') {
            cancelPageNameEdit(pageId);
        }
    });
}

function cancelPageNameEdit(pageId) {
    const listItem = document.querySelector(`.page-sitemap-item[data-page-id="${pageId}"]`);
    if (!listItem) return;

    const nameDisplay = listItem.querySelector('.page-name-display');
    const nameInput = listItem.querySelector('.page-name-edit-input');
    const editButton = listItem.querySelector('.edit-page-name-btn');

    nameDisplay.style.display = 'inline';
    nameInput.style.display = 'none';
    if(editButton) editButton.style.display = 'inline-block';

    listItem.querySelector('.save-page-name-btn')?.remove();
    listItem.querySelector('.cancel-page-name-btn')?.remove();
    editingPageId = null;
}

async function handleSavePageNameEdit(pageId, listItem) {
    const nameInput = listItem.querySelector('.page-name-edit-input');
    const newName = nameInput.value.trim();

    if (!newName) {
        uiService.showModalMessage("Validation Error", "Page name cannot be empty.", true, 2000, true);
        nameInput.focus();
        return;
    }

    try {
        // Call the apiService function to update the page details
        const updatedPage = await apiService.updatePageSitemapEntryDetails(pageId, { name: newName });

        // Update the UI with the (potentially sanitized or confirmed) name from the backend
        listItem.querySelector('.page-name-display').textContent = escapeHtml(updatedPage.name || newName);
        listItem.dataset.pageName = escapeHtmlAttribute(updatedPage.name || newName); // Update data attribute

        cancelPageNameEdit(pageId); // Revert UI to display mode
        uiService.showModalMessage("Success", `Page name updated to "${escapeHtml(updatedPage.name || newName)}".`, true, 2000);

    } catch (error) {
        console.error(`Error updating page name for ID ${pageId}:`, error);
        uiService.showModalMessage("Error", `Failed to update page name: ${escapeHtml(error.message)}`);
        // Optionally, you might want to leave the input field open or revert to original name if save fails
        // For now, it will just show an error and the user can try again or cancel.
    }
}

async function handlePageSitemapLogFavoriteToggle(event) {
    const button = event.currentTarget; // It's a span, but acts like a button
    const logId = button.getAttribute('data-log-id');
    const isCurrentlyFavorite = button.getAttribute('data-is-favorite') === 'true';
    const newFavoriteState = !isCurrentlyFavorite;

    try {
        await apiService.setProxyLogFavorite(logId, newFavoriteState); // Use existing API service function
        button.innerHTML = newFavoriteState ? '★' : '☆';
        button.classList.toggle('favorited', newFavoriteState);
        button.setAttribute('data-is-favorite', newFavoriteState.toString());
        // Optionally, show a small success message or just rely on the visual change
    } catch (favError) {
        console.error("Error toggling favorite from Page Sitemap:", favError);
        uiService.showModalMessage("Error", `Failed to update favorite status for log ${logId}: ${favError.message}`);
        // Revert UI if backend call failed (optional, but good for UX)
    }
}