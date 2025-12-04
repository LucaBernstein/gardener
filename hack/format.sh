#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

source $REPO_ROOT/hack/tools/mod/aliases.sh

echo "> Format"

goimports -l -w $@

# Format import order only after files have been formatted by imports.
echo "> Format Import Order"

goimports_reviser_opts=${GOIMPORTS_REVISER_OPTIONS:-""}

for p in "$@" ; do
  goimports-reviser $goimports_reviser_opts -recursive $p
done
