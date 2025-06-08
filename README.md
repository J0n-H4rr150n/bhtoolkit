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
*   Scope Rule Definition (In-scope, Out-of-scope)
*   Target-specific Checklists (customizable from templates)
*   Target-specific Notes
*   HTTP Proxy Log Viewer and Analysis
*   Sitemap Generation (manual additions from proxy log)
*   Findings Management
*   Synack Integration (optional, for Synack Red Team members)
*   Settings Management (UI preferences, proxy exclusions)
*   SQLite backend with database migrations
*   Chi-based router for the Go backend API
*   Vanilla JavaScript frontend

## Prerequisites

*   **Go:** Version 1.20 or higher (check with `go version`)
*   **Git:** For cloning the repository (if applicable)
*   **A modern web browser**

## Setup & Installation

1.  **Clone the Repository (if applicable):**
    ```bash
    git clone <repository_url> bhtoolkit
    cd bhtoolkit
    ```
    If you already have the code, navigate to the project root directory.

2.  **Build the Application:**
    From the project root directory, run the build script:
    ```bash
    ./run_toolkit.sh
    ```
    This script compiles the Go backend and creates an executable named `toolkit` in the current directory.

3.  **Initial Run & Database Setup:**
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

You can modify these settings by editing the `config.yaml` file. The application will create a default configuration if one doesn't exist.

## Usage

1.  **Start the Toolkit:**
    ```bash
    ./run_toolkit.sh start
    ```
2.  **Access the Web UI:**
    Open your web browser and navigate to `http://localhost:8778`.

3.  **Using the Interface:**
    *   **Platforms:** Create platforms (e.g., HackerOne, Bugcrowd, Private).
    *   **Targets:** Add targets under platforms, define their codename, link, and initial scope.
    *   **Current Target:** Set a target as "current" to focus proxy logging, checklists, and notes.
    *   **Proxy Log:** Configure your browser or tools to use the toolkit's proxy (default: `http://localhost:8088`). View, search, and analyze HTTP traffic.
    *   **Sitemap:** Manually add interesting endpoints from the proxy log to a target's sitemap.
    *   **Checklists:** Use predefined checklist templates or create custom checklist items for each target.
    *   **Notes:** Keep notes specific to each target.
    *   **Findings:** Record security findings for targets.
    *   **Settings:** Configure UI preferences and global proxy exclusion rules.

## Paths  

## Database

The application uses an SQLite database.
*   **Location:** By default, `~/.config/toolkit/bountytool.db`.
*   **Migrations:** Database schema changes are managed using `golang-migrate`. Migration files are located in the `database/migrations` directory within the project.

## Development

*   Ensure Go is installed.
*   To rebuild after code changes, use `./run_toolkit.sh`.
*   Backend API routes are defined in `api/router/handlers/` and registered in `api/router.go`.
*   Frontend JavaScript modules are in the `static/views/` directory, with `static/app.js` as the main entry point.
