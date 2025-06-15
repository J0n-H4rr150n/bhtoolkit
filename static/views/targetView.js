import { escapeHtml, escapeHtmlAttribute } from '../utils.js';

let currentlyEditingTargetId = null;

// Services and elements to be initialized
let apiService;
let uiService;
let stateService;
let currentTargetDisplayElement; // DOM element for current target display

/**
 * Initializes the Target View module with necessary services and DOM elements.
 * @param {Object} services - An object containing service instances.
 *                            Expected: apiService, uiService, stateService.
 * @param {HTMLElement} displayElement - The DOM element for displaying the current target.
 */
export function initTargetView(services, displayElement) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    currentTargetDisplayElement = displayElement;
    console.log("[TargetView] Initialized.");
}

async function getPlatformNameForTitle(platformId) {
    if (!platformId || !apiService || !apiService.getPlatformDetails) return Promise.resolve(`Platform ID: ${platformId}`);
    try {
        const platformData = await apiService.getPlatformDetails(platformId);
        return platformData ? escapeHtml(platformData.name) : `Platform ID: ${platformId} (Not Found)`;
    } catch (e) {
        console.error("Error fetching platform name for title:", e);
        return `Platform ID: ${platformId}`;
    }
}

async function fetchAndDisplayTargets(viewContentContainer, platformIdFilter = null) {
    const targetListDiv = document.getElementById('targetList');
    if (!targetListDiv) {
        console.error("targetListDiv not found in fetchAndDisplayTargets");
        return;
    }
    targetListDiv.innerHTML = '<p>Fetching targets...</p>';

    try {
        const targets = await apiService.getTargets(platformIdFilter);

        if (targets && targets.length > 0) {
            let tableHTML = `
                <h2>Targets</h2>
                <table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Codename</th>
                            <th>Slug</th>
                            <th>Link</th>
                            <th>Notes</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
            `;
            targets.forEach(target => {
                const safeCodename = escapeHtml(target.codename);
                const safeSlug = escapeHtml(target.slug);
                const safeLink = escapeHtml(target.link);
                const safeNotes = escapeHtml(target.notes || '');
                tableHTML += `
                    <tr data-target-id="${target.id}">
                        <td>${target.id}</td>
                        <td>${safeCodename}</td>
                        <td>${safeSlug}</td>
                        <td><a href="${safeLink}" target="_blank" title="${safeLink}">${safeLink.length > 40 ? safeLink.substring(0, 37) + '...' : safeLink}</a></td>
                        <td title="${safeNotes}">${safeNotes?.substring(0, 50) || ''}${safeNotes?.length > 50 ? '...' : ''}</td>
                        <td class="target-actions-cell">
                            <button class="action-button set-current-target" data-id="${target.id}" data-name="${safeCodename}" title="Set '${safeCodename}' as Current Target">📍</button>
                            <button class="action-button view-target-details" data-id="${target.id}" title="View Details">👁️</button>
                            <button class="action-button edit-target" data-id="${target.id}" title="Edit Target">✏️</button>
                            <button class="action-button delete-target" data-id="${target.id}" data-slug="${safeSlug}" title="Delete Target (Not Implemented)">🗑️</button>
                        </td>
                    </tr>
                `;
            });
            tableHTML += `</tbody></table>`;
            targetListDiv.innerHTML = tableHTML;
            attachTargetActionListeners();
        } else {
            const platformName = platformIdFilter ? await getPlatformNameForTitle(platformIdFilter) : '';
            targetListDiv.innerHTML = `<p>No targets found${platformIdFilter ? ` for platform ${platformName}` : ''}.</p>`;
        }
    } catch (error) {
        targetListDiv.innerHTML = `<p class="error-message">Error loading targets: ${escapeHtml(error.message)}</p>`;
        console.error('Error fetching targets:', error);
    }
}

function attachTargetActionListeners() {
    document.querySelectorAll('.set-current-target').forEach(button => {
        button.removeEventListener('click', handleSetCurrentTarget);
        button.addEventListener('click', handleSetCurrentTarget);
    });
    document.querySelectorAll('.view-target-details').forEach(button => {
        button.removeEventListener('click', handleViewTargetDetails);
        button.addEventListener('click', handleViewTargetDetails);
    });
    document.querySelectorAll('.edit-target').forEach(button => {
        button.removeEventListener('click', handleEditTarget);
        button.addEventListener('click', handleEditTarget);
    });
    // Placeholder for delete listener
    // document.querySelectorAll('.delete-target').forEach(button => {
    //     button.removeEventListener('click', handleDeleteTarget);
    //     button.addEventListener('click', handleDeleteTarget);
    // });
}

