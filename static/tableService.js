// static/tableService.js

let showModalMessageFunc;
let saveLayoutsToBackendFunc;
let getGlobalTableLayoutsFunc;
let updateGlobalTableLayoutsFunc;

let _isResizing = false; // Flag to track if a column resize is currently active

/**
 * Initializes the table service with necessary dependencies.
 * @param {Object} dependencies - Object containing functions.
 *                                Expected: showModalMessage, saveTableLayouts, getGlobalTableLayouts, updateGlobalTableLayouts.
 */
export function initTableService(dependencies) {
    showModalMessageFunc = dependencies.showModalMessage;
    saveLayoutsToBackendFunc = dependencies.saveTableLayouts;
    getGlobalTableLayoutsFunc = dependencies.getGlobalTableLayouts;
    updateGlobalTableLayoutsFunc = dependencies.updateGlobalTableLayouts;
    console.log("[TableService] Initialized.");
}

/**
 * Saves the current column widths for a given table to the application state and persists it to the backend.
 * @param {string} tableKey - A unique key identifying the table's layout (used for storing in global state).
 * @param {string} tableHeadId - The ID of the table's thead element from which to read column widths.
 * @param {string|number|null} currentPageSize - The current page size (items per page) to save for this table. (Optional)
 */
export function saveCurrentTableLayout(tableKey, tableHeadId, currentPageSize = null) {
    if (!getGlobalTableLayoutsFunc || !updateGlobalTableLayoutsFunc || !saveLayoutsToBackendFunc || !showModalMessageFunc) {
        console.error("TableService not initialized properly. Call initTableService with dependencies first.");
        if (showModalMessageFunc) showModalMessageFunc("Service Error", "Table service is not configured correctly to save layouts.");
        return;
    }

    const tableHead = document.getElementById(tableHeadId);
    if (!tableHead) {
        console.error(`Table head with ID '${tableHeadId}' not found for saving layout.`);
        if (showModalMessageFunc) showModalMessageFunc("Layout Error", `Could not find table header (ID: ${tableHeadId}) to save layout.`);
        return;
    }

    const currentGlobalLayouts = getGlobalTableLayoutsFunc();
    // Start with the existing layout for this table (which includes visibility flags and potentially old pageSize)
    // Perform a deep copy to avoid modifying the state directly before updateState is called.
    const existingTableLayout = currentGlobalLayouts[tableKey] ? JSON.parse(JSON.stringify(currentGlobalLayouts[tableKey])) : { columns: {}, pageSize: null };
    
    // Ensure 'columns' object exists
    if (!existingTableLayout.columns) {
        existingTableLayout.columns = {};
    }

    tableHead.querySelectorAll('th[data-col-key]').forEach(th => {
        const colKey = th.getAttribute('data-col-key');
        const currentWidth = th.style.width || getComputedStyle(th).width;
        
        // If a column is non-resizable, its width won't be changed by dragging.
        // So, currentWidth will be its default/rendered width, which is fine to save.
        // The makeTableColumnsResizable function handles not adding resizers to non-resizable columns.

        if (currentWidth && colKey) {
            // Ensure the column entry exists before trying to set width
            if (!existingTableLayout.columns[colKey]) {
                existingTableLayout.columns[colKey] = { visible: true }; // Default to visible if new
            }
            existingTableLayout.columns[colKey].width = currentWidth;
        }
    });

    if (currentPageSize !== null) {
        existingTableLayout.pageSize = parseInt(currentPageSize, 10);
    }

    const updatedGlobalLayouts = {
        ...currentGlobalLayouts,
        [tableKey]: existingTableLayout // Use the modified existing layout
    };

    updateGlobalTableLayoutsFunc(updatedGlobalLayouts);
    console.log(`[TableService] Layout for ${tableKey} updated in state:`, existingTableLayout);

    saveLayoutsToBackendFunc(updatedGlobalLayouts)
        .then(() => {
            if (showModalMessageFunc) showModalMessageFunc('Layout Saved', 'Table column widths have been saved.');
        })
        .catch(error => {
            console.error("Error saving table layouts to backend:", error);
            if (showModalMessageFunc) showModalMessageFunc('Save Error', `Failed to save table layouts: ${error.message}`);
        });
}

/**
 * Makes the columns of a table (identified by its thead ID) resizable via drag-and-drop.
 * @param {string} tableHeadId - The ID of the table's thead element.
 * @param {Object} columnDefinitions - Optional: Definitions for columns, including a 'nonResizable' flag.
 */
export function makeTableColumnsResizable(tableHeadId, columnDefinitions = {}) {
    const tableHead = document.getElementById(tableHeadId);
    if (!tableHead) {
        console.warn(`Table head with ID '${tableHeadId}' not found for making columns resizable.`);
        return;
    }

    const headers = Array.from(tableHead.querySelectorAll('th[data-col-key]'));

    headers.forEach((th, index) => {
        const colKey = th.getAttribute('data-col-key');

        // Skip adding resizer if column is marked nonResizable or it's the last header
        const colDef = columnDefinitions[colKey];
        if ((colDef && colDef.nonResizable) || (index === headers.length - 1 && !th.nextElementSibling)) {
            return;
        }

        let resizer = th.querySelector('.col-resizer');
        if (!resizer) {
            resizer = document.createElement('div');
            resizer.classList.add('col-resizer');
            th.appendChild(resizer);
            th.style.position = 'relative';
        }


        let startX, startWidth;

        const onMouseDown = (e) => {
            _isResizing = true; // Set the flag when resize starts
            e.preventDefault();
            e.stopPropagation(); // Prevent sort click on TH
            startX = e.pageX;
            startWidth = th.offsetWidth;
            

            document.documentElement.style.cursor = 'col-resize';
            th.classList.add('resizing');

            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup', onMouseUp);
        };
        
        const onMouseMove = (moveEvent) => {
            const newWidth = startWidth + (moveEvent.pageX - startX);
            if (newWidth > 30) {
                th.style.width = `${newWidth}px`;
            }
        };

        const onMouseUp = () => {
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
            document.documentElement.style.cursor = '';
            th.classList.remove('resizing');

            // IMPORTANT: Delay resetting _isResizing flag.
            // This allows any click event (which fires after mouseup) on the TH
            // to correctly see that a resize was in progress.
            setTimeout(() => { _isResizing = false; }, 0);

        };

        resizer.removeEventListener('mousedown', onMouseDown); // Remove old listener before adding
        resizer.addEventListener('mousedown', onMouseDown);
    });
}

/**
 * Returns true if a column resize operation is currently active.
 * @returns {boolean}
 */
export function getIsResizing() {
    return _isResizing;
}
