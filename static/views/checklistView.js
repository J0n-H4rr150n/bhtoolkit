import { escapeHtml, escapeHtmlAttribute, copyToClipboard, debounce } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService;

// State specific to checklist view
let isAddingNewChecklistItem = false;
let currentlyEditingChecklistItemId = null;
const NOTES_TRUNCATE_LENGTH = 100; // Max characters for notes in table

// State variables will now be primarily read from stateService for pagination, sort, filter

/**
 * Initializes the Checklist View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService, tableService.
 */
export function initChecklistView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService;
    console.log("[ChecklistView] Initialized.");
}

// --- Target Checklist Specific Functions ---

function updateChecklistSummary() {
    const summaryDisplay = document.getElementById('checklistSummaryDisplay');
    const tableBody = document.getElementById('checklistTableBody');
    const { currentPage, totalPages, totalRecords, filterQuery } = stateService.getState().paginationState.targetChecklistItems;

    // Get the count of completed items for the current filter from the state
    const totalCompletedRecordsForFilter = stateService.getState().paginationState.targetChecklistItems.totalCompletedRecordsForFilter || 0;

    if (!summaryDisplay || !tableBody) {
        // If summaryDisplay exists but tableBody doesn't (e.g., during initial load error), clear summary
        if (summaryDisplay) {
            summaryDisplay.innerHTML = '<p>No items to display or error loading data.</p>';
            summaryDisplay.style.display = 'block';
        }
        return;
    }

    const itemsOnPage = tableBody.querySelectorAll('tr[data-item-id]').length;

    let completionPercentage = 0;
    if (totalRecords > 0) {
        completionPercentage = ((totalCompletedRecordsForFilter / totalRecords) * 100).toFixed(1);
    }

    if (totalRecords > 0) {
        summaryDisplay.innerHTML = `<p>Displaying ${itemsOnPage} of ${totalRecords} items. Page ${currentPage} of ${totalPages}. (${completionPercentage}% complete)</p>`;
        summaryDisplay.style.display = 'block';
    } else if (filterQuery) {
        summaryDisplay.innerHTML = '<p>No items match the current filter.</p>';
        summaryDisplay.style.display = 'block';
    } else {
        summaryDisplay.innerHTML = '';
        summaryDisplay.style.display = 'none';
    }
}

function initiateAddNewChecklistItemRow(targetId) {
    if (isAddingNewChecklistItem) {
        uiService.showModalMessage("Info", "Please save or cancel the current new item before adding another.");
        return;
    }
    isAddingNewChecklistItem = true;

    const tableBody = document.getElementById('checklistTableBody');
    if (!tableBody) {
        console.error("Checklist table body not found.");
        isAddingNewChecklistItem = false;
        return;
    }

    const newRow = tableBody.insertRow(0);
    newRow.id = 'newChecklistItemRow';
    newRow.innerHTML = `
        <td>*</td>
        <td><input type="checkbox" disabled></td>
        <td><input type="text" class="new-checklist-item-text" placeholder="New item text..." style="width:100%;"></td>
        <td><input type="text" class="new-checklist-item-command-text" placeholder="Optional command..." style="width:100%;"></td>
        <td><textarea class="new-checklist-item-notes" placeholder="Notes..." style="width:100%; min-height: 40px;"></textarea></td>
        <td>
            <button class="action-button save-new-checklist-item" title="Save New Item">✔️</button>
            <button class="action-button cancel-new-checklist-item" title="Cancel Add Item">❌</button>
        </td>
    `;

    newRow.querySelector('.save-new-checklist-item').addEventListener('click', () => handleSaveNewChecklistItemRow(targetId));
    newRow.querySelector('.cancel-new-checklist-item').addEventListener('click', handleCancelNewChecklistItemRow);
    newRow.querySelector('.new-checklist-item-text').focus();

    ['.new-checklist-item-text', '.new-checklist-item-command-text', '.new-checklist-item-notes'].forEach(selector => {
        newRow.querySelector(selector).addEventListener('keydown', function(event) {
            if (event.key === 'Enter') {
                if (this.tagName === 'TEXTAREA' && event.shiftKey) return; // Allow Shift+Enter in textarea
                event.preventDefault();
                newRow.querySelector('.save-new-checklist-item').click();
            } else if (event.key === 'Escape') {
                handleCancelNewChecklistItemRow();
            }
        });
    });

    const addButton = document.getElementById('initiateAddChecklistItemBtn');
    if (addButton) addButton.disabled = true;
}

function handleCancelNewChecklistItemRow() {
    const newRow = document.getElementById('newChecklistItemRow');
    if (newRow) newRow.remove();
    isAddingNewChecklistItem = false;
    const addButton = document.getElementById('initiateAddChecklistItemBtn');
    if (addButton) addButton.disabled = false;
    const messageArea = document.getElementById('addChecklistItemMessage');
    if (messageArea) messageArea.textContent = '';
    updateChecklistSummary();
}

