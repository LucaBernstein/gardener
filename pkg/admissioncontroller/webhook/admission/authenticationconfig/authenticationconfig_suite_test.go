// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthenticationConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AdmissionController Webhook Admission AuthenticationConfiguration Suite")
}