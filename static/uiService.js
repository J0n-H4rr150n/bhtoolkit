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
let modalCancelCallback = null; // Add a cancel callback

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
    modalCancelCallback = null;
}

/**
 * Displays a modal with a title and message, and an "OK" button.
 * @param {string} title - The title for the modal.
 * @param {string|HTMLElement} messageOrElement - The message content (string, can include HTML) or an HTMLElement to append.
 * @param {boolean} [isTemp=false] - If true, the modal will auto-close.
 * @param {number} [autoCloseDelay=2000] - Delay in ms before auto-closing.
 */
export function showModalMessage(title, messageOrElement, isTemp = false, autoCloseDelay = 2000) {
    if (!modalTitle || !modalMessage || !modalConfirmBtn || !modalCancelBtn || !modalOkBtn) {
        console.error("Modal elements not found for showModalMessage");
        alert(`${title}\n${typeof messageOrElement === 'string' ? messageOrElement : 'Complex content'}`); // Fallback
        return;
    }
    modalTitle.textContent = title;
    modalMessage.innerHTML = ''; // Clear previous content
    if (typeof messageOrElement === 'string') {
        modalMessage.innerHTML = messageOrElement; // Use innerHTML to allow basic HTML like <br>
    } else if (messageOrElement instanceof HTMLElement) {
        modalMessage.appendChild(messageOrElement); // Append the element
    } else {
        modalMessage.textContent = String(messageOrElement); // Fallback for other types
    }
    modalConfirmBtn.classList.add('hidden');
    modalCancelBtn.classList.add('hidden');

    if (isTemp) {
        modalOkBtn.classList.add('hidden'); // Hide OK button for temp messages
        setTimeout(() => {
            // Only hide if this specific modal is still visible
            if (modalTitle.textContent === title) {
                hideModal();
            }
        }, autoCloseDelay);
    } else {
        modalOkBtn.classList.remove('hidden');
    }
    showModal();
}

/**
 * Displays a modal with a title, message, and "Confirm" and "Cancel" buttons.
 * @param {string} title - The title for the modal.
 * @param {string|HTMLElement} content - The message content for the modal (can include HTML) or an HTMLElement.
 * @param {function} onConfirm - The callback function to execute when the confirm button is clicked.
 * @param {function} [onCancel] - Optional callback for when cancel is clicked.
 * @param {string} [confirmText='Confirm'] - Text for the confirm button.
 * @param {string} [cancelText='Cancel'] - Text for the cancel button.
 * @param {boolean} [isCustomContent=false] - If true, the content is treated as raw HTML or an element.
 */
export function showModalConfirm(title, content, onConfirm, onCancel = null, confirmText = 'Confirm', cancelText = 'Cancel', isCustomContent = false) {
    if (!modalTitle || !modalMessage || !modalConfirmBtn || !modalCancelBtn || !modalOkBtn) {
       console.error("Modal elements not found for showModalConfirm");
       if (confirm(`${title}\n${content}`)) { // Fallback
           if (typeof onConfirm === 'function') onConfirm();
       }
       return;
    }
    modalTitle.textContent = title;
    modalMessage.innerHTML = ''; // Clear first
    if (isCustomContent) {
        if (typeof content === 'string') {
            modalMessage.innerHTML = content;
        } else if (content instanceof HTMLElement) {
            modalMessage.appendChild(content);
        }
    } else {
        modalMessage.textContent = content; // Treat as plain text if not custom
    }
    
    modalConfirmBtn.textContent = confirmText;
    modalCancelBtn.textContent = cancelText;

    modalConfirmBtn.classList.remove('hidden');
    modalCancelBtn.classList.remove('hidden');
    modalOkBtn.classList.add('hidden');
    
    modalConfirmCallback = onConfirm;
    modalCancelCallback = onCancel;
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
    if (modalCancelBtn) {
        modalCancelBtn.addEventListener('click', () => {
            if (typeof modalCancelCallback === 'function') modalCancelCallback();
            hideModal();
        });
    }
    if (modalConfirmBtn) {
        modalConfirmBtn.addEventListener('click', () => {
            // If the callback returns false, it means validation failed, so don't hide the modal.
            if (typeof modalConfirmCallback === 'function') {
                const result = modalConfirmCallback();
                if (result !== false) {
                    hideModal();
                }
            } else {
                hideModal();
            }
        });
    }
    if (modalOverlay) {
        modalOverlay.addEventListener('click', (event) => {
            if (event.target === modalOverlay) hideModal();
        });
    }
}

/**
 * Creates a button element with specified text, click handler, and options.
 * @param {string} text - The text content of the button.
 * @param {function} onClick - The function to call when the button is clicked.
 * @param {Object} options - Optional parameters for the button.
 *                           Expected: { id, classNames (array), disabled (boolean), title, marginRight, marginLeft }
 * @returns {HTMLButtonElement} The created button element.
 */
export function createButton(text, onClick, options = {}) { // Already exported, but good to confirm
    const button = document.createElement('button');
    button.textContent = text;
    if (options.id) button.id = options.id;
    if (options.classNames && Array.isArray(options.classNames)) {
        button.classList.add(...options.classNames);
    } else {
        button.classList.add('secondary'); // Default class if not specified
    }
    if (options.disabled) button.disabled = true;
    if (options.title) button.title = options.title;
    if (options.marginRight) button.style.marginRight = options.marginRight;
    if (options.marginLeft) button.style.marginLeft = options.marginLeft;

    button.addEventListener('click', onClick);
    return button;
}

/**
 * Creates a select element with specified options.
 * @param {Array<Object>} optionsArray - Array of objects { value, text } for select options.
 * @param {string|number} selectedValue - The value that should be pre-selected.
 * @param {function} onChange - The function to call when the select value changes.
 * @param {Object} options - Optional parameters for the select element.
 *                           Expected: { id, classNames (array), marginLeft }
 * @returns {HTMLSelectElement} The created select element.
 */
export function createSelect(optionsArray, selectedValue, onChange, options = {}) { // Already exported, but good to confirm
    const select = document.createElement('select');
    if (options.id) select.id = options.id;
    if (options.classNames && Array.isArray(options.classNames)) select.classList.add(...options.classNames);
    if (options.marginLeft) select.style.marginLeft = options.marginLeft;

    optionsArray.forEach(opt => select.add(new Option(opt.text, opt.value, false, String(opt.value) === String(selectedValue))));
    select.addEventListener('change', onChange);
    return select;
}
