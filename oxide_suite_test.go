// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
// Copyright 2024 Oxide Computer Company
package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRancherMachineDriverOxide(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RancherMachineDriverOxide Suite")
}