async function handleSaveNewChecklistItemRow(targetId) {
    const newRow = document.getElementById('newChecklistItemRow');
    if (!newRow) return;

    const itemTextInput = newRow.querySelector('.new-checklist-item-text');
    const notesTextarea = newRow.querySelector('.new-checklist-item-notes');
    const commandTextInput = newRow.querySelector('.new-checklist-item-command-text');
    const messageArea = document.getElementById('addChecklistItemMessage');

    if (messageArea) {
        messageArea.textContent = '';
        messageArea.className = 'message-area';
    }

    const itemText = itemTextInput.value.trim();
    const commandText = commandTextInput.value.trim();
    const notes = notesTextarea.value.trim();

    if (!itemText) {
        if (messageArea) {
            messageArea.textContent = 'Item text cannot be empty.';
            messageArea.classList.add('error-message');
        }
        itemTextInput.focus();
        return;
    }

    const payload = {
        target_id: parseInt(targetId, 10),
        item_text: itemText,
        item_command_text: commandText || null,
        notes: notes || null,
        is_completed: false
    };

    try {
        await apiService.addChecklistItem(payload); // newItem is returned but not used here
        if (messageArea) {
            messageArea.textContent = 'Checklist item added successfully!';
            messageArea.classList.add('success-message');
            setTimeout(() => { if(messageArea) messageArea.textContent = ''; }, 3000);
        }
        newRow.remove();
        isAddingNewChecklistItem = false;
        const addButton = document.getElementById('initiateAddChecklistItemBtn');
        if (addButton) addButton.disabled = false;
        fetchAndDisplayChecklistItems(targetId); // Refresh the list
    } catch (error) {
        if (messageArea) {
            messageArea.textContent = `Error adding item: ${escapeHtml(error.message)}`;
            messageArea.classList.add('error-message');
        }
    }
}

function handleEditChecklistItem(event) {
    const button = event.target.closest('button');
    const itemId = button.getAttribute('data-item-id');
    const itemRow = button.closest('tr');

    if (!itemRow || currentlyEditingChecklistItemId === itemId) return;

    if (isAddingNewChecklistItem) {
        uiService.showModalMessage("Info", "Please save or cancel the new item before editing another.");
        return;
    }

    cancelActiveChecklistItemEdit();
    currentlyEditingChecklistItemId = itemId;

    const textCell = itemRow.querySelector('.checklist-item-text');
    const notesCell = itemRow.querySelector('.checklist-item-notes');
    const commandCell = itemRow.querySelector('.checklist-item-command-text');
    const actionsCell = itemRow.querySelector('td:last-child');

    const currentText = textCell.textContent;
    const currentNotes = notesCell.textContent; // Direct text content for notes
    const currentCommand = commandCell.querySelector('pre')?.textContent || '';

    textCell.setAttribute('data-original-content', textCell.innerHTML);
    notesCell.setAttribute('data-original-content', notesCell.innerHTML);
    commandCell.setAttribute('data-original-content', commandCell.innerHTML);
    actionsCell.setAttribute('data-original-content', actionsCell.innerHTML);

    textCell.innerHTML = `<input type="text" class="edit-checklist-item-text-input" value="${escapeHtmlAttribute(currentText)}" style="width: 100%;">`;
    notesCell.innerHTML = `<textarea class="edit-checklist-item-notes-input" style="width: 100%; min-height: 40px;">${escapeHtml(currentNotes)}</textarea>`;
    commandCell.innerHTML = `<input type="text" class="edit-checklist-item-command-input" value="${escapeHtmlAttribute(currentCommand)}" style="width: 100%;">`;

    actionsCell.innerHTML = `
        <button class="action-button save-checklist-item-edit" data-item-id="${itemId}" title="Save Changes">✔️</button>
        <button class="action-button cancel-checklist-item-edit" data-item-id="${itemId}" title="Cancel Edit">❌</button>
    `;

    actionsCell.querySelector('.save-checklist-item-edit').addEventListener('click', handleSaveChecklistItemEdit);
    actionsCell.querySelector('.cancel-checklist-item-edit').addEventListener('click', cancelActiveChecklistItemEdit);

    itemRow.querySelector('.edit-checklist-item-text-input')?.focus();

    [
        itemRow.querySelector('.edit-checklist-item-text-input'),
        itemRow.querySelector('.edit-checklist-item-command-input'),
        itemRow.querySelector('.edit-checklist-item-notes-input')
    ].forEach(inputField => {
        inputField?.addEventListener('keydown', function(event) {
            if (event.key === 'Enter') {
                 if (this.tagName === 'TEXTAREA' && event.shiftKey) return; // Allow Shift+Enter in textarea
                event.preventDefault();
                actionsCell.querySelector('.save-checklist-item-edit').click();
            } else if (event.key === 'Escape') {
                cancelActiveChecklistItemEdit();
            }
        });
    });
}

