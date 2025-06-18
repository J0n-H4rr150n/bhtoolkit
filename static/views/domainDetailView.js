// static/views/domainDetailView.js
import { escapeHtml } from '../utils.js';

let apiService;
let uiService;
let stateService;
let viewContentContainer;

export function initDomainDetailView(services) {
    apiService = services.apiService;
    uiService = services.uiService;
    stateService = services.stateService;
    console.log("[DomainDetailView] Initialized.");
}

export async function loadDomainDetailView(mainViewContainer, domainId) {
    viewContentContainer = mainViewContainer;
    if (!viewContentContainer) {
        console.error("[DomainDetailView] viewContentContainer not provided!");
        return;
    }
    if (!domainId) {
        viewContentContainer.innerHTML = `<p class="error-message">No domain ID provided.</p>`;
        uiService.updateBreadcrumbs([{ name: "Domain Detail" }, { name: "Error" }]);
        return;
    }

    viewContentContainer.innerHTML = `<p>Loading details for domain ID: ${domainId}...</p>`;

    try {
        // We'll need a new API service function: apiService.getDomainDetails(domainId)
        const domain = await apiService.getDomainDetails(domainId); // This function needs to be created

        const appState = stateService.getState();
        let platformName = "Platform"; // Default
        let targetName = domain.target_id ? `Target ${domain.target_id}` : "Unknown Target";

        // Attempt to get target and platform names for breadcrumbs
        if (domain.target_id) {
            try {
                const targetDetails = await apiService.getTargetDetails(domain.target_id);
                targetName = targetDetails.codename || targetName;
                if (targetDetails.platform_id) {
                    const platformDetails = await apiService.getPlatformDetails(targetDetails.platform_id);
                    platformName = platformDetails.name || platformName;
                }
            } catch (breadcrumbError) {
                console.warn("Error fetching details for breadcrumbs:", breadcrumbError);
            }
        }

        uiService.updateBreadcrumbs([
            { name: "Platforms", hash: "#platforms" },
            { name: platformName, hash: domain.target_id ? `#targets?platform_id=${domain.platform_id_placeholder || ''}` : "#platforms" }, // Placeholder for platform_id
            { name: targetName, hash: domain.target_id ? `#domains?target_id=${domain.target_id}` : "#targets" }, // Link to domains view for the target
            { name: `Domain: ${escapeHtml(domain.domain_name)}` }
        ]);
        document.title = `Domain: ${escapeHtml(domain.domain_name)} - Toolkit`;

        let httpxJsonContent = "No httpx data available.";
        if (domain.httpx_full_json && domain.httpx_full_json.Valid && domain.httpx_full_json.String) {
            try {
                const parsedJson = JSON.parse(domain.httpx_full_json.String);
                httpxJsonContent = `<pre>${escapeHtml(JSON.stringify(parsedJson, null, 2))}</pre>`;
            } catch (e) {
                httpxJsonContent = `<p class="error-message">Error parsing httpx JSON data.</p><pre>${escapeHtml(domain.httpx_full_json.String)}</pre>`;
            }
        }

        viewContentContainer.innerHTML = `
            <style>
                .domain-details-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 15px; margin-bottom: 20px; }
                .domain-details-grid > div { background-color: #f9f9f9; padding: 10px; border-radius: 4px; }
                body.dark-mode .domain-details-grid > div { background-color: #2c3a47; }
                .domain-details-grid p { margin-bottom: 5px; }
            </style>
            <h1>Domain: ${escapeHtml(domain.domain_name)}</h1>
            <div class="domain-details-grid">
                <div>
                    <p><strong>ID:</strong> ${domain.id}</p>
                    <p><strong>Target ID:</strong> ${domain.target_id || 'N/A'}</p>
                    <p><strong>Source:</strong> ${escapeHtml(domain.source?.String || 'N/A')}</p>
                    <p><strong>In Scope:</strong> ${domain.is_in_scope ? 'Yes' : 'No'}</p>
                    <p><strong>Notes:</strong> ${escapeHtml(domain.notes?.String || 'N/A')}</p>
                </div>
                <div>
                    <p><strong>Status Code:</strong> ${domain.http_status_code?.Valid ? domain.http_status_code.Int64 : '-'}</p>
                    <p><strong>Content Length:</strong> ${domain.http_content_length?.Valid ? domain.http_content_length.Int64 : '-'}</p>
                    <p><strong>Title:</strong> ${escapeHtml(domain.http_title?.String || '-')}</p>
                    <p><strong>Tech:</strong> ${escapeHtml(domain.http_tech?.String || '-')}</p>
                    <p><strong>Server:</strong> ${escapeHtml(domain.http_server?.String || '-')}</p>
                </div>
            </div>
            <p style="clear:both;"><strong>Created At:</strong> ${new Date(domain.created_at).toLocaleString()} | <strong>Updated At:</strong> ${new Date(domain.updated_at).toLocaleString()}</p>

            <div class="tabs" style="margin-top: 20px;">
                <button class="tab-button active" data-tab="httpxTab">HTTPX Data</button>
                <!-- Add more tabs here as needed -->
            </div>

            <div id="httpxTab" class="tab-content active">
                <h3>HTTPX Full JSON Output</h3>
                ${httpxJsonContent}
            </div>
            
        `;

        viewContentContainer.querySelectorAll('.tabs .tab-button').forEach(button => {
            button.addEventListener('click', (event) => {
                const tabId = event.target.dataset.tab;
                viewContentContainer.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
                viewContentContainer.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
                event.target.classList.add('active');
                viewContentContainer.querySelector(`#${tabId}`).classList.add('active');
            });
        });

    } catch (error) {
        console.error("Error loading domain details:", error);
        viewContentContainer.innerHTML = `<p class="error-message">Error loading domain details: ${escapeHtml(error.message)}</p>`;
        uiService.updateBreadcrumbs([{ name: "Domain Detail" }, { name: "Error" }]);
    }
}