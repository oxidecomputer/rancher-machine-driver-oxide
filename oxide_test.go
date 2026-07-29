// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/commands/commandstest"
	"github.com/rancher/machine/libmachine/state"
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
					Data: map[string]any{},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("required option \"oxide-host\" not set"))
				Expect(err.Error()).To(ContainSubstring("required option \"oxide-token\" not set"))
				Expect(err.Error()).To(ContainSubstring("required option \"oxide-project\" not set"))
				Expect(err.Error()).To(ContainSubstring("required option \"oxide-boot-disk-image-id\" not set"))
			})
		})
	})

	Describe("Remove", func() {
		It("should finish removing resources after a partial failure", func() {
			sshKeyExists := true
			instanceExists := true
			bootDiskExists := true
			additionalDiskExists := true
			failInstanceDelete := true

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/instance-id/stop":
					if !instanceExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					_, _ = w.Write([]byte("{}"))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id":
					if !instanceExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					_, _ = w.Write([]byte(`{"run_state":"stopped"}`))
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/me/ssh-keys/ssh-key-id":
					if !sshKeyExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					sshKeyExists = false
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/instances/instance-id":
					if failInstanceDelete {
						failInstanceDelete = false
						http.Error(w, "failed deleting instance", http.StatusInternalServerError)
						return
					}
					if !instanceExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					instanceExists = false
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/disks/boot-disk-id":
					if !bootDiskExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					bootDiskExists = false
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/disks/additional-disk-id":
					if !additionalDiskExists {
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
					additionalDiskExists = false
					w.WriteHeader(http.StatusNoContent)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.SSHPublicKeyID = "ssh-key-id"
			SUT.InstanceID = "instance-id"
			SUT.BootDiskID = "boot-disk-id"
			SUT.AdditionalDiskIDs = []string{"additional-disk-id"}

			Expect(SUT.Remove()).To(HaveOccurred())
			Expect(sshKeyExists).To(BeFalse())
			Expect(instanceExists).To(BeTrue())
			Expect(bootDiskExists).To(BeTrue())
			Expect(additionalDiskExists).To(BeTrue())

			Expect(SUT.Remove()).To(Succeed())
			Expect(instanceExists).To(BeFalse())
			Expect(bootDiskExists).To(BeFalse())
			Expect(additionalDiskExists).To(BeFalse())

			Expect(SUT.Remove()).To(Succeed())
		})
	})

	DescribeTable("RancherMachineState mapping is correct",
		func(instanceState oxide.InstanceState, expectedState state.State) {
			Expect(toRancherMachineState(instanceState)).To(Equal(expectedState))
		},
		Entry("creating", oxide.InstanceStateCreating, state.Starting),
		Entry("starting", oxide.InstanceStateStarting, state.Starting),
		Entry("running", oxide.InstanceStateRunning, state.Running),
		Entry("stopping", oxide.InstanceStateStopping, state.Stopping),
		Entry("stopped", oxide.InstanceStateStopped, state.Stopped),
		Entry("repairing", oxide.InstanceStateRepairing, state.Starting),
		Entry("rebooting", oxide.InstanceStateRebooting, state.Starting),
		Entry("migrating", oxide.InstanceStateMigrating, state.Running),
		Entry("failed", oxide.InstanceStateFailed, state.Error),
		Entry("destroyed", oxide.InstanceStateDestroyed, state.NotFound),
		Entry("unknown", oxide.InstanceState("unknown"), state.None),
	)
})

func defaultMockDriverOptions() (rv *commandstest.FakeFlagger) {
	rv = &commandstest.FakeFlagger{
		Data: map[string]any{},
	}

	rv.Data[flagHost] = "host"
	rv.Data[flagToken] = "token"
	rv.Data[flagProject] = "project"
	rv.Data[flagBootDiskImageID] = "image"

	return rv
}
