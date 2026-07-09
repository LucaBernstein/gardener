// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package tools allows importing scripts in this directory in other projects via go mod dependencies.
package tools

// Import controller-runtime to get version from this go.mod (synced via go.work).
import _ "sigs.k8s.io/controller-runtime"
