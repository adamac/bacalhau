#!/usr/bin/env bash

set -euo pipefail

PATH_TO_PROJECT_ROOT=$(git rev-parse --show-toplevel)
SWAGGER_DIR="${PATH_TO_PROJECT_ROOT}/pkg/swagger"
WEBUI_PATH="${PATH_TO_PROJECT_ROOT}/webui/lib/api/schema"
cd "${PATH_TO_PROJECT_ROOT}" || exit

echo "Currently executing in ${PWD}"

# Check if 'swag' is installed and install it if not
if ! command -v swag &> /dev/null; then
    echo "'swag' not found. Installing 'swag'..."
    go install github.com/swaggo/swag/cmd/swag@latest
    if [[ $? -ne 0 ]]; then
        echo "Failed to install 'swag'. Exiting."
        exit 1
    fi
    echo "'swag' installed successfully."
fi

swag init \
--outputTypes "go,json" \
--parseDependency \
--parseInternal \
--generalInfo "api.go" \
--overridesFile .swaggo \
--output "${SWAGGER_DIR}" \
--dir "pkg/publicapi,pkg/models,pkg/config/types"

echo "swagger.json generated - moving from ${SWAGGER_DIR} to ${WEBUI_PATH}"

# See if the path exists, if not create it
if [[ ! -d "${WEBUI_PATH}" ]]; then
    mkdir -p "${WEBUI_PATH}"
fi
cp "${SWAGGER_DIR}/swagger.json" "${WEBUI_PATH}"

echo "swagger.json copied to ${WEBUI_PATH}/swagger.json"