async function handleSaveChecklistItemEdit(event) {
    const button = event.target.closest('button');
    const itemId = button.getAttribute('data-item-id');
    const itemRow = document.querySelector(`tr[data-item-id="${itemId}"]`);
    if (!itemRow) return;

    const textInput = itemRow.querySelector('.edit-checklist-item-text-input');
    const notesTextarea = itemRow.querySelector('.edit-checklist-item-notes-input');
    const commandInput = itemRow.querySelector('.edit-checklist-item-command-input');
    const isCompletedCheckbox = itemRow.querySelector('.checklist-item-complete');

    const newItemText = textInput.value.trim();
    const newNotes = notesTextarea.value.trim();
    const newCommand = commandInput.value.trim();
    const isCompleted = isCompletedCheckbox ? isCompletedCheckbox.checked : false;

    if (!newItemText) {
        uiService.showModalMessage('Error', 'Item text cannot be empty.');
        return;
    }

    const payload = {
        item_text: newItemText,
        notes: newNotes || null,
        item_command_text: newCommand || null,
        is_completed: isCompleted
    };

    try {
        const updatedItem = await apiService.updateChecklistItem(itemId, payload);
        restoreChecklistItemRow(itemId, updatedItem);
        uiService.showModalMessage('Success', `Checklist item ID ${itemId} updated.`);
    } catch (error) {
        uiService.showModalMessage('Error', `Error updating item ${itemId}: ${escapeHtml(error.message)}`);
        cancelActiveChecklistItemEdit(); // Restore to original on error
    }
}

async function handleToggleChecklistItemComplete(event) {
    const checkbox = event.target;
    const itemId = checkbox.getAttribute('data-item-id');
    const isCompleted = checkbox.checked;
    const row = checkbox.closest('tr');

    if (row) row.classList.toggle('completed-item', isCompleted);

    try {
        // Fetch current data from attributes to ensure we have the latest text if it was just edited (though edit mode handles its own save)
        const itemText = row.getAttribute('data-item-text') || row.querySelector('.checklist-item-text').textContent;
        const itemCommandText = row.getAttribute('data-item-command-text') || row.querySelector('.checklist-item-command-text pre')?.textContent || '';
        const notes = row.getAttribute('data-item-notes') || row.querySelector('.checklist-item-notes').textContent;

        const payload = {
            item_text: itemText,
            item_command_text: itemCommandText,
            notes: notes,
            is_completed: isCompleted
        };
        await apiService.updateChecklistItem(itemId, payload);
    } catch (error) {
        if (row) row.classList.toggle('completed-item', !isCompleted); // Revert UI on error
        checkbox.checked = !isCompleted;
        uiService.showModalMessage('Error', `Failed to update item ${itemId} completion: ${escapeHtml(error.message)}`);
    }
    updateChecklistSummary();
}

function restoreChecklistItemRow(itemId, updatedItemData = null) {
    const itemRow = document.querySelector(`tr[data-item-id="${itemId}"]`);
    if (!itemRow) return;

    const textCell = itemRow.querySelector('.checklist-item-text');
    const notesCell = itemRow.querySelector('.checklist-item-notes');
    const commandCell = itemRow.querySelector('.checklist-item-command-text');
    const actionsCell = itemRow.querySelector('td:last-child');
    const completeCheckbox = itemRow.querySelector('.checklist-item-complete');

    if (updatedItemData) {
        const safeText = escapeHtml(updatedItemData.item_text);
        const safeCommand = escapeHtml(updatedItemData.item_command_text?.String || '');
        const safeNotes = escapeHtml(updatedItemData.notes?.String || '');
        if (textCell) textCell.textContent = safeText;
        if (commandCell) commandCell.innerHTML = `<pre class="command-text">${safeCommand}</pre>`;
        if (notesCell) notesCell.textContent = safeNotes;
        if (completeCheckbox) completeCheckbox.checked = updatedItemData.is_completed;
        itemRow.classList.toggle('completed-item', updatedItemData.is_completed);
        // Update data attributes
        itemRow.setAttribute('data-item-text', updatedItemData.item_text);
        itemRow.setAttribute('data-item-command-text', updatedItemData.item_command_text?.String || '');
        itemRow.setAttribute('data-item-notes', updatedItemData.notes?.String || '');

        if (actionsCell) {
            const itemIdAttr = updatedItemData.id;
            const commandForButton = updatedItemData.item_command_text?.String || '';
            actionsCell.innerHTML = `
                <button class="action-button view-checklist-item-details" data-item-id="${itemIdAttr}" title="View Details">👁️</button>
                <button class="action-button edit-checklist-item" data-item-id="${itemIdAttr}" title="Edit Item">✏️</button>
                ${commandForButton ? `<button class="action-button copy-checklist-command" data-command="${escapeHtmlAttribute(commandForButton)}" title="Copy Command">📋</button>` : ''}
                <button class="action-button delete-checklist-item" data-item-id="${itemIdAttr}" title="Delete Item">🗑️</button>
            `;
            attachChecklistItemActionListenersForRow(itemRow);
        }
    } else { // Restore from original content if no new data (cancellation)
        if (textCell && textCell.hasAttribute('data-original-content')) textCell.innerHTML = textCell.getAttribute('data-original-content');
        if (commandCell && commandCell.hasAttribute('data-original-content')) commandCell.innerHTML = commandCell.getAttribute('data-original-content');
        if (notesCell && notesCell.hasAttribute('data-original-content')) notesCell.innerHTML = notesCell.getAttribute('data-original-content');
        if (actionsCell && actionsCell.hasAttribute('data-original-content')) {
            actionsCell.innerHTML = actionsCell.getAttribute('data-original-content');
            attachChecklistItemActionListenersForRow(itemRow);
        }
    }
    currentlyEditingChecklistItemId = null;
    updateChecklistSummary();
}

