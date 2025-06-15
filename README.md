# Bug Hunter Toolkit (bhtoolkit)

```
        .       .       .       .       .       .       .       .
    .       .       * .       .       * .       .    .      .   
                .       .       .       .       .       .

      ____  _   _ _______  ____   ____   _    _  __ _____ _______ 
     |  _ \| | | |__   __|/ __ \ / __ \ | |  | |/ /|_   _|__   __|
     | |_) | |_| |  | |  | |  | | |  | || |  | ' /   | |    | |   
     |  _ <|  _  |  | |  | |  | | |  | || |  |  <    | |    | |   
     | |_) | | | |  | |  | |__| | |__| || |__| . \  _| |_   | |   
     |____/|_| |_|  |_|   \____/ \____/ |______|\_\|_____|  |_|   

        .       .       * .       .       * .       .
    .       .       .       .       .       .       .       .       *
                .       Bug Hunter Toolkit      .       .
    .       .       .       .       .       .       .       .

```  

The Bug Hunter Toolkit is a web application designed to assist bug bounty hunters and penetration testers in managing their targets, scope, notes, checklists, and proxy logs.

## Features

*   Platform and Target Management
*   Scope Rule Definition (In-scope, Out-of-scope per target)
*   Target-specific Checklists (customizable from templates)
*   Target-specific Notes (Markdown supported)
*   HTTP Proxy Log Viewer and Analysis
    *   Detailed request/response view
    *   JavaScript Link Extraction
    *   Comment Finder
    *   Parameterized URL Analysis (identifying unique URL structures with varying parameters)
*   Request Modifier (send custom/modified HTTP requests)
*   Sitemap Generation (from proxy logs and manual additions)
*   Page Sitemap (manual page recording & log association for user journey mapping)
*   Domains Management
    *   Manual domain/subdomain tracking per target
    *   Subfinder Integration for automated subdomain discovery
*   Findings Management
*   Synack Integration (optional, for Synack Red Team members)
*   Graph Visualizations (Sitemap, Page Sitemap)
*   Settings Management (UI preferences, proxy exclusions, table layouts)
*   SQLite backend with database migrations
*   Chi-based router for the Go backend API
*   Vanilla JavaScript frontend

## Prerequisites

*   **Go:** Version 1.20 or higher (check with `go version`)
*   **Git:** For cloning the repository (if applicable)
*   **A modern web browser**
*   **Subfinder (Optional but Recommended):** For automated subdomain discovery. Install from ProjectDiscovery's GitHub.

## Setup & Installation

1.  **Clone the Repository (if applicable):**
    ```bash
    git clone <repository_url> bhtoolkit
    cd bhtoolkit
    ```
    If you already have the code, navigate to the project root directory.

2.  **Build the Application:**
    From the project root directory, run the build script:
    *(This script also handles database migrations.)*
    ```bash
    ./run_toolkit.sh
    ```
    This script compiles the Go backend and creates an executable named `toolkit` in the current directory.

3.  **Install Subfinder (Optional):**
    If you plan to use the automated subdomain discovery feature, ensure `subfinder` is installed and accessible in your system's PATH.
    ```bash
    go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
    ```
4.  **Initial Run & Database Setup:**
    The first time you run the application, it will initialize the SQLite database and apply migrations.
    ```bash
    ./run_toolkit.sh start
    ```
    This will start the web server, typically on `http://localhost:8778`.

## Configuration

The application uses a configuration file located at `~/.config/toolkit/config.yaml` (on Linux/macOS).
The default configuration includes:
*   **Database Path:** `~/.config/toolkit/bountytool.db`
*   **Log Paths:** `~/.config/toolkit/logs/app.log` and `~/.config/toolkit/logs/proxy.log`  
*   **API Server Address:** `:8778`
*   **Proxy Server Address:** `:8088` (for the HTTP proxy feature)
*   **Proxy Certificate Path:** `~/.config/toolkit/certs/proxy_cert.pem`
*   **Proxy Key Path:** `~/.config/toolkit/certs/proxy_key.pem`