function attachTargetActionListenersForRow(rowElement) {
    rowElement.querySelector('.set-current-target')?.addEventListener('click', handleSetCurrentTarget);
    rowElement.querySelector('.view-target-details')?.addEventListener('click', handleViewTargetDetails);
    rowElement.querySelector('.edit-target')?.addEventListener('click', handleEditTarget);
    // Placeholder for delete listener
    // rowElement.querySelector('.delete-target')?.addEventListener('click', handleDeleteTarget);
}

export function cancelActiveTargetEdit() {
    if (currentlyEditingTargetId) {
        restoreTargetRow(currentlyEditingTargetId);
        // currentlyEditingTargetId is reset within restoreTargetRow
    }
}

function restoreTargetRow(targetId, updatedTargetData = null) {
    const targetRow = document.querySelector(`tr[data-target-id="${targetId}"]`);
    if (!targetRow) return;

    const linkCell = targetRow.querySelector('td:nth-child(4)');
    const notesCell = targetRow.querySelector('td:nth-child(5)');
    const actionsCell = targetRow.querySelector('.target-actions-cell');

    if (updatedTargetData) {
        const safeLink = escapeHtml(updatedTargetData.link);
        const safeNotes = escapeHtml(updatedTargetData.notes || '');
        // Link cell still needs innerHTML for the <a> tag, but its content is escaped.
        if (linkCell) {
            const linkDisplay = safeLink.length > 40 ? safeLink.substring(0, 37) + '...' : safeLink;
            linkCell.innerHTML = `<a href="${escapeHtmlAttribute(updatedTargetData.link)}" target="_blank" title="${safeLink}">${linkDisplay}</a>`;
        }
        if (notesCell) {
            // Use textContent for the notes cell for maximum safety.
            // The full note is in the title, which is an attribute and thus safe.
            notesCell.textContent = safeNotes.substring(0, 50) + (safeNotes.length > 50 ? '...' : '');
            notesCell.setAttribute('title', safeNotes); // Keep full notes in title
        }
    } else { // Restore from data-original-content if no new data (i.e., cancellation)
        if (linkCell && linkCell.hasAttribute('data-original-content')) linkCell.innerHTML = linkCell.getAttribute('data-original-content');
        if (notesCell && notesCell.hasAttribute('data-original-content')) notesCell.innerHTML = notesCell.getAttribute('data-original-content');
    }

    // Always restore actions cell from original content or rebuild if not available
    if (actionsCell) {
        const originalContent = actionsCell.getAttribute('data-original-content');
        if (originalContent) {
            actionsCell.innerHTML = originalContent;
        } else {
            // Fallback: Rebuild if original content wasn't stored (shouldn't happen if edit started correctly)
            const safeCodename = escapeHtml(targetRow.cells[1].textContent); // Assuming codename is in the second cell
            const safeSlug = escapeHtml(targetRow.cells[2].textContent); // Assuming slug is in the third cell
            actionsCell.innerHTML = `
                <button class="action-button set-current-target" data-id="${targetId}" data-name="${safeCodename}" title="Set '${safeCodename}' as Current Target">📍</button>
                <button class="action-button view-target-details" data-id="${targetId}" title="View Details">👁️</button>
                <button class="action-button edit-target" data-id="${targetId}" title="Edit Target">✏️</button>
                <button class="action-button delete-target" data-id="${targetId}" data-slug="${safeSlug}" title="Delete Target (Not Implemented)">🗑️</button>
            `;
        }
        attachTargetActionListenersForRow(targetRow); // Re-attach listeners to restored/rebuilt buttons
    }
    currentlyEditingTargetId = null;
}

async function handleSetCurrentTarget(event) {
    const button = event.target.closest('button');
    const targetId = button.getAttribute('data-id');
    const targetName = button.getAttribute('data-name');
    const targetIdNum = parseInt(targetId, 10);

    if (isNaN(targetIdNum)) {
        console.error("Invalid target ID for setCurrentTarget:", targetId);
        uiService.showModalMessage("Error", "Invalid target ID encountered.");
        return;
    }

    try {
        await apiService.setCurrentTargetSetting(targetIdNum);
        stateService.updateState({
            currentTargetId: targetIdNum,
            currentTargetName: targetName
        });

        if (currentTargetDisplayElement) {
            const appState = stateService.getState(); // Get fresh state
            currentTargetDisplayElement.textContent = `Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId})`;
            currentTargetDisplayElement.title = `Current Target: ${escapeHtml(appState.currentTargetName)} (ID: ${appState.currentTargetId})`;
        }
        uiService.showModalMessage('Current Target Set', `Set current target to "${escapeHtml(targetName)}" (ID: ${targetIdNum}).`);
    } catch (error) {
        console.error("Error setting current target:", error);
        uiService.showModalMessage("Error", `Failed to set current target: ${error.message}`);
    }
}