function attachChecklistItemActionListenersForRow(rowElement) {
    rowElement.querySelector('.edit-checklist-item')?.addEventListener('click', handleEditChecklistItem);
    rowElement.querySelector('.view-checklist-item-details')?.addEventListener('click', handleViewChecklistItemDetails);
    rowElement.querySelector('.delete-checklist-item')?.addEventListener('click', handleDeleteChecklistItem);
    rowElement.querySelector('.copy-checklist-command')?.addEventListener('click', handleCopyChecklistCommand);
}

async function handleDeleteChecklistItem(event) {
    const button = event.target.closest('button');
    const itemId = button.getAttribute('data-item-id');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId; // Assumes currentTargetId is correctly set in state

    uiService.showModalConfirm('Confirm Delete', `Are you sure you want to delete checklist item ID ${itemId}?`, async () => {
        try {
            await apiService.deleteChecklistItem(itemId);
            uiService.showModalMessage('Success', `Checklist item ID ${itemId} deleted.`);
            fetchAndDisplayChecklistItems(targetId); // Refresh the list for the current target
        } catch (error) {
            uiService.showModalMessage('Error', `Failed to delete item ${itemId}: ${escapeHtml(error.message)}`);
        }
    });
}

function handleCopyChecklistCommand(event) {
    const button = event.target.closest('button');
    const commandToCopy = button.getAttribute('data-command');
    if (commandToCopy) {
        copyToClipboard(commandToCopy)
            .then(() => uiService.showModalMessage('Copied', 'Command copied to clipboard!', true, 1500))
            .catch(err => {
                console.error('Failed to copy command: ', err);
                uiService.showModalMessage('Copy Failed', 'Could not copy command to clipboard. Check console for details.');
            });
    } else {
        uiService.showModalMessage('No Command', 'There is no command associated with this item to copy.');
    }
}

export function cancelActiveChecklistItemEdit() {
    if (currentlyEditingChecklistItemId) {
        restoreChecklistItemRow(currentlyEditingChecklistItemId);
        // currentlyEditingChecklistItemId is reset within restoreChecklistItemRow
    }
    if (isAddingNewChecklistItem) { // Also cancel new item row if active
        handleCancelNewChecklistItemRow();
    }
}

/**
 * Fetches and displays checklist items for a given target.
 * This is typically called when the 'current-target' view is loaded, or its checklist tab is shown.
 * @param {number|string} targetId - The ID of the target.
 */
export async function fetchAndDisplayChecklistItems(targetId) {
    const checklistContentDiv = document.getElementById('targetChecklistContent');
    if (!checklistContentDiv) {
        console.error("Checklist content div not found for target", targetId);
        return;
    }
    checklistContentDiv.innerHTML = '<p>Fetching checklist items...</p>';

    const appState = stateService.getState();
    const { currentPage, limit, sortBy, sortOrder, filterQuery, showIncompleteOnly } = appState.paginationState.targetChecklistItems;

    const params = {
        page: currentPage,
        limit: limit,
        sort_by: sortBy,
        sort_order: sortOrder,
        filter: filterQuery,
        show_incomplete_only: showIncompleteOnly // New parameter
    };

    try {
        const response = await apiService.getChecklistItems(targetId, params);
        stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
            ...appState.paginationState.targetChecklistItems,
            totalRecords: response.total_records,
            totalPages: response.total_pages,
            totalCompletedRecordsForFilter: response.total_completed_records_for_filter || 0, // Store this
            sortBy: response.sort_by || sortBy,
            sortOrder: response.sort_order || sortOrder,
            filterQuery: response.filter !== undefined ? response.filter : filterQuery,
            showIncompleteOnly: response.show_incomplete_only !== undefined ? response.show_incomplete_only : showIncompleteOnly,
        }}});

        renderChecklistTable(checklistContentDiv, response.items || [], targetId, response);
    } catch (error) {
        checklistContentDiv.innerHTML = `<p class="error-message">Error loading checklist: ${escapeHtml(error.message)}</p>`;
        updateChecklistSummary(); // Update summary to show error/no items
        renderPaginationControls(document.getElementById('checklistPaginationControlsContainer'), targetId, { page: 1, total_pages: 0, total_records: 0, limit: currentChecklistItemsPerPage }); // Render empty controls
    }
}

