#!/usr/bin/env bash

# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

goimports() {
  go tool -modfile "$REPO_ROOT/hack/tools/mod/go.mod" goimports "$@"
}
export -f goimports

golangci-lint() {
  go tool -modfile "$REPO_ROOT/hack/tools/mod/go.mod" golangci-lint "$@"
}
export -f golangci-lint

goimports-reviser() {
  go tool -modfile "$REPO_ROOT/hack/tools/mod/go.mod" goimports-reviser "$@"
}
export -f goimports-reviser

gosec() {
  go tool -modfile "$REPO_ROOT/hack/tools/mod/go.mod" gosec "$@"
}
export -f gosec
