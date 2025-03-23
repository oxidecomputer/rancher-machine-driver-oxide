// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	"strconv"

	"github.com/dustin/go-humanize"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Size Helpers", func() {
	Describe("HumanizeSize", func() {
		DescribeTable("simple sizes",
			func(given string, expected int) {
				Expect(HumanizeSize(given)).To(Equal(expected))
			},
			Entry("1", "1", 1),
			Entry("20", "20", 20),
			Entry("1Kb", "1000", 1000),
			Entry("1Kib", "1024", humanize.KiByte),
			Entry("1PiB", "1PiB", humanize.PiByte),
			Entry("1PB", "1PB", humanize.PByte),
		)
		DescribeTable("errors",
			func(given string, expected string) {
				_, err := HumanizeSize(given)
				Expect(err).To(MatchError(ContainSubstring(expected)))
			},
			Entry("1PiB + 1", strconv.Itoa(humanize.PiByte+1), "invalid size definition, size too large"),
			Entry("borked", "borked", "invalid syntax"),
		)
	})
})