function renderChecklistTable(containerElement, items, targetId, paginationData) {
    let summaryHtml = '';
    let summaryContent = ''; // Will be populated by updateChecklistSummary

    const appState = stateService.getState();
    const { filterQuery, showIncompleteOnly, sortBy, sortOrder } = appState.paginationState.targetChecklistItems;

    // The main button container div
    let buttonsHtml = `
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
            <input type="search" id="checklistSearchInput" placeholder="Filter checklist items..." value="${escapeHtmlAttribute(filterQuery)}" style="padding: 8px; border: 1px solid #ccc; border-radius: 4px; flex-grow: 1; margin-right: 10px;">
            <label for="showIncompleteToggle" style="white-space: nowrap;"><input type="checkbox" id="showIncompleteToggle" ${showIncompleteOnly ? 'checked' : ''}> Show Incomplete Only</label>
        </div>
        <div id="checklistActionsContainer" style="margin-bottom: 15px; display: flex; align-items: center; gap: 10px; flex-wrap: wrap;">
            <button id="initiateAddChecklistItemBtn" class="primary small-button">Add New Item</button>
            <button id="copyFromTemplateBtn" class="secondary small-button" data-target-id="${targetId}">Copy from Template</button>
            <button id="deleteAllChecklistItemsBtn" class="secondary small-button" data-target-id="${targetId}">Delete All Checklist Items</button>
            <button id="saveChecklistLayoutBtn" class="secondary small-button" style="margin-left: auto;">Save Column Layout</button>
        </div>
        <div id="addChecklistItemMessage" class="message-area" style="margin-top: 10px; margin-bottom: 10px;"></div>
    `;


    let html = `
        <div id="checklistSummaryDisplay" class="checklist-summary" style="margin-bottom: 10px; padding: 8px; background-color: #e9ecef; border-radius: 4px; font-size: 0.9em;">
            ${summaryContent}
        </div>
        ${buttonsHtml} 
    `;
    const globalTableLayouts = appState.globalTableLayouts || {};
    const tableKey = 'currentTargetChecklist';
    const savedTableWidths = globalTableLayouts[tableKey] || {};

    const columnConfig = {
        id: { default: '5%', id: 'col-checklist-id', label: '#', sortKey: 'id' },
        is_completed: { default: '10%', id: 'col-checklist-status', label: 'Status', sortKey: 'is_completed' },
        item_text: { default: '30%', id: 'col-checklist-item', label: 'Item', sortKey: 'item_text' },
        item_command_text: { default: '20%', id: 'col-checklist-command', label: 'Command', sortKey: 'item_command_text' },
        notes: { default: 'auto', id: 'col-checklist-notes', label: 'Notes', sortKey: 'notes' },
        actions: { default: '18%', id: 'col-checklist-actions', label: 'Actions', sortKey: null } // Actions not sortable, adjusted width
    };
    
    html += `
        <table style="table-layout: fixed;">
            <thead id="checklistTableHead">
                <tr>`;
    for (const key in columnConfig) {
        const col = columnConfig[key];
        const sortableClass = col.sortKey ? 'sortable' : '';
        let sortIndicator = '';
        if (col.sortKey === sortBy) {
            sortIndicator = sortOrder === 'asc' ? ' <span class="sort-arrow">▲</span>' : ' <span class="sort-arrow">▼</span>';
        }
        html += `<th style="width: ${savedTableWidths[key] || col.default};" data-col-key="${col.sortKey || key}" id="${col.id}" class="${sortableClass}">${col.label}${sortIndicator}</th>`;
    }
    html += `       </tr>
            </thead>
            <tbody id="checklistTableBody">`;

    if (!items || items.length === 0) {
        const numberOfColumns = Object.keys(columnConfig).length;
        html += `<tr><td colspan="${numberOfColumns}" style="text-align: center; padding: 10px;">No checklist items defined for this target yet.</td></tr>`;
    } else {
        items.forEach((item) => { // Index is not reliable for overall numbering with pagination
            const notesText = item.notes && item.notes.Valid ? item.notes.String : '';
            const commandText = item.item_command_text && item.item_command_text.Valid ? item.item_command_text.String : '';
            const isCompleted = item.is_completed;
            
            let displayNotesText = notesText;
            if (notesText.length > NOTES_TRUNCATE_LENGTH) {
                displayNotesText = escapeHtml(notesText.substring(0, NOTES_TRUNCATE_LENGTH)) + '...';
            } else {
                displayNotesText = escapeHtml(notesText);
            }

            html += `
            <tr data-item-id="${item.id}" class="${isCompleted ? 'completed-item' : ''}"
                data-item-text="${escapeHtmlAttribute(item.item_text)}"
                data-item-command-text="${escapeHtmlAttribute(commandText)}"
                data-item-notes="${escapeHtmlAttribute(notesText)}">
                <td>${item.id}</td> 
                <td><input type="checkbox" class="checklist-item-complete" data-item-id="${item.id}" ${isCompleted ? 'checked' : ''}></td>
                <td class="checklist-item-text">${escapeHtml(item.item_text)}</td>
                <td class="checklist-item-command-text"><pre class="command-text">${escapeHtml(commandText)}</pre></td>
                <td class="checklist-item-notes" data-full-notes="${escapeHtmlAttribute(notesText)}">${displayNotesText}</td>
                <td>
                    <button class="action-button view-checklist-item-details" data-item-id="${item.id}" title="View Details">👁️</button>
                    <button class="action-button edit-checklist-item" data-item-id="${item.id}" title="Edit Item">✏️</button>
                    ${commandText ? `<button class="action-button copy-checklist-command" data-command="${escapeHtmlAttribute(commandText)}" title="Copy Command">📋</button>` : ''}
                    <button class="action-button delete-checklist-item" data-item-id="${item.id}" title="Delete Item">🗑️</button>
                </td>
            </tr>`;
        });
    }
    html += `</tbody></table>`;

    html += `<div id="checklistPaginationControlsContainer" style="margin-top: 15px;"></div>`;
    containerElement.innerHTML = html;

    document.getElementById('initiateAddChecklistItemBtn')?.addEventListener('click', () => initiateAddNewChecklistItemRow(targetId));
    document.getElementById('saveChecklistLayoutBtn')?.addEventListener('click', () => {
        tableService.saveCurrentTableLayout(tableKey, 'checklistTableHead');
    });
    document.getElementById('copyFromTemplateBtn')?.addEventListener('click', () => displayCopyFromTemplateModal(targetId));
    
    const deleteAllBtn = document.getElementById('deleteAllChecklistItemsBtn');
    if (deleteAllBtn) {
        deleteAllBtn.addEventListener('click', (event) => handleDeleteAllChecklistItems(event));
    }
    const searchInput = document.getElementById('checklistSearchInput');
    if (searchInput) {
        searchInput.addEventListener('input', debounce((event) => handleChecklistSearch(event, targetId), 750));
    }
    const showIncompleteToggle = document.getElementById('showIncompleteToggle');
    if (showIncompleteToggle) {
        showIncompleteToggle.addEventListener('change', (event) => handleShowIncompleteToggle(event, targetId));
    }


    document.querySelectorAll('#checklistTableHead th.sortable').forEach(th => {
        th.addEventListener('click', (event) => {
            event.preventDefault(); // Prevent default link behavior / page jump
            handleChecklistSort(th.dataset.colKey, targetId);
        });
    });

    document.querySelectorAll('.checklist-item-complete').forEach(checkbox => {
        checkbox.addEventListener('change', handleToggleChecklistItemComplete);
    });
    document.querySelectorAll('.edit-checklist-item').forEach(button => {
        button.addEventListener('click', handleEditChecklistItem);
    });
    document.querySelectorAll('.view-checklist-item-details').forEach(button => {
        button.addEventListener('click', handleViewChecklistItemDetails);
    });
    document.querySelectorAll('.delete-checklist-item').forEach(button => {
        button.addEventListener('click', handleDeleteChecklistItem);
    });
    document.querySelectorAll('.copy-checklist-command').forEach(button => {
        button.addEventListener('click', handleCopyChecklistCommand);
    });

    tableService.makeTableColumnsResizable('checklistTableHead');
    renderPaginationControls(document.getElementById('checklistPaginationControlsContainer'), targetId, paginationData);
    updateChecklistSummary();
}