You can modify these settings by editing the `config.yaml` file. The application will create a default configuration if one doesn't exist. The `certs` directory will also be created if it doesn't exist.  

### Proxy Certificate Setup (Crucial for HTTPS Interception)

To intercept and analyze HTTPS traffic, the toolkit's proxy needs a Certificate Authority (CA) certificate that your browser/system trusts.  

**1. Generate CA Certificate and Key:**
   The toolkit provides a command to generate the necessary CA certificate and key. Run the following from the project root:  
   `./run_toolkit.sh proxy init-ca`

   This command creates `proxy_cert.pem` and `proxy_key.pem` specifically for "Toolkit Proxy CA" and makes them valid for `localhost`.

   Alternatively, you can use OpenSSL, but it's more complex to generate a CA and sign a certificate for `localhost`.

**2. Configure Toolkit:**
   Ensure your `~/.config/toolkit/config.yaml` points to these generated files:
   ```yaml
   proxy:
     address: ":8088"
     # ... other proxy settings
     proxy_cert_path: "~/.config/toolkit/certs/proxy_cert.pem"
     proxy_key_path: "~/.config/toolkit/certs/proxy_key.pem"
   ```
   The toolkit will expand `~` to your home directory.

**3. Trust the CA Certificate:**
   *   For the generated `proxy_cert.pem` to be trusted as a CA itself (if you didn't use `mkcert -install`), you'll need to import `proxy_cert.pem` (or the `mkcert` root CA certificate, usually found via `mkcert -CAROOT`) into your browser's and/or operating system's list of trusted root certificate authorities. The process varies by browser and OS.
   *   **Firefox:** Preferences -> Privacy & Security -> Certificates -> View Certificates -> Authorities -> Import...
   *   **Chrome/Edge (on macOS/Windows):** Often use the system's trust store. Import into Keychain Access (macOS) or Certificate Manager (Windows - `certmgr.msc`).
   *   **Linux:** Consult your distribution's documentation for adding trusted CA certificates (e.g., `update-ca-certificates`).

## Usage

1.  **Start the Toolkit:**
    ```bash
    ./run_toolkit.sh start
    ```
2.  **Access the Web UI:**
    Open your web browser and navigate to `http://localhost:8778`.

3.  **Using the Interface:**
    *   **Platforms:** Create platforms (e.g., HackerOne, Bugcrowd, Private, Synack, Local).
    *   **Targets:** Add targets under platforms, define their codename, link, and initial scope.
    *   **Current Target:** Set a target as "current" to focus proxy logging, checklists, and notes.
    *   **Proxy Log:** Configure your browser or tools to use the toolkit's proxy (default: `http://localhost:8088` after setting up the CA certificate). View, search, and analyze HTTP traffic. Perform JS analysis, find comments, and analyze URL parameters.
    *   **Modifier:** Send selected requests from the proxy log or create new ones to modify and resend HTTP requests.
    *   **Page Sitemap:** Record user journeys by starting/stopping page recordings, which automatically associates relevant proxy logs.
    *   **Domains:** Manage domains and subdomains for the current target. Use Subfinder integration to discover new subdomains.
    *   **Checklists:** Use predefined checklist templates or create custom checklist items for each target (OWASP Juice Shop challenges are pre-defined).
    *   **Notes:** Keep notes specific to each target.
    *   **Findings:** Record security findings for targets.
    *   **Visualizer:** View graphical representations of your sitemap and page sitemap data.
    *   **Settings:** Configure UI preferences and global proxy exclusion rules.

## Database

The application uses an SQLite database.
*   **Location:** By default, `~/.config/toolkit/bountytool.db`.
*   **Migrations:** Database schema changes are managed using `golang-migrate`. Migration files are located in the `database/migrations` directory within the project.

## Development

*   Ensure Go is installed.
*   To rebuild after code changes, use `./run_toolkit.sh`.
*   Backend API routes are defined in `api/router/handlers/` and registered in `api/router.go`.
*   Frontend JavaScript modules are in the `static/views/` directory, with `static/app.js` as the main entry point.