function handleViewTargetDetails(event) {
    const button = event.target.closest('button');
    const targetId = button.getAttribute('data-id');
    window.location.hash = `current-target?id=${targetId}`;
}

function handleEditTarget(event) {
    const button = event.target.closest('button');
    const targetId = button.getAttribute('data-id');
    const targetRow = button.closest('tr');

    if (!targetRow || currentlyEditingTargetId === targetId) return;

    cancelActiveTargetEdit(); // Cancel any other ongoing edit
    currentlyEditingTargetId = targetId;

    const linkCell = targetRow.querySelector('td:nth-child(4)');
    const notesCell = targetRow.querySelector('td:nth-child(5)');
    const actionsCell = targetRow.querySelector('.target-actions-cell');

    const currentLink = targetRow.querySelector('a')?.href || linkCell.textContent;
    const currentNotes = notesCell.getAttribute('title') || notesCell.textContent; // Use title for full notes

    // Store original content
    if (linkCell) linkCell.setAttribute('data-original-content', linkCell.innerHTML);
    if (notesCell) notesCell.setAttribute('data-original-content', notesCell.innerHTML);
    if (actionsCell) actionsCell.setAttribute('data-original-content', actionsCell.innerHTML);

    if (linkCell) linkCell.innerHTML = `<input type="text" class="edit-target-link-input" value="${escapeHtmlAttribute(currentLink)}" style="width: 95%;">`;
    if (notesCell) notesCell.innerHTML = `<textarea class="edit-target-notes-input" style="width: 95%; min-height: 40px;">${escapeHtml(currentNotes)}</textarea>`;

    if (actionsCell) {
        actionsCell.innerHTML = `
            <button class="action-button save-target-edit" data-id="${targetId}" title="Save Changes">✔️</button>
            <button class="action-button cancel-target-edit" data-id="${targetId}" title="Cancel Edit">❌</button>
        `;
        actionsCell.querySelector('.save-target-edit').addEventListener('click', handleSaveTargetEdit);
        actionsCell.querySelector('.cancel-target-edit').addEventListener('click', cancelActiveTargetEdit);
    }
    targetRow.querySelector('.edit-target-link-input')?.focus();

    // Add Enter key listener for saving
    [
        targetRow.querySelector('.edit-target-link-input'),
        targetRow.querySelector('.edit-target-notes-input')
    ].forEach(inputField => {
        inputField?.addEventListener('keydown', function(event) {
            if (event.key === 'Enter') {
                if (inputField.tagName === 'TEXTAREA' && event.shiftKey) {
                    return; // Allow shift+enter for newlines in textarea
                }
                event.preventDefault();
                actionsCell.querySelector('.save-target-edit').click();
            }
        });
    });
}

async function handleSaveTargetEdit(event) {
    const button = event.target.closest('button');
    const targetId = button.getAttribute('data-id');
    const targetRow = document.querySelector(`tr[data-target-id="${targetId}"]`);
    if (!targetRow) return;

    const linkInput = targetRow.querySelector('.edit-target-link-input');
    const notesInput = targetRow.querySelector('.edit-target-notes-input');

    const newLink = linkInput.value.trim();
    const newNotes = notesInput.value.trim();

    if (!newLink) {
        uiService.showModalMessage('Error', 'Target link cannot be empty.');
        return;
    }
    if (!newLink.startsWith('#') && !newLink.startsWith('http://') && !newLink.startsWith('https://')) {
        uiService.showModalMessage('Error', 'Link must be a valid URL (starting with http:// or https://) or a placeholder (starting with #).');
        return;
    }

    const payload = { link: newLink, notes: newNotes };

    try {
        const updatedTarget = await apiService.updateTarget(targetId, payload); // API should return full updated target
        restoreTargetRow(targetId, updatedTarget);
        uiService.showModalMessage('Success', `Target "${escapeHtml(updatedTarget.codename)}" updated.`);
    } catch (error) {
        uiService.showModalMessage('Error', `Error updating target: ${error.message}`);
        restoreTargetRow(targetId); // Restore to original on error
    }
}

