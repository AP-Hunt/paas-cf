#!/bin/sh

set -eu
SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
DEPLOYMENT_DIR=${DEPLOYMENT_DIR:-"${SCRIPT_DIR}"/deployments}

spruce merge \
  --prune meta \
  --prune secrets \
  --prune terraform_outputs \
  "${DEPLOYMENT_DIR}"/*.yml \
  "${DEPLOYMENT_DIR}"/aws/*.yml \
  "$@"

