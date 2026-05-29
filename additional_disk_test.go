package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oxidecomputer/oxide.go/oxide"
)

var _ = Describe("AdditionalDisks", func() {
	Context("createInstanceDiskAttachment", func() {
		It("should return a valid local disk attachment", func() {
			aDisk := additionalDisk{
				size:    8675309,
				label:   "the",
				backend: oxide.DiskBackendTypeLocal,
			}

			actual := aDisk.createInstanceDiskAttachment(1, "number")
			attachment, ok := actual.Value.(*oxide.InstanceDiskAttachmentCreate)
			Expect(ok).To(BeTrue())

			Expect(attachment.Name).To(Equal(oxide.Name("disk-01-the-number")))
			Expect(attachment.Size).To(Equal(oxide.ByteCount(8675309)))
			Expect(attachment.Description).To(Equal(defaultDescription))

			_, ok = attachment.DiskBackend.Value.(*oxide.DiskBackendLocal)
			Expect(ok).To(BeTrue())
		})

		It("should return a valid distributed disk attachment", func() {
			aDisk := additionalDisk{
				size:    8675309,
				label:   "the",
				backend: oxide.DiskBackendTypeDistributed,
			}

			actual := aDisk.createInstanceDiskAttachment(1, "number")
			attachment, ok := actual.Value.(*oxide.InstanceDiskAttachmentCreate)
			Expect(ok).To(BeTrue())

			Expect(attachment.Name).To(Equal(oxide.Name("disk-01-the-number")))
			Expect(attachment.Size).To(Equal(oxide.ByteCount(8675309)))
			Expect(attachment.Description).To(Equal(defaultDescription))

			backend, ok := attachment.DiskBackend.Value.(*oxide.DiskBackendDistributed)
			Expect(ok).To(BeTrue())

			source, ok := backend.DiskSource.Value.(*oxide.DiskSourceBlank)
			Expect(source.BlockSize).To(Equal(oxide.BlockSize(4096)))
		})
	})

	Context("name", func() {
		DescribeTable("should return a valid name (diskNumber as 1 and machineName as machine)",
			func(expected string, aDisk additionalDisk) {
				Expect(aDisk.name("machine", 1)).To(Equal(expected))
			},
			Entry("default is distributed", "disk-01-label-machine", additionalDisk{
				label:   "label",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("local disk", "disk-01-label-machine", additionalDisk{
				label:   "label",
				backend: oxide.DiskBackendTypeLocal,
			}),
		)

		It("should truncate a long named disk - intelligently - local", func() {
			aDisk := additionalDisk{
				size:    8675309,
				label:   "additional",
				backend: oxide.DiskBackendTypeLocal,
			}
			Expect(aDisk.name("las01-sandbox-violet-control-plane-rqqxv-mphz5", 1)).
				To(SatisfyAll(
					HaveLen(63),
					Equal("disk-01-additional-las01-sandbox-violet-control-pla-rqqxv-mphz5"),
				))
		})

		It("should truncate a long named disk - intelligently - distributed", func() {
			aDisk := additionalDisk{
				size:    8675309,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}
			Expect(aDisk.name("las01-sandbox-violet-control-plane-rqqxv-mphz5", 1)).
				To(SatisfyAll(
					HaveLen(63),
					Equal("disk-01-additional-las01-sandbox-violet-control-pla-rqqxv-mphz5"),
				))
		})
	})

	Context("parseAdditionalDisk", func() {
		DescribeTable("Old way",
			func(s string, expected additionalDisk) {
				Expect(parseAdditionalDisk(s)).To(Equal(expected))
			},
			Entry("parses integer without suffix", "21474836480", additionalDisk{
				size:    21474836480,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix", "10GiB", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with space suffix", "10 GiB", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer without suffix and label", "21474836480,data", additionalDisk{
				size:    21474836480,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix and label", "10GiB,data", additionalDisk{
				size:    10737418240,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with space suffix and label", "10 GiB,data", additionalDisk{
				size:    10737418240,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer without suffix trailing comma", "21474836480,", additionalDisk{
				size:    21474836480,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix trailing comma", "10GiB,", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with no label", "10GiB,,local", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses valid distributed backend with no label", "10GiB,,distributed", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with awesome label", "10GiB,awesome,local", additionalDisk{
				size:    10737418240,
				label:   "awesome",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses valid distributed backend with some label", "10GiB,some,distributed", additionalDisk{
				size:    10737418240,
				label:   "some",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			// extra spaces
			Entry("parses integer with suffix trailing comma", "10GiB , ", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with no label", "10GiB,  ,local", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses valid distributed backend with no label", "10GiB  ,,  distributed", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with awesome label", "10GiB , awesome , local  ", additionalDisk{
				size:    10737418240,
				label:   "awesome",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses valid distributed backend with some label", "10GiB  ,  some ,distributed  ", additionalDisk{
				size:    10737418240,
				label:   "some",
				backend: oxide.DiskBackendTypeDistributed,
			}),
		)

		DescribeTable("New way",
			func(s string, expected additionalDisk) {
				Expect(parseAdditionalDisk(s)).To(Equal(expected))
			},
			Entry("parses integer without suffix", "size=21474836480", additionalDisk{
				size:    21474836480,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix", "size=10GiB", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with space suffix", "size=10 GiB", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer without suffix and label", "size=21474836480,label=data", additionalDisk{
				size:    21474836480,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer without suffix and label - order", "label=data,size=21474836480", additionalDisk{
				size:    21474836480,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix and label", "size=10GiB,label=data", additionalDisk{
				size:    10737418240,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer and extra `,`", "size=21474836480,", additionalDisk{
				size:    21474836480,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with suffix trailing comma", "size=10GiB", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with no label", "size=10GiB,type=local", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses valid distributed backend with no label", "size=10GiB,,type=distributed", additionalDisk{
				size:    10737418240,
				label:   "additional",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses valid local backend with awesome label", "size=10GiB,label=awesome,type=local", additionalDisk{
				size:    10737418240,
				label:   "awesome",
				backend: oxide.DiskBackendTypeLocal,
			}),
			Entry("parses spaces", "type = distributed , label = some , size =  10GiB,", additionalDisk{
				size:    10737418240,
				label:   "some",
				backend: oxide.DiskBackendTypeDistributed,
			}),
			Entry("parses integer with space suffix and label - ignore extra", "size=10 GiB,label=data, ", additionalDisk{
				size:    10737418240,
				label:   "data",
				backend: oxide.DiskBackendTypeDistributed,
			}),
		)

		DescribeTable("Errors",
			func(s string, errStr string) {
				_, err := parseAdditionalDisk(s)
				Expect(err).To(SatisfyAll(
					HaveOccurred(),
					MatchError(ContainSubstring(errStr)),
				))
			},
			Entry("errors with empty string", "", "invalid format empty string given"),
			Entry("errors with empty invalid format", ",", "failed parsing size"),
			Entry("errors with no size", ",foo", "failed parsing size"),
			Entry("errors with invalid size unit suffix", "20 ABC,", "failed parsing size"),
			Entry("errors with invalid backend", "20GiB,,backend", "invalid backend"),
			Entry("errors with too many args", "five,is,right,out,", "invalid format"),
			Entry("error don't mix your metaphors", "label=awesome,local", "invalid format \"local\""),
			Entry("error no size new way", "label=awesome", "failed parsing size \"\": strconv.ParseFloat"),
			Entry("error bad key new way", "bad=good?", `invalid additional disk part: "bad", expected "size", "label", or "type"`),
		)
	})
})