async function handleAddTarget(event, platformId, viewContentContainer) {
    event.preventDefault();
    const form = event.target;
    const codenameInput = form.querySelector('#targetCodename');
    const slugInput = form.querySelector('#targetSlug');
    const linkInput = form.querySelector('#targetLink');
    const notesInput = form.querySelector('#targetNotes');
    const messageArea = document.getElementById('addTargetMessage');

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    const codename = codenameInput.value.trim();
    const slug = slugInput.value.trim();
    const link = linkInput.value.trim();
    const notes = notesInput.value.trim();

    if (!codename || !slug || !link) {
        messageArea.textContent = 'Codename, Slug, and Link are required fields.';
        messageArea.classList.add('error-message');
        return;
    }
    if (!link.startsWith('#') && !link.startsWith('http://') && !link.startsWith('https://')) {
        messageArea.textContent = 'Link must be a valid URL (starting with http:// or https://) or a placeholder (starting with #).';
        messageArea.classList.add('error-message');
        return;
    }

    const payload = {
        platform_id: parseInt(platformId, 10),
        codename: codename,
        slug: slug,
        link: link,
        notes: notes || null // Send null if notes is empty
    };

    try {
        const newTarget = await apiService.addTarget(payload);
        messageArea.textContent = `Target '${escapeHtml(newTarget.codename)}' (ID: ${newTarget.id}) added successfully!`;
        messageArea.classList.add('success-message');
        form.reset();
        await fetchAndDisplayTargets(viewContentContainer, platformId); // Refresh the list
    } catch (error) {
        messageArea.textContent = `Error adding target: ${escapeHtml(error.message)}`;
        messageArea.classList.add('error-message');
    }
}

/**
 * Loads the targets view, optionally filtered by platform ID.
 * This is the main entry point for rendering the targets view.
 * @param {HTMLElement} viewContentContainer - The DOM element where the view content will be rendered.
 * @param {number|string|null} platformIdFilter - Optional platform ID to filter targets by.
 */
export async function loadTargetsView(viewContentContainer, platformIdFilter = null) {
    if (!apiService || !uiService || !stateService) {
        console.error("TargetView not initialized. Call initTargetView with services and displayElement first.");
        if (viewContentContainer) viewContentContainer.innerHTML = "<p class='error-message'>TargetView module not initialized. Critical services are missing.</p>";
        return;
    }

    let title = "All Targets";
    let platformName = '';

    if (platformIdFilter) {
        platformName = await getPlatformNameForTitle(platformIdFilter); // Uses initialized apiService
        title = `Targets for Platform: ${platformName}`;
    }

    if (!viewContentContainer) {
        console.error("viewContentContainer not found in loadTargetsView!");
        return;
    }

    let addTargetFormHTML = '';
    if (platformIdFilter) {
        // Only show the add form when viewing targets for a specific platform
        addTargetFormHTML = `
            <form id="addTargetForm" class="inline-form" style="margin-bottom: 20px; padding: 15px; border: 1px solid #ddd; border-radius: 4px;">
                <h4>Add New Target to ${escapeHtml(platformName)}</h4>
                <div class="form-group">
                    <input type="text" id="targetCodename" name="codename" placeholder="Codename (e.g., example-web)" required>
                </div>
                <div class="form-group">
                    <input type="text" id="targetSlug" name="slug" placeholder="Slug (e.g., example-web)" required>
                </div>
                <div class="form-group">
                    <input type="url" id="targetLink" name="link" placeholder="Link (e.g., https://example.com)" required>
                </div>
                 <div class="form-group">
                    <textarea id="targetNotes" name="notes" placeholder="Optional notes..." rows="1"></textarea>
                </div>
                <button type="submit" class="primary">Add Target</button>
            </form>
            <div id="addTargetMessage" class="message-area" style="margin-bottom: 15px;"></div>
        `;
    }

    viewContentContainer.innerHTML = `
        <h1>${title}</h1>
        ${addTargetFormHTML}
        <div id="targetList">Loading targets...</div>
    `;

    if (platformIdFilter) {
        const addTargetForm = document.getElementById('addTargetForm');
        if (addTargetForm) {
            // Create a bound version of handleAddTarget to correctly pass platformIdFilter and viewContentContainer
            const boundHandleAddTarget = (event) => handleAddTarget(event, platformIdFilter, viewContentContainer);
            addTargetForm.removeEventListener('submit', boundHandleAddTarget); // Ensure no duplicates
            addTargetForm.addEventListener('submit', boundHandleAddTarget);
        }
    }

    await fetchAndDisplayTargets(viewContentContainer, platformIdFilter);
}
