// static/uiService.js
import { escapeHtml } from './utils.js';

let modalOverlay;
let modalTitle;
let modalMessage;
let modalConfirmBtn;
let modalCancelBtn;
let modalOkBtn;
let breadcrumbContainer;

let modalConfirmCallback = null;

/**
 * Shows the modal dialog.
 */
function showModal() {
    if (modalOverlay) modalOverlay.classList.remove('hidden');
}

/**
 * Hides the modal dialog and clears any confirm callback.
 */
function hideModal() {
    if (modalOverlay) modalOverlay.classList.add('hidden');
    modalConfirmCallback = null;
}

/**
 * Displays a modal with a title and message, and an "OK" button.
 * @param {string} title - The title for the modal.
 * @param {string} message - The message content for the modal (can include HTML).
 */
export function showModalMessage(title, message) {
    if (!modalTitle || !modalMessage || !modalConfirmBtn || !modalCancelBtn || !modalOkBtn) {
        console.error("Modal elements not found for showModalMessage");
        alert(`${title}\n${message}`); // Fallback
        return;
    }
    modalTitle.textContent = title;
    modalMessage.innerHTML = message;
    modalConfirmBtn.classList.add('hidden');
    modalCancelBtn.classList.add('hidden');
    modalOkBtn.classList.remove('hidden');
    showModal();
}

/**
 * Displays a modal with a title, message, and "Confirm" and "Cancel" buttons.
 * @param {string} title - The title for the modal.
 * @param {string} message - The message content for the modal (can include HTML).
 * @param {function} onConfirm - The callback function to execute when the confirm button is clicked.
 */
export function showModalConfirm(title, message, onConfirm) {
    if (!modalTitle || !modalMessage || !modalConfirmBtn || !modalCancelBtn || !modalOkBtn) {
       console.error("Modal elements not found for showModalConfirm");
       if (confirm(`${title}\n${message}`)) { // Fallback
           if (typeof onConfirm === 'function') onConfirm();
       }
       return;
    }
    modalTitle.textContent = title;
    modalMessage.innerHTML = message;
    modalConfirmBtn.classList.remove('hidden');
    modalCancelBtn.classList.remove('hidden');
    modalOkBtn.classList.add('hidden');
    modalConfirmCallback = onConfirm;
    showModal();
}

/**
 * Updates the breadcrumb navigation.
 * @param {Array<Object>} segments - An array of segment objects. Each object should have a 'name' and optionally a 'hash'.
 */
export function updateBreadcrumbs(segments) {
    if (!breadcrumbContainer) {
        console.warn("Breadcrumb container not found in uiService.");
        return;
    }
    breadcrumbContainer.innerHTML = '';

    const homeSegment = { name: 'Home', hash: '#platforms' };
    const allSegments = [homeSegment];

    segments.forEach(seg => {
        if (seg && seg.name && seg.name.toLowerCase() !== 'home') {
            allSegments.push(seg);
        }
    });

    allSegments.forEach((segment, index) => {
        if (index > 0) {
            const separator = document.createElement('span');
            separator.className = 'separator';
            separator.textContent = '>';
            breadcrumbContainer.appendChild(separator);
        }

        if (segment.hash && index < allSegments.length - 1) {
            const link = document.createElement('a');
            link.href = segment.hash;
            link.textContent = escapeHtml(segment.name);
            breadcrumbContainer.appendChild(link);
        } else {
            const span = document.createElement('span');
            span.className = 'current-page';
            span.textContent = escapeHtml(segment.name);
            breadcrumbContainer.appendChild(span);
        }
    });
}

/**
 * Initializes the UI service with necessary DOM elements and attaches event listeners.
 * @param {Object} elements - An object containing DOM elements.
 *                            Expected: modalOverlay, modalTitle, modalMessage,
 *                            modalConfirmBtn, modalCancelBtn, modalOkBtn, breadcrumbContainer.
 */
export function initUIService(elements) {
    modalOverlay = elements.modalOverlay;
    modalTitle = elements.modalTitle;
    modalMessage = elements.modalMessage;
    modalConfirmBtn = elements.modalConfirmBtn;
    modalCancelBtn = elements.modalCancelBtn;
    modalOkBtn = elements.modalOkBtn;
    breadcrumbContainer = elements.breadcrumbContainer;

    if (modalOkBtn) modalOkBtn.addEventListener('click', hideModal);
    if (modalCancelBtn) modalCancelBtn.addEventListener('click', hideModal);
    if (modalConfirmBtn) {
        modalConfirmBtn.addEventListener('click', () => {
            if (typeof modalConfirmCallback === 'function') modalConfirmCallback();
            hideModal();
        });
    }
    if (modalOverlay) {
        modalOverlay.addEventListener('click', (event) => {
            if (event.target === modalOverlay) hideModal();
        });
    }
}
