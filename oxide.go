// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/libmachine/drivers"
	"github.com/rancher/machine/libmachine/log"
	"github.com/rancher/machine/libmachine/ssh"
	"github.com/rancher/machine/libmachine/state"
)

const (
	defaultSSHUser          = "oxide"
	defaultSSHPort          = 22
	defaultDescription      = "Managed by the Oxide Rancher machine driver."
	errRequiredOptionNotSet = "required option not set: "
)

// make sure Driver implements the drivers.Driver interface.
var _ drivers.Driver = &Driver{}

// Driver is the Oxide Rancher machine driver. Rancher uses this machine driver
// to provision instances on Oxide.
type Driver struct {
	// BaseDriver is embedded to provide common fields and methods as
	// recommended by the https://github.com/rancher/machine source code.
	*drivers.BaseDriver

	// Oxide silo domain name (e.g., https://silo01.oxide.example.com). This is
	// `OXIDE_HOST` when authenticating via the Oxide CLI.
	Host string

	// Oxide API token. This is `OXIDE_TOKEN` when authenticating via the Oxide CLI.
	Token string

	// Oxide project to create instances within.
	Project string

	// Number of vCPUs to give the instance.
	VCPUS int

	// Amount of memory, in bytes, to give the instance.
	Memory int

	// Size of the instance's boot disk, in bytes.
	BootDiskSize int

	// Image ID to use for the instance's boot disk.
	BootDiskImageID string

	// Size of hte instance's additional disk
	AdditionalDiskSizesStrings []string
	AdditionalDisks            []AdditionalDisk

	// VPC for the instance.
	VPC string

	// Subnet for the instance.
	Subnet string

	//
	UserDataFile string

	// Additional SSH public keys to inject into the instance.
	AdditionalSSHPublicKeyIDs []string

	// ID of the created instance. Used to retrieve instance state during
	// `GetState` and to delete the instance during `Remove`.
	InstanceID string

	// ID of the instance's boot disk. Used to delete the boot disk during
	// `Remove`.
	BootDiskID string

	// IDs of the additional disks
	AdditionalDiskIDs []string

	// ID of the generated SSH public key that's injected into the instance.
	// Used to delete the SSH public key during `Remove`.
	SSHPublicKeyID string

	oxideClient *oxide.Client
}

// newDriver creates a new Oxide rancher machine driver.
func newDriver(machineName, storePath string) *Driver {
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: machineName,
			SSHUser:     defaultSSHUser,
			SSHPort:     defaultSSHPort,
			StorePath:   storePath,
		},
	}
}