async function handleViewChecklistItemDetails(event) {
    const button = event.target.closest('button');
    const itemId = button.getAttribute('data-item-id');
    const itemRow = document.querySelector(`tr[data-item-id="${itemId}"]`);

    if (!itemRow) {
        uiService.showModalMessage("Error", "Could not find item details to display.");
        return;
    }

    // Retrieve data from the row's attributes and elements
    const itemText = itemRow.getAttribute('data-item-text') || 'N/A';
    const commandText = itemRow.getAttribute('data-item-command-text') || '';
    const notesCell = itemRow.querySelector('.checklist-item-notes');
    const fullNotes = notesCell ? notesCell.getAttribute('data-full-notes') : 'N/A';
    const isCompletedCheckbox = itemRow.querySelector('.checklist-item-complete');
    const isCompleted = isCompletedCheckbox ? isCompletedCheckbox.checked : false;

    const modalTitle = `Details for Checklist Item #${itemId}`;
    const modalContent = `
        <div style="text-align: left; max-height: 70vh; overflow-y: auto; padding-right: 15px;">
            <p><strong>ID:</strong> ${itemId}</p>
            <p><strong>Status:</strong> ${isCompleted ? 'Completed' : 'Incomplete'}</p>
            <p><strong>Item Text:</strong></p>
            <pre style="white-space: pre-wrap; word-wrap: break-word; background-color: #f0f0f0; padding: 8px; border-radius: 4px; border: 1px solid #e0e0e0;">${escapeHtml(itemText)}</pre>
            <p><strong>Command Text:</strong></p>
            <pre style="white-space: pre-wrap; word-wrap: break-word; background-color: #f0f0f0; padding: 8px; border-radius: 4px; border: 1px solid #e0e0e0;">${commandText ? escapeHtml(commandText) : '(None)'}</pre>
            <p><strong>Notes:</strong></p>
            <pre style="white-space: pre-wrap; word-wrap: break-word; background-color: #f0f0f0; padding: 10px; border-radius: 4px; border: 1px solid #e0e0e0; max-height: 400px; overflow-y: auto;">${fullNotes ? escapeHtml(fullNotes) : '(None)'}</pre>
        </div>
    `;

    uiService.showModalMessage(modalTitle, modalContent);
}

