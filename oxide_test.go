// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

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
				Entry("sshUser", []string{flagSSHUser}),
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
				Expect(err.Error()).To(ContainSubstring("required option \"oxide-ssh-user\" not set"))
			})
		})
	})

	Describe("PreCreateCheck", func() {
		It("should validate every anti-affinity group", func() {
			requestedGroups := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Query().Get("project")).To(Equal("project"))
				requestedGroups = append(requestedGroups, r.URL.Path)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.Project = "project"
			SUT.AntiAffinityGroups = []string{"data", "workers"}

			Expect(SUT.PreCreateCheck()).To(Succeed())
			Expect(requestedGroups).To(ConsistOf(
				"/v1/anti-affinity-groups/data",
				"/v1/anti-affinity-groups/workers",
			))
		})

		It("should validate an anti-affinity group ID without specifying a project", func() {
			const groupID = "4b92e59d-bd20-4a8a-8d78-a154411c06cd"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/v1/anti-affinity-groups/" + groupID))
				Expect(r.URL.Query()).NotTo(HaveKey("project"))
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.Project = "project"
			SUT.AntiAffinityGroups = []string{groupID}

			Expect(SUT.PreCreateCheck()).To(Succeed())
		})

		It("should identify a missing anti-affinity group", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.Project = "project"
			SUT.AntiAffinityGroups = []string{"missing"}

			err := SUT.PreCreateCheck()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`anti-affinity group "missing" does not exist in project "project"`))
		})
	})

	Describe("Remove", func() {
		It("should retry removal after a transient failure", func() {
			sshKeyExists := true
			instanceExists := true
			bootDiskExists := true
			additionalDiskExists := true
			instanceDeleteAttempts := 0

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
					instanceDeleteAttempts++
					if instanceDeleteAttempts == 1 {
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

			Expect(SUT.Remove()).To(Succeed())
			Expect(sshKeyExists).To(BeFalse())
			Expect(instanceExists).To(BeFalse())
			Expect(bootDiskExists).To(BeFalse())
			Expect(additionalDiskExists).To(BeFalse())
			Expect(instanceDeleteAttempts).To(Equal(2))

			Expect(SUT.Remove()).To(Succeed())
		})

		It("should keep polling after transient stop and state failures", func() {
			stopAttempts := 0
			stateAttempts := 0
			instanceDeleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/instance-id/stop":
					stopAttempts++
					http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id":
					stateAttempts++
					if stateAttempts <= retryAttempts+1 {
						http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
						return
					}
					_, _ = w.Write([]byte(`{"run_state":"stopped"}`))
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/instances/instance-id":
					instanceDeleted = true
					w.WriteHeader(http.StatusNoContent)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"

			Expect(SUT.Remove()).To(Succeed())
			Expect(stopAttempts).To(Equal(retryAttempts))
			Expect(stateAttempts).To(Equal(retryAttempts + 2))
			Expect(instanceDeleted).To(BeTrue())
		})

		It("should attempt every resource after permanent deletion failures", func() {
			deleteRequests := make(map[string]int)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/instance-id/stop":
					_, _ = w.Write([]byte("{}"))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id":
					_, _ = w.Write([]byte(`{"run_state":"stopped"}`))
				case r.Method == http.MethodDelete:
					deleteRequests[r.URL.Path]++
					http.Error(w, "invalid request", http.StatusBadRequest)
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

			err := SUT.Remove()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed deleting SSH key"))
			Expect(err.Error()).To(ContainSubstring("failed deleting instance"))
			Expect(err.Error()).To(ContainSubstring("failed deleting boot disk"))
			Expect(err.Error()).To(ContainSubstring("failed deleting additional disk"))
			Expect(deleteRequests).To(HaveLen(4))
			for _, requests := range deleteRequests {
				Expect(requests).To(Equal(1))
			}
		})

		It("should recover missing additional disk IDs before deletion", func() {
			additionalDiskDeleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id/disks":
					_, _ = w.Write([]byte(`{"items":[{"id":"additional-disk-id","name":"disk-00-data-bob"}]}`))
				case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/instance-id/stop":
					_, _ = w.Write([]byte("{}"))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id":
					_, _ = w.Write([]byte(`{"run_state":"stopped"}`))
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/instances/instance-id":
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/disks/additional-disk-id":
					additionalDiskDeleted = true
					w.WriteHeader(http.StatusNoContent)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"
			SUT.additionalDisks = []additionalDisk{{label: "data"}}

			Expect(SUT.Remove()).To(Succeed())
			Expect(SUT.AdditionalDiskIDs).To(Equal([]string{"additional-disk-id"}))
			Expect(additionalDiskDeleted).To(BeTrue())
		})

		It("should preserve the instance when disk ID recovery fails", func() {
			stopCalled := false
			instanceDeleteCalled := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/instance-id/disks":
					http.Error(w, "invalid request", http.StatusBadRequest)
				case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/instance-id/stop":
					stopCalled = true
					_, _ = w.Write([]byte("{}"))
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/instances/instance-id":
					instanceDeleteCalled = true
					w.WriteHeader(http.StatusNoContent)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"
			SUT.additionalDisks = []additionalDisk{{label: "data"}}

			err := SUT.Remove()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed recording additional disks"))
			Expect(stopCalled).To(BeFalse())
			Expect(instanceDeleteCalled).To(BeFalse())
		})

		It("should bound retries with the provided context", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			attempts := 0
			start := time.Now()
			err := retry(ctx, func() error {
				attempts++
				<-ctx.Done()
				return ctx.Err()
			}, retryAttempts, time.Second)
			Expect(err).To(MatchError(context.DeadlineExceeded))
			Expect(time.Since(start)).To(BeNumerically("<", time.Second))
			Expect(attempts).To(Equal(1))
		})

	})

	Describe("recordAdditionalDiskIDs", func() {
		It("should record every disk except the boot disk", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/v1/instances/instance-id/disks"))
				_, _ = w.Write([]byte(`{"items":[{"id":"boot-disk-id"},{"id":"additional-disk-id","name":"disk-00-data-bob"},{"id":"unrelated-disk-id","name":"unrelated"}]}`))
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"
			SUT.BootDiskID = "boot-disk-id"
			SUT.additionalDisks = []additionalDisk{{label: "data"}}
			client, err := SUT.createOxideClient()
			Expect(err).NotTo(HaveOccurred())
			SUT.oxideClient = client

			Expect(SUT.recordAdditionalDiskIDs(context.Background())).To(Succeed())
			Expect(SUT.AdditionalDiskIDs).To(Equal([]string{"additional-disk-id"}))
		})

		It("should fail when not every configured disk is listed", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"items":[{"id":"boot-disk-id"}]}`))
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"
			SUT.BootDiskID = "boot-disk-id"
			SUT.additionalDisks = []additionalDisk{{}}
			client, err := SUT.createOxideClient()
			Expect(err).NotTo(HaveOccurred())
			SUT.oxideClient = client

			err = SUT.recordAdditionalDiskIDs(context.Background())
			Expect(err).To(MatchError("found 0 of 1 additional disks"))
		})

		It("should retain disk IDs across incomplete listings", func() {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					_, _ = w.Write([]byte(`{"items":[{"id":"additional-disk-1","name":"disk-00-one-bob"}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"items":[{"id":"additional-disk-2","name":"disk-01-two-bob"}]}`))
			}))
			defer server.Close()

			SUT.Host = server.URL
			SUT.Token = "token"
			SUT.InstanceID = "instance-id"
			SUT.BootDiskID = "boot-disk-id"
			SUT.additionalDisks = []additionalDisk{
				{label: "one"},
				{label: "two"},
			}
			client, err := SUT.createOxideClient()
			Expect(err).NotTo(HaveOccurred())
			SUT.oxideClient = client

			Expect(retry(context.Background(), func() error {
				return SUT.recordAdditionalDiskIDs(context.Background())
			}, retryAttempts, time.Millisecond)).To(Succeed())
			Expect(SUT.AdditionalDiskIDs).To(ConsistOf(
				"additional-disk-1",
				"additional-disk-2",
			))
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
	rv.Data[flagSSHUser] = "oxide"

	return rv
}