// Create creates the instance and any necessary dependencies (e.g., SSH keys,
// disks) and updates the machine driver with state for use by other methods.
// Create must start the instance otherwise the machine driver will time out
// waiting for the instance to start.
func (d *Driver) Create() error {
	ctx := context.TODO()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			log.Errorf("Failed to create Oxide client: %v", err)
			return err
		}
		d.oxideClient = client
	}
	log.Debug("Created Oxide client")

	pubKey, err := d.createSSHKeyPair()
	log.Debug("Created SSH public key")
	if err != nil {
		log.Errorf("Failed to create SSH public key: %v", err)
		return err
	}

	d.SSHPublicKeyID = pubKey.Id

	sshPublicKeyIDs := []oxide.NameOrId{
		oxide.NameOrId(d.SSHPublicKeyID),
	}
	for _, additionalSSHPublicKeyID := range d.AdditionalSSHPublicKeyIDs {
		sshPublicKeyIDs = append(sshPublicKeyIDs, oxide.NameOrId(additionalSSHPublicKeyID))
	}

	var userData []byte
	if d.UserDataFile != "" {
		b, err := os.ReadFile(d.UserDataFile)
		if err != nil {
			return err
		}
		userData = b
	}

	startInstance := false
	icp := oxide.InstanceCreateParams{
		Project: oxide.NameOrId(d.Project),
		Body: &oxide.InstanceCreate{
			Start: &startInstance,
			BootDisk: &oxide.InstanceDiskAttachment{
				Description: defaultDescription,
				DiskSource: oxide.DiskSource{
					Type:    oxide.DiskSourceTypeImage,
					ImageId: d.BootDiskImageID,
				},
				Name: oxide.Name("disk-" + d.GetMachineName()),
				Size: oxide.ByteCount(d.BootDiskSize),
				Type: oxide.InstanceDiskAttachmentTypeCreate,
			},
			Disks:       []oxide.InstanceDiskAttachment{},
			Description: defaultDescription,
			Hostname:    oxide.Hostname(d.GetMachineName()),
			Memory:      oxide.ByteCount(d.Memory),
			Name:        oxide.Name(d.GetMachineName()),
			Ncpus:       oxide.InstanceCpuCount(d.VCPUS),
			NetworkInterfaces: oxide.InstanceNetworkInterfaceAttachment{
				Params: []oxide.InstanceNetworkInterfaceCreate{
					{
						Description: defaultDescription,
						Name:        oxide.Name("nic-" + d.GetMachineName()),
						SubnetName:  oxide.Name(d.Subnet),
						VpcName:     oxide.Name(d.VPC),
					},
				},
				Type: oxide.InstanceNetworkInterfaceAttachmentTypeCreate,
			},
			SshPublicKeys: sshPublicKeyIDs,
			UserData:      base64.StdEncoding.EncodeToString(userData),
		},
	}

	d.AdditionalDiskIDs = make([]string, 0)
	for i, additionalDisk := range d.AdditionalDisks {
		diskName := additionalDisk.DiskName(d.GetMachineName(), i)
		log.Errorf("create disk %s", diskName)
		disk, err := d.oxideClient.DiskCreate(ctx, oxide.DiskCreateParams{
			Project: oxide.NameOrId(d.Project),
			Body: &oxide.DiskCreate{
				Description: defaultDescription,
				DiskSource: oxide.DiskSource{
					BlockSize: oxide.BlockSize(512),
					Type:      oxide.DiskSourceTypeBlank,
				},
				Name: diskName,
				Size: oxide.ByteCount(additionalDisk.Size),
			},
		})
		if err != nil {
			log.Errorf("Error creating disk %s: %v", diskName, err)
			return err
		}
		log.Infof("Created disk %s", diskName)

		d.AdditionalDiskIDs = append(d.AdditionalDiskIDs, disk.Id)
	}

	instance, err := d.oxideClient.InstanceCreate(ctx, icp)
	if err != nil {
		log.Errorf("Error creating instance: %v", err)
		return err
	}

	d.InstanceID = instance.Id
	d.BootDiskID = instance.BootDiskId

	log.Infof("boot disk id %s", d.BootDiskID)

	for _, diskId := range d.AdditionalDiskIDs {
		_, err = d.oxideClient.InstanceDiskAttach(ctx, oxide.InstanceDiskAttachParams{
			Instance: oxide.NameOrId(instance.Id),
			Body: &oxide.DiskPath{
				Disk: oxide.NameOrId(diskId),
			},
		})
		if err != nil {
			log.Errorf("Error attaching disk %s: %v", diskId, err)
			return err
		}
		log.Infof("Attached disk %s", diskId)
	}

	inilp := oxide.InstanceNetworkInterfaceListParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	networkInterfaces, err := d.oxideClient.InstanceNetworkInterfaceListAllPages(ctx, inilp)
	if err != nil {
		log.Errorf("Error listing network interfaces: %v", err)
		return err
	}

	if len(networkInterfaces) == 0 {
		err = errors.New("no valid network interfaces found")
		log.Errorf("Error listing network interfaces: %v", err)
		return err
	}
	d.IPAddress = networkInterfaces[0].Ip

	return d.Start()
}

// DriverName returns the name of this machine driver.
func (d *Driver) DriverName() string {
	return "oxide"
}

// GetSSHHostname returns the IP address or DNS name of the instance.
// This IP address or DNS name must be accessible from Rancher.
func (d *Driver) GetSSHHostname() (string, error) {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetIP()
}

// GetState fetches the current state of the instance and returns it as
// a standardized state representation that Rancher can understand.
func (d *Driver) GetState() (state.State, error) {
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return state.None, err
		}
		d.oxideClient = client
	}

	instance, err := d.oxideClient.InstanceView(ctx, oxide.InstanceViewParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		log.Errorf("Error getting instance state %s: %v", d.InstanceID, err)
		return state.None, err
	}

	return toRancherMachineState(instance.RunState), nil
}

// GetURL builds and returns a Docker-compatible URL that can be used to
// connect to the instance.
func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		log.Errorf("driver is not running: %v", err)
		return "", err
	}

	ip, err := d.GetIP()
	if err != nil {
		log.Errorf("Error getting IP: %v", err)
		return "", err
	}

	u := url.URL{
		Scheme: "tcp",
		Host:   net.JoinHostPort(ip, "2376"),
	}

	return u.String(), nil
}

// Kill forcefully stops the instance but does not remove it.
func (d *Driver) Kill() error {
	return d.Stop()
}

// PreCreateCheck performs necessary driver validation before creating any
// instance.
func (d *Driver) PreCreateCheck() (err error) {
	if d.UserDataFile != "" {
		if _, err = os.Stat(d.UserDataFile); os.IsNotExist(err) {
			err = fmt.Errorf("user data file %s could not be found", d.UserDataFile)
		}
	}

	return
}

