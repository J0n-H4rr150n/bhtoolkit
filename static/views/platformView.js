import { escapeHtml } from '../utils.js';

let currentlyEditingPlatformId = null;

/**
 * Loads the platforms view, including the form to add new platforms and the list of existing ones.
 * @param {HTMLElement} viewContentContainer - The DOM element where the view content will be rendered.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalConfirm - Function to display a confirmation modal.
 * @param {function} showModalMessage - Function to display an informational modal.
 * @param {function} handleAddPlatformCallback - Callback function to handle the submission of the add platform form.
 */
export async function loadPlatformsView(viewContentContainer, API_BASE, showModalConfirm, showModalMessage, handleAddPlatformCallback) {
    if (!viewContentContainer) {
        console.error("viewContentContainer not found in loadPlatformsView!");
        return;
    }
    viewContentContainer.innerHTML = `
        <h1>Platforms</h1>
        <form id="addPlatformForm" class="inline-form">
            <div class="form-group">
                <label for="platformName" class="hidden">Platform Name:</label>
                <input type="text" id="platformName" name="name" placeholder="Enter new platform name..." required>
            </div>
            <button type="submit" class="primary">Add Platform</button>
        </form>
        <div id="addPlatformMessage" class="message-area" style="margin-bottom: 15px;"></div>
        <div id="platformList">Loading platforms...</div>
    `;
    await fetchAndDisplayPlatforms(API_BASE, showModalConfirm, showModalMessage);
    const addForm = document.getElementById('addPlatformForm');
    if (addForm) {
        addForm.removeEventListener('submit', handleAddPlatformCallback);
        addForm.addEventListener('submit', handleAddPlatformCallback);
    }
}

/**
 * Fetches platforms from the API and displays them in a table.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalConfirm - Function to display a confirmation modal.
 * @param {function} showModalMessage - Function to display an informational modal.
 */
export async function fetchAndDisplayPlatforms(API_BASE, showModalConfirm, showModalMessage) {
    const platformListDiv = document.getElementById('platformList');
    if (!platformListDiv) {
        return;
    }
    platformListDiv.innerHTML = '<p>Fetching platforms...</p>';

    const fetchUrl = `${API_BASE}/platforms`;
    try {
        const response = await fetch(fetchUrl);
        if (!response.ok) {
            let errorMsg = `HTTP error! status: ${response.status} ${response.statusText}`;
            let responseText = await response.text();
             try {
                const errorData = JSON.parse(responseText);
                errorMsg = errorData.message || errorMsg;
            } catch(e) {
                errorMsg += ` - Response body: ${responseText.substring(0, 100)}...`;
            }
            throw new Error(errorMsg);
        }
        const platforms = await response.json();

        if (platforms && platforms.length > 0) {
            let tableHTML = `
                <h2>Existing Platforms</h2>
                <table>
                    <thead><tr><th>ID</th><th>Name</th><th>Actions</th></tr></thead>
                    <tbody>
            `;
            platforms.forEach(platform => {
                const safeName = escapeHtml(platform.name);
                tableHTML += `
                    <tr data-platform-id="${platform.id}" data-platform-name="${safeName}">
                        <td>${platform.id}</td>
                        <td class="platform-name-cell">${safeName}</td>
                        <td class="platform-actions-cell">
                            <button class="action-button view-platform" data-id="${platform.id}" data-name="${safeName}" title="View Targets for ${safeName}">👁️</button>
                            <button class="action-button edit-platform" data-id="${platform.id}" data-name="${safeName}" title="Edit Platform ${safeName}">✏️</button>
                            <button class="action-button delete-platform" data-id="${platform.id}" data-name="${safeName}" title="Delete Platform ${safeName}">🗑️</button>
                        </td>
                    </tr>
                `;
            });
            tableHTML += `</tbody></table>`;
            platformListDiv.innerHTML = tableHTML;
            attachPlatformActionListeners(API_BASE, showModalConfirm, showModalMessage);
        } else {
            platformListDiv.innerHTML = '<p>No platforms found.</p>';
        }
    } catch (error) {
         platformListDiv.innerHTML = `<p class="error-message">Error loading platforms: ${escapeHtml(error.message)}</p>`;
         console.error('Error fetching platforms:', error);
    }
}

function attachPlatformActionListeners(API_BASE, showModalConfirm, showModalMessage) {
     document.querySelectorAll('.view-platform').forEach(button => {
        button.removeEventListener('click', handleViewPlatform);
        button.addEventListener('click', handleViewPlatform);
    });
    document.querySelectorAll('.edit-platform').forEach(button => {
        button.removeEventListener('click', (event) => handleEditPlatform(event, API_BASE, showModalMessage));
        button.addEventListener('click', (event) => handleEditPlatform(event, API_BASE, showModalMessage));
    });
    document.querySelectorAll('.delete-platform').forEach(button => {
        button.removeEventListener('click', (event) => handleDeletePlatform(event, API_BASE, showModalConfirm, showModalMessage));
        button.addEventListener('click', (event) => handleDeletePlatform(event, API_BASE, showModalConfirm, showModalMessage));
    });
}

