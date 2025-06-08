import { escapeHtml, escapeHtmlAttribute, copyToClipboard } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService;

// DOM element references (will be queried within functions or passed)
let viewContentContainer; // Main container, passed to load functions

// State specific to checklist view
let isAddingNewChecklistItem = false;
let currentlyEditingChecklistItemId = null;

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

    if (!summaryDisplay || !tableBody) {
        return;
    }

    const allItemRows = tableBody.querySelectorAll('tr[data-item-id]');
    const totalCount = allItemRows.length;
    const completedCount = Array.from(allItemRows).filter(row => {
        const checkbox = row.querySelector('.checklist-item-complete');
        return checkbox && checkbox.checked;
    }).length;

    if (totalCount > 1) {
        const pendingCount = totalCount - completedCount;
        const percentDone = totalCount > 0 ? ((completedCount / totalCount) * 100).toFixed(1) : 0;
        const percentLeft = totalCount > 0 ? ((pendingCount / totalCount) * 100).toFixed(1) : 0;

        summaryDisplay.innerHTML = `
            <p><strong>Progress:</strong> ${completedCount} / ${totalCount} items completed.</p>
            <p>${percentDone}% done, ${percentLeft}% remaining.</p>
        `;
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
                event.preventDefault();
                newRow.querySelector('.save-new-checklist-item').click();
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
        const newItem = await apiService.addChecklistItem(payload);
        if (messageArea) {
            messageArea.textContent = 'Checklist item added successfully!';
            messageArea.classList.add('success-message');
        }
        newRow.remove();
        isAddingNewChecklistItem = false;
        const addButton = document.getElementById('initiateAddChecklistItemBtn');
        if (addButton) addButton.disabled = false;
        fetchAndDisplayChecklistItems(targetId);
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
    const currentNotes = notesCell.textContent;
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
                event.preventDefault();
                actionsCell.querySelector('.save-checklist-item-edit').click();
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
        cancelActiveChecklistItemEdit();
    }
}