// Remove stops and removes the instance and any dependencies so that
// they no longer exist in Oxide.
func (d *Driver) Remove() error {
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	if err := d.Stop(); err != nil {
		log.Errorf("Error stopping instance: %v", err)
		return err
	}

	// The instance cannot be deleted until it's stopped. Wait for it to stop.
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	for {
		select {
		case <-stopCtx.Done():
			err := fmt.Errorf("timed out waiting for instance to stop: %w", stopCtx.Err())
			log.Errorf("Error stopping instance: %v", err)
			return err
		default:
		}

		currentState, err := d.GetState()
		if err != nil {
			log.Errorf("Error getting instance state: %v", err)
			return err
		}

		if currentState == state.Stopped {
			break
		}
	}

	if err := d.oxideClient.CurrentUserSshKeyDelete(ctx, oxide.CurrentUserSshKeyDeleteParams{
		SshKey: oxide.NameOrId(d.SSHPublicKeyID),
	}); err != nil {
		log.Errorf("Error deleting ssh key: %v", err)
		return err
	}

	if err := d.oxideClient.InstanceDelete(ctx, oxide.InstanceDeleteParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}); err != nil {
		log.Errorf("Error deleting instance: %v", err)
		return err
	}

	if err := d.oxideClient.DiskDelete(ctx, oxide.DiskDeleteParams{
		Disk: oxide.NameOrId(d.BootDiskID),
	}); err != nil {
		log.Errorf("Error deleting disk: %v", err)
		return err
	}

	for _, diskID := range d.AdditionalDiskIDs {
		log.Infof("diskID to delete: (%s)", diskID)
		if err := d.oxideClient.DiskDelete(ctx, oxide.DiskDeleteParams{
			Disk: oxide.NameOrId(diskID),
		}); err != nil {
			log.Errorf("Error deleting disk %s: %v", diskID, err)
			return err
		}
	}

	return nil
}

// Restart restarts the instance without changing its configuration.
func (d *Driver) Restart() error {
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	log.Debug("Restarting instance")
	irp := oxide.InstanceRebootParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceReboot(ctx, irp); err != nil {
		log.Errorf("Error restarting instance: %v", err)
		return err
	}

	return nil
}

// Start starts the instance.
func (d *Driver) Start() error {
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	log.Debug("starting instance")
	isp := oxide.InstanceStartParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceStart(ctx, isp); err != nil {
		log.Errorf("error starting instance: %v", err)
		return err
	}

	return nil
}

// Stop stops the instance.
func (d *Driver) Stop() error {
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	log.Debug("stopping instance")
	isp := oxide.InstanceStopParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceStop(ctx, isp); err != nil {
		log.Errorf("error stopping instance: %v", err)
		return err
	}

	return nil
}

// createSSHKeyPair creates a new SSH key pair, saves both the private and
// public key to the store path for the machine driver to use, and uploads the
// public key to Oxide to be injected into the instance.
func (d *Driver) createSSHKeyPair() (*oxide.SshKey, error) {
	log.Debug("creating ssh key")
	ctx := context.Background()

	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return nil, err
		}
		d.oxideClient = client
	}

	d.SSHKeyPath = d.GetSSHKeyPath()
	if err := ssh.GenerateSSHKey(d.SSHKeyPath); err != nil {
		return nil, err
	}

	b, err := os.ReadFile(d.SSHKeyPath + ".pub")
	if err != nil {
		return nil, err
	}

	cuscp := oxide.CurrentUserSshKeyCreateParams{
		Body: &oxide.SshKeyCreate{
			Description: defaultDescription,
			Name:        oxide.Name(d.GetMachineName()),
			PublicKey:   string(b),
		},
	}
	return d.oxideClient.CurrentUserSshKeyCreate(ctx, cuscp)
}

// toRancherMachineState converts an Oxide instance state to a Rancher machine
// state. The semantics of the Rancher machine state.State values are not well
// defined so the mappings are best effort based on reading the Rancher machine
// source code and other implementations.
func toRancherMachineState(instanceState oxide.InstanceState) state.State {
	switch instanceState {
	// oxide.InstanceStateRepairing describes an instance that is attempting to recover
	// from a failure so state.Starting seems like the best fit.
	case oxide.InstanceStateCreating, oxide.InstanceStateStarting, oxide.InstanceStateRebooting, oxide.InstanceStateRepairing:
		return state.Starting

	// oxide.InstanceStateMigrating describes an instance undergoing a live
	// migration between hypervisors. The instance is meant to be available for
	// the duration of the migration so state.Running seems like the best fit.
	case oxide.InstanceStateRunning, oxide.InstanceStateMigrating:
		return state.Running

	case oxide.InstanceStateStopping:
		return state.Stopping

	case oxide.InstanceStateStopped:
		return state.Stopped

	case oxide.InstanceStateFailed:
		return state.Error

	case oxide.InstanceStateDestroyed:
		return state.NotFound

	default:
		return state.None
	}
}
