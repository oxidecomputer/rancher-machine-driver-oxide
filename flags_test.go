// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/machine/commands/commandstest"
)

var _ = Describe("Driver", func() {
	var SUT *Driver
	var opts *commandstest.FakeFlagger

	BeforeEach(func() {
		SUT = newDriver("bob", "path")
		opts = defaultMockDriverOptions()
	})

	Describe("SetConfigFromFlags", func() {
		It("should succeed when all required fields are given", func() {
			Expect(SUT.SetConfigFromFlags(opts)).To(Succeed())
		})

		Describe("errors", func() {
			DescribeTable("should fail when a required string field is missing",
				func(fields []string) {
					for _, field := range fields {
						opts.Data[field] = ""
					}
					err := SUT.SetConfigFromFlags(opts)
					Expect(err).To(HaveOccurred())
				},
				Entry("host", []string{flagHost}),
				Entry("token", []string{flagToken}),
				Entry("project", []string{flagProject}),
				Entry("diskImageId", []string{flagBootDiskImageID}),
			)

			It("should fail when nothing is given", func() {
				err := SUT.SetConfigFromFlags(&commandstest.FakeFlagger{
					Data: map[string]interface{}{},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-host"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-token"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-project"))
				Expect(err.Error()).To(ContainSubstring("required option not set: oxide-boot-disk-image-id"))
			})
		})

		Describe("Set additional disk sizes", func() {
			It("should ingest additional disks with default name and duplicates", func() {
				opts.Data[flagAdditionalDisksSizes] = []string{
					"20 MiB",
					"30 Gib,longhorn",
					"40 GB,minio",
					"1234,minio",
				}

				Expect(SUT.SetConfigFromFlags(opts)).To(Succeed())
				Expect(SUT.AdditionalDiskSizesStrings).To(Equal([]string{
					"20 MiB",
					"30 Gib,longhorn",
					"40 GB,minio",
					"1234,minio",
				}))
				Expect(SUT.AdditionalDisks).To(Equal([]AdditionalDisk{
					{
						Label: "additional",
						Size:  20971520,
					},
					{
						Label: "longhorn",
						Size:  32212254720,
					},
					{
						Label: "minio",
						Size:  40000000000,
					},
					{
						Label: "minio",
						Size:  1234,
					},
				}))
			})
		})
	})
})

func defaultMockDriverOptions() (rv *commandstest.FakeFlagger) {
	rv = &commandstest.FakeFlagger{
		Data: map[string]interface{}{},
	}

	rv.Data[flagHost] = "host"
	rv.Data[flagToken] = "token"
	rv.Data[flagProject] = "project"
	rv.Data[flagBootDiskImageID] = "image"
	rv.Data[flagMemory] = "4GiB"
	rv.Data[flagBootDiskSize] = "20GiB"

	return rv
}