async function handleToggleChecklistItemComplete(event) {
    const checkbox = event.target;
    const itemId = checkbox.getAttribute('data-item-id');
    const isCompleted = checkbox.checked;
    const row = checkbox.closest('tr');

    if (row) row.classList.toggle('completed-item', isCompleted);

    try {
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
        if (row) row.classList.toggle('completed-item', !isCompleted);
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
        itemRow.setAttribute('data-item-text', updatedItemData.item_text);
        itemRow.setAttribute('data-item-command-text', updatedItemData.item_command_text?.String || '');
        itemRow.setAttribute('data-item-notes', updatedItemData.notes?.String || '');

        if (actionsCell) {
            const itemIdAttr = updatedItemData.id;
            const commandForButton = updatedItemData.item_command_text?.String || '';
            actionsCell.innerHTML = `
                <button class="action-button edit-checklist-item" data-item-id="${itemIdAttr}" title="Edit Item">✏️</button>
                ${commandForButton ? `<button class="action-button copy-checklist-command" data-command="${escapeHtmlAttribute(commandForButton)}" title="Copy Command">📋</button>` : ''}
                <button class="action-button delete-checklist-item" data-item-id="${itemIdAttr}" title="Delete Item">🗑️</button>
            `;
            attachChecklistItemActionListenersForRow(itemRow);
        }
    } else {
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
    rowElement.querySelector('.delete-checklist-item')?.addEventListener('click', handleDeleteChecklistItem);
    rowElement.querySelector('.copy-checklist-command')?.addEventListener('click', handleCopyChecklistCommand);
}

async function handleDeleteChecklistItem(event) {
    const button = event.target.closest('button');
    const itemId = button.getAttribute('data-item-id');
    const appState = stateService.getState();
    const targetId = appState.currentTargetId;

    uiService.showModalConfirm('Confirm Delete', `Are you sure you want to delete checklist item ID ${itemId}?`, async () => {
        try {
            await apiService.deleteChecklistItem(itemId);
            uiService.showModalMessage('Success', `Checklist item ID ${itemId} deleted.`);
            fetchAndDisplayChecklistItems(targetId);
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
            .then(() => uiService.showModalMessage('Copied', 'Command copied to clipboard!'))
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
    }
    if (isAddingNewChecklistItem) {
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

    try {
        const items = await apiService.getChecklistItems(targetId);
        renderChecklistTable(checklistContentDiv, items || [], targetId);
    } catch (error) {
        checklistContentDiv.innerHTML = `<p class="error-message">Error loading checklist: ${escapeHtml(error.message)}</p>`;
    }
}

function renderChecklistTable(containerElement, items, targetId) {
    let summaryHtml = '';
    let summaryContent = '';
    if (items && items.length > 1) {
        const totalCount = items.length;
        const completedCount = items.filter(item => item.is_completed).length;
        const pendingCount = totalCount - completedCount;
        const percentDone = totalCount > 0 ? ((completedCount / totalCount) * 100).toFixed(1) : 0;
        const percentLeft = totalCount > 0 ? ((pendingCount / totalCount) * 100).toFixed(1) : 0;

        summaryContent = `
                <p><strong>Progress:</strong> ${completedCount} / ${totalCount} items completed.</p>
                <p>${percentDone}% done, ${percentLeft}% remaining.</p>
        `;
    }

    let html = `
        <div id="checklistSummaryDisplay" class="checklist-summary" style="margin-bottom: 10px; padding: 8px; background-color: #e9ecef; border-radius: 4px; font-size: 0.9em;">
            ${summaryContent}
        </div>
        <div style="margin-bottom: 15px;">
            <button id="initiateAddChecklistItemBtn" class="primary" style="margin-right:10px;">Add New Item</button>
            <button id="saveChecklistLayoutBtn" class="secondary small-button">Save Column Layout</button>
            <div id="addChecklistItemMessage" class="message-area" style="margin-top: 10px;"></div>
        </div>
    `;
    const appState = stateService.getState();
    const globalTableLayouts = appState.globalTableLayouts;
    const tableKey = 'currentTargetChecklist';
    const savedTableWidths = globalTableLayouts[tableKey] || {};

    const columnConfig = {
        index: { default: '5%', id: 'col-checklist-index' },
        status: { default: '10%', id: 'col-checklist-status' },
        item: { default: '35%', id: 'col-checklist-item' },
        command: { default: '20%', id: 'col-checklist-command' },
        notes: { default: 'auto', id: 'col-checklist-notes' },
        actions: { default: '15%', id: 'col-checklist-actions' }
    };
    
    // Always render the table structure
    html += `
        <table style="table-layout: fixed;">
            <thead id="checklistTableHead">
                <tr>
                    <th style="width: ${savedTableWidths.index || columnConfig.index.default};" data-col-key="index" id="${columnConfig.index.id}">#</th>
                    <th style="width: ${savedTableWidths.status || columnConfig.status.default};" data-col-key="status" id="${columnConfig.status.id}">Status</th>
                    <th style="width: ${savedTableWidths.item || columnConfig.item.default};" data-col-key="item" id="${columnConfig.item.id}">Item</th>
                    <th style="width: ${savedTableWidths.command || columnConfig.command.default};" data-col-key="command" id="${columnConfig.command.id}">Command</th>
                    <th style="width: ${savedTableWidths.notes || columnConfig.notes.default};" data-col-key="notes" id="${columnConfig.notes.id}">Notes</th>
                    <th style="width: ${savedTableWidths.actions || columnConfig.actions.default};" data-col-key="actions" id="${columnConfig.actions.id}">Actions</th>
                </tr>
            </thead>
            <tbody id="checklistTableBody">`;

    if (!items || items.length === 0) {
        const numberOfColumns = Object.keys(columnConfig).length;
        html += `<tr><td colspan="${numberOfColumns}" style="text-align: center; padding: 10px;">No checklist items defined for this target yet.</td></tr>`;
    } else {
        items.forEach((item, index) => {
            const notesText = item.notes && item.notes.Valid ? item.notes.String : '';
            const commandText = item.item_command_text && item.item_command_text.Valid ? item.item_command_text.String : '';
            const isCompleted = item.is_completed;

            html += `
            <tr data-item-id="${item.id}" class="${isCompleted ? 'completed-item' : ''}"
                data-item-text="${escapeHtmlAttribute(item.item_text)}"
                data-item-command-text="${escapeHtmlAttribute(commandText)}"
                data-item-notes="${escapeHtmlAttribute(notesText)}">
                <td>${index + 1}</td>
                <td><input type="checkbox" class="checklist-item-complete" data-item-id="${item.id}" ${isCompleted ? 'checked' : ''}></td>
                <td class="checklist-item-text">${escapeHtml(item.item_text)}</td>
                <td class="checklist-item-command-text"><pre class="command-text">${escapeHtml(commandText)}</pre></td>
                <td class="checklist-item-notes">${escapeHtml(notesText)}</td>
                <td>
                    <button class="action-button edit-checklist-item" data-item-id="${item.id}" title="Edit Item">✏️</button>
                    ${commandText ? `<button class="action-button copy-checklist-command" data-command="${escapeHtmlAttribute(commandText)}" title="Copy Command">📋</button>` : ''}
                    <button class="action-button delete-checklist-item" data-item-id="${item.id}" title="Delete Item">🗑️</button>
                </td>
            </tr>`;
        });
    }
    html += `</tbody></table>`;

    if (items && items.length > 0) {
        html += `<p style="margin-top:5px;">${items.length} total item(s).</p>`;
    }
    containerElement.innerHTML = html;

    document.getElementById('initiateAddChecklistItemBtn')?.addEventListener('click', () => initiateAddNewChecklistItemRow(targetId));
    document.getElementById('saveChecklistLayoutBtn')?.addEventListener('click', () => {
        tableService.saveCurrentTableLayout(tableKey, 'checklistTableHead');
    });

    document.querySelectorAll('.checklist-item-complete').forEach(checkbox => {
        checkbox.addEventListener('change', handleToggleChecklistItemComplete);
    });
    document.querySelectorAll('.edit-checklist-item').forEach(button => {
        button.addEventListener('click', handleEditChecklistItem);
    });
    document.querySelectorAll('.delete-checklist-item').forEach(button => {
        button.addEventListener('click', handleDeleteChecklistItem);
    });
    document.querySelectorAll('.copy-checklist-command').forEach(button => {
        button.addEventListener('click', handleCopyChecklistCommand);
    });

    tableService.makeTableColumnsResizable('checklistTableHead');
    updateChecklistSummary();
}

// --- Checklist Templates Specific Functions ---

async function fetchAndPopulateChecklistTemplatesDropdown() {
    const dropdown = document.getElementById('checklistTemplateDropdown');
    if (!dropdown) return;

    try {
        const templates = await apiService.getChecklistTemplates();
        templates.forEach(template => {
            const option = document.createElement('option');
            option.value = template.id;
            option.textContent = escapeHtml(template.name);
            dropdown.appendChild(option);
        });

        const appState = stateService.getState();
        if (appState.currentChecklistTemplateId && dropdown.querySelector(`option[value="${appState.currentChecklistTemplateId}"]`)) {
            dropdown.value = appState.currentChecklistTemplateId;
            const tableContainer = document.getElementById('checklistTemplateItemsTableContainer');
            if (tableContainer && tableContainer.innerHTML.includes('Please select a template')) {
                 await fetchAndDisplayChecklistTemplateItems();
            }
        }

        dropdown.addEventListener('change', async (event) => {
            const selectedTemplateId = event.target.value ? parseInt(event.target.value, 10) : null;
            if (selectedTemplateId) {
                window.location.hash = `#checklist-templates?template_id=${selectedTemplateId}`;
            } else {
                window.location.hash = `#checklist-templates`;
            }
        });
    } catch (error) {
        console.error("Error fetching checklist templates:", error);
        const messageArea = document.getElementById('checklistTemplateMessage');
        if (messageArea) messageArea.textContent = `Error loading templates: ${escapeHtml(error.message)}`;
    }
}

async function fetchAndDisplayChecklistTemplateItems() {
    const tableContainer = document.getElementById('checklistTemplateItemsTableContainer');
    const paginationControlsDiv = document.getElementById('checklistTemplatePaginationControls');
    const messageArea = document.getElementById('checklistTemplateMessage');

    if (!tableContainer || !paginationControlsDiv || !messageArea) {
        console.error("Required elements not found for fetchAndDisplayChecklistTemplateItems");
        return;
    }
    messageArea.textContent = '';

    const appState = stateService.getState();
    const currentTemplateId = appState.currentChecklistTemplateId;

    if (!currentTemplateId) {
        tableContainer.innerHTML = '<p>Please select a template to view its items.</p>';
        paginationControlsDiv.innerHTML = '';
        return;
    }

    tableContainer.innerHTML = `<p>Fetching items for template ID: ${currentTemplateId}...</p>`;
    paginationControlsDiv.innerHTML = '';

    const { currentPage, limit } = appState.paginationState.checklistTemplateItems;
    try {
        const apiResponse = await apiService.getChecklistTemplateItems(currentTemplateId, { page: currentPage, limit });
        stateService.updateState({
            paginationState: {
                checklistTemplateItems: {
                    ...appState.paginationState.checklistTemplateItems,
                    totalPages: apiResponse.total_pages || 1,
                    totalRecords: apiResponse.total_records || 0,
                }
            }
        });
        renderChecklistTemplateItemsTable(apiResponse.items || []);
        renderChecklistTemplatePaginationControls();
    } catch (error) {
        console.error("Error fetching template items:", error);
        tableContainer.innerHTML = `<p class="error-message">Error loading items: ${escapeHtml(error.message)}</p>`;
    }
}

function renderChecklistTemplateItemsTable(items) {
    const tableContainer = document.getElementById('checklistTemplateItemsTableContainer');
    if (!tableContainer) {
        console.error("checklistTemplateItemsTableContainer not found in renderChecklistTemplateItemsTable");
        return;
    }

    const appState = stateService.getState();
    const globalTableLayouts = appState.globalTableLayouts;
    const tableKey = 'checklistTemplateItemsTable';
    const savedTableWidths = globalTableLayouts[tableKey] || {};

    const columnConfig = {
        index: { default: '3%', id: 'col-template-item-index' },
        checkbox: { default: '5%', id: 'col-template-item-checkbox' },
        itemText: { default: '40%', id: 'col-template-item-text' },
        command: { default: '30%', id: 'col-template-item-command' },
        notes: { default: 'auto', id: 'col-template-item-notes' }
    };

    if (!items || items.length === 0) {
        tableContainer.innerHTML = "<p>No items found for this template.</p>";
        return;
    }
    let html = `
        <div style="margin-bottom: 10px;">
             <button id="saveTemplateItemsLayoutBtn" class="secondary small-button">Save Column Layout</button>
        </div>
        <table style="table-layout: fixed;">
            <thead id="checklistTemplateItemsTableHead">
                <tr>
                    <th style="width: ${savedTableWidths.index || columnConfig.index.default};" data-col-key="index" id="${columnConfig.index.id}">#</th>
                    <th style="width: ${savedTableWidths.checkbox || columnConfig.checkbox.default};" data-col-key="checkbox" id="${columnConfig.checkbox.id}"><input type="checkbox" id="selectAllTemplateItems" title="Select/Deselect All on this page"></th>
                    <th style="width: ${savedTableWidths.itemText || columnConfig.itemText.default};" data-col-key="itemText" id="${columnConfig.itemText.id}">Item Text</th>
                    <th style="width: ${savedTableWidths.command || columnConfig.command.default};" data-col-key="command" id="${columnConfig.command.id}">Command</th>
                    <th style="width: ${savedTableWidths.notes || columnConfig.notes.default};" data-col-key="notes" id="${columnConfig.notes.id}">Notes</th>
                </tr>
            </thead>
            <tbody>`;
    items.forEach((item, index) => {
        const notesText = item.notes && item.notes.Valid ? item.notes.String : '';
        const commandText = item.item_command_text && item.item_command_text.Valid ? item.item_command_text.String : '';
        html += `<tr>
                    <td>${index + 1}</td>
                    <td><input type="checkbox" class="template-item-checkbox" data-item-id="${item.id}" data-item-text="${escapeHtmlAttribute(item.item_text)}" data-item-command-text="${escapeHtmlAttribute(commandText)}" data-item-notes="${escapeHtmlAttribute(notesText)}"></td>
                    <td>${escapeHtml(item.item_text)}</td>
                    <td class="checklist-item-command-text"><pre class="command-text">${escapeHtml(commandText)}</pre></td>
                    <td>${escapeHtml(notesText)}</td>
                 </tr>`;
    });
    html += `</tbody></table>`;
    tableContainer.innerHTML = html;

    document.getElementById('saveTemplateItemsLayoutBtn')?.addEventListener('click', () => {
        tableService.saveCurrentTableLayout(tableKey, 'checklistTemplateItemsTableHead');
    });

    const selectAllCheckbox = document.getElementById('selectAllTemplateItems');
    if (selectAllCheckbox) {
        selectAllCheckbox.addEventListener('change', (event) => {
            document.querySelectorAll('.template-item-checkbox').forEach(cb => cb.checked = event.target.checked);
        });
    }
    tableService.makeTableColumnsResizable('checklistTemplateItemsTableHead');
}

function renderChecklistTemplatePaginationControls() {
    const container = document.getElementById('checklistTemplatePaginationControls');
    if (!container) return;

    const appState = stateService.getState();
    const { currentPage, totalPages, totalRecords, limit } = appState.paginationState.checklistTemplateItems;
    const currentTemplateId = appState.currentChecklistTemplateId;

    if (totalPages <= 1) {
        container.innerHTML = totalRecords > 0 ? `<p>${totalRecords} total item(s).</p>` : '';
        return;
    }

    let paginationHTML = `<p>Page ${currentPage} of ${totalPages} (${totalRecords} total items)</p>`;

    const prevButton = document.createElement('button');
    prevButton.className = 'secondary';
    prevButton.style.marginRight = '5px';
    prevButton.innerHTML = '&laquo; Previous';
    if (currentPage <= 1) prevButton.disabled = true;
    prevButton.addEventListener('click', () => {
        if (currentPage > 1) {
            window.location.hash = `#checklist-templates?template_id=${currentTemplateId}&page=${currentPage - 1}&limit=${limit}`;
        }
    });

    const nextButton = document.createElement('button');
    nextButton.className = 'secondary';
    nextButton.innerHTML = 'Next &raquo;';
    if (currentPage >= totalPages) nextButton.disabled = true;
    nextButton.addEventListener('click', () => {
        if (currentPage < totalPages) {
            window.location.hash = `#checklist-templates?template_id=${currentTemplateId}&page=${currentPage + 1}&limit=${limit}`;
        }
    });

    container.innerHTML = '';
    container.appendChild(document.createRange().createContextualFragment(paginationHTML));
    if (currentPage > 1) container.appendChild(prevButton);
    if (currentPage < totalPages) container.appendChild(nextButton);
}

async function handleCopySelectedItemsToTarget() {
    const messageArea = document.getElementById('copyTemplateItemsMessage');
    if (!messageArea) {
        console.error("copyTemplateItemsMessage element not found.");
        uiService.showModalMessage("UI Error", "Could not find the message area on the page.");
        return;
    }
    messageArea.textContent = '';
    messageArea.className = 'message-area';

    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;
    const currentTargetName = appState.currentTargetName;

    if (!currentTargetId) {
        const msg = 'Please set a current target before copying items. You can set a target from the "Targets" list or the "Current Target" view using the 📍 button.';
        messageArea.textContent = msg;
        messageArea.classList.add('error-message');
        uiService.showModalMessage("Action Required", msg);
        return;
    }

    const selectedCheckboxes = document.querySelectorAll('.template-item-checkbox:checked');
    if (selectedCheckboxes.length === 0) {
        const msg = 'Please select at least one item from the template to copy.';
        messageArea.textContent = msg;
        messageArea.classList.add('error-message');
        uiService.showModalMessage("Selection Needed", msg);
        return;
    }

    const itemsToCopy = [];
    selectedCheckboxes.forEach(checkbox => {
        itemsToCopy.push({
            item_text: checkbox.getAttribute('data-item-text'),
            item_command_text: checkbox.getAttribute('data-item-command-text') || '',
            notes: checkbox.getAttribute('data-item-notes') || ''
        });
    });

    const payload = {
        target_id: currentTargetId,
        items: itemsToCopy
    };

    uiService.showModalMessage("Copying Items...", `Attempting to copy ${itemsToCopy.length} selected item(s) to target "${escapeHtml(currentTargetName)}" (ID: ${currentTargetId})...`);

    try {
        const responseData = await apiService.copyChecklistTemplateItemsToTarget(payload);
        uiService.showModalMessage("Copy Complete", escapeHtml(responseData.message));
    } catch (error) {
        console.error("Error copying checklist items:", error);
        uiService.showModalMessage("Network Error", `Network error during copy: ${escapeHtml(error.message)}`);
    }
}

/**
 * Loads the checklist templates view.
 * @param {HTMLElement} mainViewContainer - The main container element for the view.
 */
export async function loadChecklistTemplatesView(mainViewContainer) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("viewContentContainer not provided to loadChecklistTemplatesView!");
        return;
    }
    if (!apiService || !uiService || !stateService || !tableService) {
        console.error("ChecklistView not initialized. Call initChecklistView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>ChecklistView module not initialized. Critical services are missing.</p>";
        return;
    }

    const appState = stateService.getState();
    const currentTargetId = appState.currentTargetId;

    viewContentContainer.innerHTML = `
        <h1>Checklist Templates</h1>
        <div class="form-group" style="margin-bottom: 20px;">
            <label for="checklistTemplateDropdown">Select Template:</label>
            <select id="checklistTemplateDropdown" style="min-width: 300px;">
                <option value="">-- Select a Template --</option>
            </select>
        </div>
        <div id="checklistTemplateMessage" class="message-area" style="margin-bottom: 15px;"></div>
        <div id="checklistTemplateItemsTableContainer">
            <p>Please select a template to view its items.</p>
        </div>
        <div id="checklistTemplatePaginationControls" class="pagination-controls" style="margin-top: 15px; text-align:center;"></div>
        <button id="copySelectedTemplateItemsBtn" class="primary" style="margin-top: 20px;" ${!currentTargetId ? 'disabled title="Set a current target first"' : ''}>
            Copy Selected to Current Target
        </button>
        <div id="copyTemplateItemsMessage" class="message-area" style="margin-top: 10px;"></div>
    `;

    await fetchAndPopulateChecklistTemplatesDropdown();
    const copyBtn = document.getElementById('copySelectedTemplateItemsBtn');
    if (copyBtn) {
        copyBtn.addEventListener('click', handleCopySelectedItemsToTarget);
    }

    if (appState.currentChecklistTemplateId) {
        const dropdown = document.getElementById('checklistTemplateDropdown');
        if (dropdown && dropdown.options.length > 1) {
            const tableContainer = document.getElementById('checklistTemplateItemsTableContainer');
            if (tableContainer && tableContainer.innerHTML.includes('Please select a template')) {
                await fetchAndDisplayChecklistTemplateItems();
            }
        }
    }
}
