#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

GOFMT="goimports -w"

bad_files=$(git ls-files "*.go" | grep -v replace | grep -v vendor | xargs -I {} $GOFMT -l {})
if [[ -n "${bad_files}" ]]; then
  echo "FAIL: '$GOFMT' needs to be run on the following files: "
  echo "${bad_files}"
  echo "FAIL: please execute make goimports -w ${bad_files}"
  exit 1
fi