/**
 * Handles the click event for viewing targets of a specific platform.
 * Navigates to the targets view with the platform_id.
 * @param {Event} event - The click event.
 */
export function handleViewPlatform(event) {
    const button = event.target.closest('button');
    const platformId = button.getAttribute('data-id');
    window.location.hash = `targets?platform_id=${platformId}`;
}

/**
 * Handles the click event to initiate editing of a platform's name.
 * Replaces the platform name with an input field and provides save/cancel buttons.
 * @param {Event} event - The click event.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalMessage - Function to display an informational modal.
 */
export function handleEditPlatform(event, API_BASE, showModalMessage) {
    const button = event.target.closest('button');
    const platformId = button.getAttribute('data-id');
    const platformRow = button.closest('tr');
    if (!platformRow || currentlyEditingPlatformId === platformId) return;

    if (currentlyEditingPlatformId) {
        restorePlatformRow(currentlyEditingPlatformId, null, API_BASE, showModalMessage);
    }
    currentlyEditingPlatformId = platformId;

    const nameCell = platformRow.querySelector('.platform-name-cell');
    const actionsCell = platformRow.querySelector('.platform-actions-cell');
    const currentName = platformRow.getAttribute('data-platform-name');

    actionsCell.setAttribute('data-original-content', actionsCell.innerHTML);
    nameCell.setAttribute('data-original-content', nameCell.innerHTML);

    nameCell.innerHTML = `<input type="text" class="edit-platform-input" value="${currentName}" style="width: 100%;">`;
    actionsCell.innerHTML = `
        <button class="action-button save-platform-edit" data-id="${platformId}" title="Save Changes">✔️</button>
        <button class="action-button cancel-platform-edit" data-id="${platformId}" title="Cancel Edit">❌</button>
    `;

    actionsCell.querySelector('.save-platform-edit').addEventListener('click', (e) => handleSavePlatformEdit(e, API_BASE, showModalMessage));
    actionsCell.querySelector('.cancel-platform-edit').addEventListener('click', () => {
        restorePlatformRow(platformId, null, API_BASE, showModalMessage);
    });

    const inputField = nameCell.querySelector('input');
    inputField.focus();
    inputField.select();

    inputField.addEventListener('keydown', function(event) {
        if (event.key === 'Enter') {
            event.preventDefault();
            actionsCell.querySelector('.save-platform-edit').click();
        }
    });
}

/**
 * Handles saving the edited platform name to the backend.
 * @param {Event} event - The click event from the save button.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalMessage - Function to display an informational modal.
 */
export async function handleSavePlatformEdit(event, API_BASE, showModalMessage) {
    const button = event.target.closest('button');
    const platformId = button.getAttribute('data-id');
    const platformRow = document.querySelector(`tr[data-platform-id="${platformId}"]`);
    if (!platformRow) return;

    const inputField = platformRow.querySelector('.edit-platform-input');
    const newName = inputField.value.trim();

    if (!newName) {
        (showModalMessage || alert)('Error', 'Platform name cannot be empty.');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/platforms/${platformId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: newName })
        });

        if (response.ok) {
            const updatedPlatform = await response.json();
            restorePlatformRow(platformId, escapeHtml(updatedPlatform.name), API_BASE, showModalMessage);
            (showModalMessage || alert)('Success', `Platform updated to "${escapeHtml(updatedPlatform.name)}".`);
        } else {
            let errorMsg = `HTTP error! status: ${response.status}`;
            try { const errorData = await response.json(); errorMsg = errorData.message || errorMsg; } catch (e) {}
            (showModalMessage || alert)('Error', `Error updating platform: ${errorMsg}`);
            restorePlatformRow(platformId, null, API_BASE, showModalMessage);
        }
    } catch (error) {
        (showModalMessage || alert)('Network Error', `Network error updating platform: ${error.message}`);
        restorePlatformRow(platformId, null, API_BASE, showModalMessage);
    }
}

/**
 * Restores a platform row to its display state, either with updated data or original data.
 * @param {string|number} platformId - The ID of the platform whose row is to be restored.
 * @param {string|null} updatedName - The new name if the update was successful, otherwise null.
 * @param {string} API_BASE - The base URL for API calls (for re-attaching delete listener).
 * @param {function} showModalMessage - Function to display an informational modal (for re-attaching delete listener).
 */
