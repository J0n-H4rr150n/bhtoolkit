import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

// Module-level variables for services
let apiService;
let uiService;
let stateService;
let tableService;

// DOM element references (will be queried within functions or passed)
let viewContentContainer; // Main container, passed to load functions

/**
 * Initializes the Checklist Template View module with necessary services.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService, tableService.
 */
export function initChecklistTemplateView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    tableService = services.tableService;
    console.log("[ChecklistTemplateView] Initialized.");
}

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
        console.error("ChecklistTemplateView not initialized. Call initChecklistTemplateView with services first.");
        viewContentContainer.innerHTML = "<p class='error-message'>ChecklistTemplateView module not initialized. Critical services are missing.</p>";
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
