#!/bin/bash

# Navigate to the script's directory to ensure we are in the project root.
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR" || exit 1 # Exit if cd fails

echo "=== Rebuilding toolkit (to ensure swag parses up-to-date code)... ==="
# Build the project first to catch any compilation errors before generating docs
if go build -o toolkit .; then
    echo "=== Build successful. ==="
    echo ""
else
    echo ""
    echo "!!! Build failed. Please check errors above before generating docs. !!!"
    exit 1
fi

echo "=== Generating OpenAPI specification using swag... ==="

# Ensure swag is installed
if ! command -v swag &> /dev/null
then
    echo "swag command could not be found. Please install it first:"
    echo "go install github.com/swaggo/swag/cmd/swag@latest"
    exit 1
fi

# Run swag init
# -g: Go files entry point (main.go or where your main router/API annotations are)
#     We'll use api/docs.go as the primary entry for general API info.
# -o: Output directory for the generated docs
# --parseDependency:      Parse API definitions in dependency packages
# --parseInternal:        Parse API definitions in internal packages
# --parseDepth 5:         Dependency parsing depth (adjust as needed)
# --outputTypes yaml,json: Generate both YAML and JSON
# --instanceName "instance1" # Add an instance name if you have multiple swagger instances (not common for single API)

# Adjust the -g flag to point to the file containing your main API info annotations (@title, @version, etc.)
# This is now toolkit/api/docs.go
swag init -g api/docs.go -o ./docs --outputTypes yaml,json --parseDependency --parseInternal --parseDepth 5

if [ $? -eq 0 ]; then
    echo "=== OpenAPI specification generated successfully in ./docs directory. ==="
    echo "You can find swagger.yaml and swagger.json there."
else
    echo ""
    echo "!!! OpenAPI specification generation failed. Please check errors above. !!!"
    exit 1
fi

rm -rf /home/kali/.config/toolkit/swagger/*