function renderPaginationControls(container, targetId, paginationData) {
    if (!container || !paginationData) {
        if (container) container.innerHTML = ''; // Clear if no data
        return;
    }
    const appState = stateService.getState(); // Get fresh state for pagination
    const { page, total_pages, total_records, limit } = paginationData;
    // Update global state variables based on the data received from backend
    stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
        ...appState.paginationState.targetChecklistItems,
        currentPage: page, totalPages: total_pages, totalRecords: total_records, limit: limit
    }}});

    let paginationHTML = `<div class="pagination-controls">`;
    if (total_pages > 1) {
        paginationHTML += `<button id="checklistPrevPage" class="button" ${page <= 1 ? 'disabled' : ''}>&laquo; Prev</button>`;
        paginationHTML += `<span> Page <input type="number" id="checklistPageInput" value="${page}" min="1" max="${total_pages}" style="width: 60px; text-align: center;"> of ${total_pages} </span>`;
        paginationHTML += `<button id="checklistNextPage" class="button" ${page >= total_pages ? 'disabled' : ''}>Next &raquo;</button>`;
    }
    paginationHTML += `<span style="margin-left: 15px;">Show: <select id="checklistItemsPerPageSelect">`;
    [10, 25, 50, 100, 200].forEach(val => {
        paginationHTML += `<option value="${val}" ${limit === val ? 'selected' : ''}>${val}</option>`;
    });
    paginationHTML += `</select> items per page.</span>`;
    // Total records already shown in summary, so might be redundant here.
    // paginationHTML += `<span style="margin-left: 15px;">Total Items: ${total_records}</span>`;
    paginationHTML += `</div>`;

    container.innerHTML = paginationHTML;

    document.getElementById('checklistPrevPage')?.addEventListener('click', () => {
        const s = stateService.getState().paginationState.targetChecklistItems;
        if (s.currentPage > 1) {
            stateService.updateState({ paginationState: { ...stateService.getState().paginationState, targetChecklistItems: {...s, currentPage: s.currentPage - 1 }}});
            fetchAndDisplayChecklistItems(targetId);
        }
    });
    document.getElementById('checklistNextPage')?.addEventListener('click', () => {
        const s = stateService.getState().paginationState.targetChecklistItems;
        if (s.currentPage < s.totalPages) {
            stateService.updateState({ paginationState: { ...stateService.getState().paginationState, targetChecklistItems: {...s, currentPage: s.currentPage + 1 }}});
            fetchAndDisplayChecklistItems(targetId);
        }
    });
    document.getElementById('checklistPageInput')?.addEventListener('change', (e) => {
        let newPage = parseInt(e.target.value, 10);
        const s = stateService.getState().paginationState.targetChecklistItems;
        if (newPage >= 1 && newPage <= s.totalPages) {
            stateService.updateState({ paginationState: { ...stateService.getState().paginationState, targetChecklistItems: {...s, currentPage: newPage }}});
            fetchAndDisplayChecklistItems(targetId);
        } else {
            e.target.value = s.currentPage; // Reset to current if invalid
        }
    });
    document.getElementById('checklistItemsPerPageSelect')?.addEventListener('change', (e) => {
        const newLimit = parseInt(e.target.value, 10);
        const s = stateService.getState().paginationState.targetChecklistItems;
        stateService.updateState({ paginationState: { ...stateService.getState().paginationState, targetChecklistItems: {...s, limit: newLimit, currentPage: 1 }}});
        fetchAndDisplayChecklistItems(targetId);
    });
}

function handleChecklistSearch(event, targetId) {
    const newFilterQuery = event.target.value;
    const appState = stateService.getState();
    stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
        ...appState.paginationState.targetChecklistItems,
        filterQuery: newFilterQuery,
        currentPage: 1 
    }}});
    fetchAndDisplayChecklistItems(targetId);
}

function handleShowIncompleteToggle(event, targetId) {
    const newShowIncompleteOnly = event.target.checked;
    const appState = stateService.getState();
    stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
        ...appState.paginationState.targetChecklistItems,
        showIncompleteOnly: newShowIncompleteOnly,
        currentPage: 1
    }}});
    fetchAndDisplayChecklistItems(targetId);
}

function handleChecklistSort(columnKey, targetId, event) {
    // event parameter is no longer needed here if preventDefault is handled in the listener
    // Prevent sorting if a column resize was just completed
    if (tableService && typeof tableService.getIsResizing === 'function' && tableService.getIsResizing()) {
        console.log('[ChecklistView] Sort prevented due to active resize operation.');
        return;
    }

    if (!columnKey) return; 
    const appState = stateService.getState();
    const currentSortState = appState.paginationState.targetChecklistItems;
    let newSortDirection = 'asc';

    if (currentSortState.sortBy === columnKey) {
        newSortDirection = currentSortState.sortOrder === 'asc' ? 'desc' : 'asc';
    } else {
        newSortDirection = 'asc'; // Default to ascending for new column
    }
    stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
        ...currentSortState,
        sortBy: columnKey,
        sortOrder: newSortDirection,
        currentPage: 1
    }}});
    fetchAndDisplayChecklistItems(targetId);
}

// Moved from app.js to be co-located with the UI it controls
async function handleDeleteAllChecklistItems(event) {
    const button = event.target;
    const targetId = button.getAttribute('data-target-id');
    if (!targetId) {
        uiService.showModalMessage("Error", "Target ID not found for deleting checklist items.");
        return;
    }

    uiService.showModalConfirm("Confirm Delete All", "Are you sure you want to delete ALL checklist items for this target? This action cannot be undone.", async () => {
        try {
            await apiService.deleteAllChecklistItemsForTarget(targetId);
            uiService.showModalMessage("Success", "All checklist items for this target have been deleted.");
            const appState = stateService.getState();
            stateService.updateState({ paginationState: { ...appState.paginationState, targetChecklistItems: {
                ...appState.paginationState.targetChecklistItems,
                currentPage: 1,
                filterQuery: ''
            }}});
            fetchAndDisplayChecklistItems(targetId);
        } catch (error) {
            uiService.showModalMessage("Error", `Failed to delete checklist items: ${escapeHtml(error.message)}`);
        }
    });
}