export function restorePlatformRow(platformId, updatedName = null, API_BASE, showModalMessage) {
     const platformRow = document.querySelector(`tr[data-platform-id="${platformId}"]`);
     if (!platformRow) return;

     const nameCell = platformRow.querySelector('.platform-name-cell');
     const actionsCell = platformRow.querySelector('.platform-actions-cell');

     const displayName = updatedName !== null ? updatedName : platformRow.getAttribute('data-platform-name');

    if (nameCell) {
        // Use textContent for safer display of the platform name
        nameCell.textContent = displayName; // displayName is already HTML-escaped
    }
     platformRow.setAttribute('data-platform-name', displayName);
     const originalActionsHTML = actionsCell ? actionsCell.getAttribute('data-original-content') : null;

     if(actionsCell && originalActionsHTML) {
         actionsCell.innerHTML = originalActionsHTML;
         attachPlatformActionListeners(API_BASE, null, showModalMessage); // Re-attach all listeners for simplicity
     } else if (actionsCell) {
        actionsCell.innerHTML = `
            <button class="action-button view-platform" data-id="${platformId}" data-name="${displayName}" title="View Targets">👁️</button>
            <button class="action-button edit-platform" data-id="${platformId}" data-name="${displayName}" title="Edit Platform">✏️</button>
            <button class="action-button delete-platform" data-id="${platformId}" data-name="${displayName}" title="Delete Platform">🗑️</button>
        `;
        actionsCell.querySelector('.view-platform')?.addEventListener('click', handleViewPlatform);
        actionsCell.querySelector('.edit-platform')?.addEventListener('click', (e) => handleEditPlatform(e, API_BASE, showModalMessage));
        actionsCell.querySelector('.delete-platform')?.addEventListener('click', (e) => handleDeletePlatform(e, API_BASE, null, showModalMessage));
     }
     if (currentlyEditingPlatformId === platformId) {
        currentlyEditingPlatformId = null;
     }
}

/**
 * Handles deleting a platform after user confirmation.
 * @param {Event} event - The click event from the delete button.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalConfirm - Function to display a confirmation modal.
 * @param {function} showModalMessage - Function to display an informational modal.
 */
export async function handleDeletePlatform(event, API_BASE, showModalConfirm, showModalMessage) {
    const button = event.target.closest('button');
    const platformId = button.getAttribute('data-id');
    const platformName = button.getAttribute('data-name');

    const confirmFn = showModalConfirm || confirm;
    const messageFn = showModalMessage || alert;

    confirmFn(
        'Confirm Deletion',
        `Are you sure you want to delete platform "${platformName}" (ID: ${platformId})? This will also delete associated targets and cannot be undone.`,
        async () => {
            try {
                const response = await fetch(`${API_BASE}/platforms/${platformId}`, {
                    method: 'DELETE',
                });

                if (response.ok) {
                    const rowToRemove = document.querySelector(`tr[data-platform-id="${platformId}"]`);
                    if (rowToRemove) rowToRemove.remove();
                    messageFn('Success', `Platform "${platformName}" deleted successfully.`);
                } else {
                    let errorMsg = `HTTP error! status: ${response.status}`;
                    try { const errorData = await response.json(); errorMsg = errorData.message || errorMsg; } catch (e) {}
                    messageFn('Error', `Error deleting platform: ${errorMsg}`);
                }
            } catch (error) {
                messageFn('Network Error', `Network error deleting platform: ${error.message}`);
            }
        }
    );
}

/**
 * Handles the submission of the add platform form.
 * @param {Event} event - The form submission event.
 * @param {string} API_BASE - The base URL for API calls.
 * @param {function} showModalMessage - Function to display an informational modal.
 * @param {function} fetchAndDisplayPlatformsCallback - Callback to refresh the platforms list.
 */
export async function handleAddPlatform(event, API_BASE, showModalMessage, fetchAndDisplayPlatformsCallback) {
    event.preventDefault();
    const form = event.target;
    const platformNameInput = document.getElementById('platformName');
    const messageArea = document.getElementById('addPlatformMessage');
    const platformName = platformNameInput.value.trim();

    messageArea.textContent = '';
    messageArea.className = 'message-area';

    if (!platformName) {
        messageArea.textContent = 'Platform name cannot be empty.';
        messageArea.classList.add('error-message');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/platforms`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', },
            body: JSON.stringify({ name: platformName }),
        });

        if (response.ok) {
            const newPlatform = await response.json();
            messageArea.textContent = `Platform '${escapeHtml(newPlatform.name)}' (ID: ${newPlatform.id}) added successfully!`;
            messageArea.classList.add('success-message');
            form.reset();
            if (fetchAndDisplayPlatformsCallback) {
                await fetchAndDisplayPlatformsCallback();
            }
        } else {
            let errorMsg = `HTTP error! status: ${response.status}`;
            try { const errorData = await response.json(); errorMsg = errorData.message || errorMsg; } catch (e) {}
            messageArea.textContent = `Error adding platform: ${errorMsg}`;
            messageArea.classList.add('error-message');
        }
    } catch (error) {
        messageArea.textContent = `Network error: ${error.message}`;
        messageArea.classList.add('error-message');
    }
}