// --- Checklist Templates Specific Functions (Modal for copying) ---

async function displayCopyFromTemplateModal(targetId) {
    const modalContentId = 'copyFromTemplateModalContent';
    let modalContentHTML = `
        <div id="${modalContentId}">
            <div class="form-group">
                <label for="templateSelectDropdown">Select Template:</label>
                <select id="templateSelectDropdown" style="width:100%;">
                    <option value="">-- Loading templates --</option>
                </select>
            </div>
            <div id="templateItemsPreviewContainer" style="margin-top:15px; max-height: 300px; overflow-y: auto; border: 1px solid #ccc; padding: 10px;">
                <p>Select a template to preview its items.</p>
            </div>
            <div id="templateCopyMessage" class="message-area" style="margin-top:10px;"></div>
        </div>
    `;

    uiService.showModalConfirm(
        "Copy Items from Template",
        modalContentHTML,
        async () => { // onConfirm callback
            const selectedTemplateId = document.getElementById('templateSelectDropdown').value;
            const selectedCheckboxes = document.querySelectorAll('#templateItemsPreviewContainer .template-item-checkbox-modal:checked');
            const messageArea = document.getElementById('templateCopyMessage');

            if (!selectedTemplateId) {
                if(messageArea) messageArea.textContent = "Please select a template.";
                return false; // Prevent modal from closing
            }
            if (selectedCheckboxes.length === 0) {
                if(messageArea) messageArea.textContent = "Please select at least one item to copy.";
                return false; // Prevent modal from closing
            }

            const itemsToCopy = [];
            selectedCheckboxes.forEach(checkbox => {
                itemsToCopy.push({
                    item_text: checkbox.getAttribute('data-item-text'),
                    item_command_text: checkbox.getAttribute('data-item-command-text') || '',
                    notes: checkbox.getAttribute('data-item-notes') || ''
                });
            });

            const payload = { target_id: parseInt(targetId, 10), items: itemsToCopy };
            try {
                const responseData = await apiService.copyChecklistTemplateItemsToTarget(payload);
                uiService.showModalMessage("Copy Complete", escapeHtml(responseData.message)); // Show a new modal for success
                fetchAndDisplayChecklistItems(targetId); // Refresh the target's checklist
                return true; // Close the copy modal
            } catch (error) {
                if(messageArea) messageArea.textContent = `Error copying items: ${escapeHtml(error.message)}`;
                console.error("Error copying checklist items from template:", error);
                return false; // Keep modal open on error
            }
        },
        () => { /* onCancel callback - do nothing */ },
        "Copy Selected Items", // Confirm button text
        "Cancel", // Cancel button text
        true // isCustomContent = true
    );

    // Populate dropdown after modal is shown
    const dropdown = document.getElementById('templateSelectDropdown');
    const previewContainer = document.getElementById('templateItemsPreviewContainer');
    try {
        const templates = await apiService.getChecklistTemplates();
        dropdown.innerHTML = '<option value="">-- Select a Template --</option>'; // Clear loading
        templates.forEach(template => {
            const option = document.createElement('option');
            option.value = template.id;
            option.textContent = escapeHtml(template.name);
            dropdown.appendChild(option);
        });
        dropdown.addEventListener('change', async (e) => {
            const templateId = e.target.value;
            if (templateId) {
                previewContainer.innerHTML = '<p>Loading items...</p>';
                try {
                    // Fetch ALL items for preview, no pagination for this modal view
                    const itemsResponse = await apiService.getChecklistTemplateItems(templateId, { page: 1, limit: 1000 });
                    const items = itemsResponse.items || [];
                    if (items.length > 0) {
                        let itemsHtml = '<ul>';
                        items.forEach(item => {
                            const notesText = item.notes && item.notes.Valid ? item.notes.String : '';
                            const commandText = item.item_command_text && item.item_command_text.Valid ? item.item_command_text.String : '';
                            itemsHtml += `<li>
                                <input type="checkbox" class="template-item-checkbox-modal" checked
                                       data-item-text="${escapeHtmlAttribute(item.item_text)}"
                                       data-item-command-text="${escapeHtmlAttribute(commandText)}"
                                       data-item-notes="${escapeHtmlAttribute(notesText)}">
                                ${escapeHtml(item.item_text)}
                            </li>`;
                        });
                        itemsHtml += '</ul>';
                        previewContainer.innerHTML = itemsHtml;
                    } else {
                        previewContainer.innerHTML = '<p>No items in this template.</p>';
                    }
                } catch (itemError) {
                    previewContainer.innerHTML = `<p class="error-message">Error loading template items: ${escapeHtml(itemError.message)}</p>`;
                }
            } else {
                previewContainer.innerHTML = '<p>Select a template to preview its items.</p>';
            }
        });
    } catch (error) {
        dropdown.innerHTML = '<option value="">-- Error loading templates --</option>';
        previewContainer.innerHTML = `<p class="error-message">Could not load templates: ${escapeHtml(error.message)}</p>`;
    }
}
