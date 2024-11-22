// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Copyright 2024 Oxide Computer Company

// Package oxide contains the Oxide Rancher machine driver, also known as a
// node driver in Rancher parlance. Rancher uses this machine driver to
// provision instances on Oxide and install Kubernetes on those instances.
package oxide

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
	"github.com/rancher/machine/libmachine/mcnflag"
	"github.com/rancher/machine/libmachine/ssh"
	"github.com/rancher/machine/libmachine/state"
)

const (
	defaultSSHUser     = "oxide"
	defaultSSHPort     = 22
	defaultDescription = "Managed by the Oxide Rancher machine driver."
)

const (
	flagHost                      = "oxide-host"
	flagToken                     = "oxide-token"
	flagProject                   = "oxide-project"
	flagVCPUs                     = "oxide-vcpus"
	flagMemory                    = "oxide-memory"
	flagBootDiskSize              = "oxide-boot-disk-size"
	flagBootDiskImageID           = "oxide-boot-disk-image-id"
	flagVPC                       = "oxide-vpc"
	flagSubnet                    = "oxide-subnet"
	flagUserDataFile              = "oxide-user-data-file"
	flagSSHUser                   = "oxide-ssh-user"
	flagAdditionalSSHPublicKeyIDs = "oxide-additional-ssh-public-key-ids"
)

// Compile-time check to ensure Driver implements the drivers.Driver interface.
var _ drivers.Driver = new(Driver)

// Driver is the Oxide Rancher machine driver. Rancher uses this machine driver
// to provision instances on Oxide and install Kubernetes on those instances.
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

	// ID of the generated SSH public key that's injected into the instance.
	// Used to delete the SSH public key during `Remove`.
	SSHPublicKeyID string
}

// NewDriver creates a new Oxide rancher machine driver.
func NewDriver(machineName string, storePath string) *Driver {
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
	client, err := d.createOxideClient()
	if err != nil {
		return err
	}

	sshKey, err := d.configureSSHKey()
	if err != nil {
		return err
	}

	d.SSHPublicKeyID = sshKey.Id

	sshPublicKeyIDs := []oxide.NameOrId{oxide.NameOrId(d.SSHPublicKeyID)}
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

	instance, err := client.InstanceCreate(context.TODO(), oxide.InstanceCreateParams{
		Project: oxide.NameOrId(d.Project),
		Body: &oxide.InstanceCreate{
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
	})
	if err != nil {
		return err
	}

	d.InstanceID = instance.Id
	d.BootDiskID = instance.BootDiskId

	networkInterfaces, err := client.InstanceNetworkInterfaceListAllPages(context.TODO(), oxide.InstanceNetworkInterfaceListParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return err
	}

	// TODO: Don't assume the first network interface is the one to use.
	for _, networkInterface := range networkInterfaces {
		d.IPAddress = networkInterface.Ip
		break
	}

	return nil
}

// DriverName returns the name of this machine driver.
func (d *Driver) DriverName() string {
	return "oxide"
}

// GetCreateFlags configures the CLI flags for machine driver.
func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		// API authentication.
		mcnflag.StringFlag{
			Name:   flagHost,
			Usage:  "Oxide silo domain name (e.g., https://silo01.oxide.example.com). This is `OXIDE_HOST` when authenticating via the Oxide CLI.",
			EnvVar: "OXIDE_HOST",
		},
		mcnflag.StringFlag{
			Name:   flagToken,
			Usage:  "Oxide API token. This is `OXIDE_TOKEN` when authenticating via the Oxide CLI.",
			EnvVar: "OXIDE_TOKEN",
		},
		mcnflag.StringFlag{
			Name:   flagProject,
			Usage:  "Oxide project to create instances within.",
			EnvVar: "OXIDE_PROJECT",
		},

		// Instance hardware.
		mcnflag.IntFlag{
			Name:   flagVCPUs,
			Usage:  "Number of vCPUs to give the instance.",
			EnvVar: "OXIDE_VCPUS",
			Value:  2,
		},
		mcnflag.IntFlag{
			Name:   flagMemory,
			Usage:  "Amount of memory, in bytes, to give the instance.",
			EnvVar: "OXIDE_MEMORY",
			Value:  4294967296, // 4 GiB
		},

		// Boot disk.
		mcnflag.IntFlag{
			Name:   flagBootDiskSize,
			Usage:  "Size of the instance's boot disk, in bytes.",
			EnvVar: "OXIDE_BOOT_DISK_SIZE",
			Value:  21474836480, // 20 GiB
		},
		mcnflag.StringFlag{
			Name:   flagBootDiskImageID,
			Usage:  "Image ID to use for the instance's boot disk.",
			EnvVar: "OXIDE_BOOT_DISK_IMAGE_ID",
		},

		// Networking.
		mcnflag.StringFlag{
			Name:   flagVPC,
			Usage:  "VPC for the instance.",
			EnvVar: "OXIDE_VPC",
			Value:  "default",
		},
		mcnflag.StringFlag{
			Name:   flagSubnet,
			Usage:  "Subnet for the instance.",
			EnvVar: "OXIDE_SUBNET",
			Value:  "default",
		},

		// User data.
		mcnflag.StringFlag{
			Name:   flagUserDataFile,
			Usage:  "Path to a file containing instance user data",
			EnvVar: "OXIDE_USER_DATA_FILE",
		},

		// SSH information.
		mcnflag.StringFlag{
			Name:   flagSSHUser,
			Usage:  "User to SSH into the instance",
			EnvVar: "OXIDE_SSH_USER",
		},
		mcnflag.StringSliceFlag{
			Name:   flagAdditionalSSHPublicKeyIDs,
			Usage:  "Additional SSH public keys to inject into the instance.",
			EnvVar: "OXIDE_ADDITIONAL_SSH_PUBLIC_KEY_IDS",
		},
	}
}

// GetIP returns the IP address of the instance. This IP address must be
// accessible from Rancher.
func (d *Driver) GetIP() (string, error) {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetIP()
}

// GetMachineName returns the machine name that Rancher configured when it
// initialized this machine driver.
func (d *Driver) GetMachineName() string {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetMachineName()
}

// GetSSHHostname returns the IP address or DNS name of the instance.
// This IP address or DNS name must be accessible from Rancher.
func (d *Driver) GetSSHHostname() (string, error) {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetIP()
}

// GetSSHKeyPath returns the file path where the machine driver stores SSH
// keys.
func (d *Driver) GetSSHKeyPath() string {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetSSHKeyPath()
}

// GetSSHPort returns the port that Rancher will use to SSH into the instance.
func (d *Driver) GetSSHPort() (int, error) {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetSSHPort()
}

// GetSSHUsername returns the user that Rancher will use to SSH into the
// instance.
func (d *Driver) GetSSHUsername() string {
	// Use the embedded BaseDriver's logic.
	return d.BaseDriver.GetSSHUsername()
}

// GetState fetches the current state of the instance and returns it as
// a standardized state representation that Rancher can understand.
func (d *Driver) GetState() (state.State, error) {
	client, err := d.createOxideClient()
	if err != nil {
		return state.None, err
	}

	instance, err := client.InstanceView(context.TODO(), oxide.InstanceViewParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return state.None, err
	}

	return toRancherMachineState(instance.RunState), nil
}

// GetURL builds and returns a Docker-compatible URL that can be used to
// connect to the instance.
func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", err
	}

	ip, err := d.GetIP()
	if err != nil {
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
func (d *Driver) PreCreateCheck() error {
	if d.UserDataFile != "" {
		if _, err := os.Stat(d.UserDataFile); os.IsNotExist(err) {
			return fmt.Errorf("user data file %s could not be found", d.UserDataFile)
		}
	}
	return nil
}

// Remove stops and removes the instance and any dependencies so that
// they no longer exist in Oxide.
func (d *Driver) Remove() error {
	client, err := d.createOxideClient()
	if err != nil {
		return err
	}

	if err := d.Stop(); err != nil {
		return err
	}

	// The instance cannot be deleted until it's stopped. Wait for it to stop.
	stopCtx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	for {
		select {
		case <-stopCtx.Done():
			return fmt.Errorf("timed out waiting for instance to stop: %w", stopCtx.Err())
		default:
		}

		currentState, err := d.GetState()
		if err != nil {
			return err
		}

		if currentState == state.Stopped {
			break
		}
	}

	if err := client.CurrentUserSshKeyDelete(context.TODO(), oxide.CurrentUserSshKeyDeleteParams{
		SshKey: oxide.NameOrId(d.SSHPublicKeyID),
	}); err != nil {
		return err
	}

	if err := client.InstanceDelete(context.TODO(), oxide.InstanceDeleteParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}); err != nil {
		return err
	}

	if err := client.DiskDelete(context.TODO(), oxide.DiskDeleteParams{
		Disk: oxide.NameOrId(d.BootDiskID),
	}); err != nil {
		return err
	}

	return nil
}

// Restart restarts the instance without changing its configuration.
func (d *Driver) Restart() error {
	client, err := d.createOxideClient()
	if err != nil {
		return err
	}

	_, err = client.InstanceReboot(context.TODO(), oxide.InstanceRebootParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return err
	}

	return nil
}

// SetConfigFromFlags reads the CLI flags and sets necessary state on the
// driver for use by other methods.
func (d *Driver) SetConfigFromFlags(opts drivers.DriverOptions) error {
	d.Host = opts.String(flagHost)
	d.Token = opts.String(flagToken)
	d.Project = opts.String(flagProject)
	d.VCPUS = opts.Int(flagVCPUs)
	d.Memory = opts.Int(flagMemory)
	d.BootDiskSize = opts.Int(flagBootDiskSize)
	d.BootDiskImageID = opts.String(flagBootDiskImageID)
	d.VPC = opts.String(flagVPC)
	d.Subnet = opts.String(flagSubnet)
	d.UserDataFile = opts.String(flagUserDataFile)
	d.SSHUser = opts.String(flagSSHUser)
	d.AdditionalSSHPublicKeyIDs = opts.StringSlice(flagAdditionalSSHPublicKeyIDs)
	d.SSHPort = defaultSSHPort

	var err error

	if d.Host == "" {
		err = errors.Join(err, fmt.Errorf("required option not set: %s", flagHost))
	}

	if d.Token == "" {
		err = errors.Join(err, fmt.Errorf("required option not set: %s", flagToken))
	}

	if d.Project == "" {
		err = errors.Join(err, fmt.Errorf("required option not set: %s", flagProject))
	}

	if d.BootDiskImageID == "" {
		err = errors.Join(err, fmt.Errorf("required option not set: %s", flagBootDiskImageID))
	}

	return errors.Unwrap(err)
}

// Start starts the instance.
func (d *Driver) Start() error {
	client, err := d.createOxideClient()
	if err != nil {
		return err
	}

	_, err = client.InstanceStart(context.TODO(), oxide.InstanceStartParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return err
	}

	return nil
}

// Stop stops the instance.
func (d *Driver) Stop() error {
	client, err := d.createOxideClient()
	if err != nil {
		return err
	}

	_, err = client.InstanceStop(context.TODO(), oxide.InstanceStopParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return err
	}

	return nil
}

// createOxideClient creates a Oxide client from the machine driver
// configuration.
func (d *Driver) createOxideClient() (*oxide.Client, error) {
	return oxide.NewClient(&oxide.Config{
		Host:      d.Host,
		Token:     d.Token,
		UserAgent: "Oxide Rancher Machine Driver/0.0.1 (Go; Linux) [Environment: Development]",
	})
}

// configureSSHKey creates a new SSH key pair, saves both the private and
// public key to the store path for the machine driver to use, and uploads the
// public key to Oxide to be injected into the instance.
// TODO: Support an existing SSH key pair.
func (d *Driver) configureSSHKey() (*oxide.SshKey, error) {
	client, err := d.createOxideClient()
	if err != nil {
		return nil, err
	}

	d.SSHKeyPath = d.GetSSHKeyPath()

	if err := ssh.GenerateSSHKey(d.SSHKeyPath); err != nil {
		return nil, err
	}

	sshPublicKeyBytes, err := os.ReadFile(d.SSHKeyPath + ".pub")
	if err != nil {
		return nil, err
	}

	oxideSSHKey, err := client.CurrentUserSshKeyCreate(context.TODO(), oxide.CurrentUserSshKeyCreateParams{
		Body: &oxide.SshKeyCreate{
			Description: defaultDescription,
			Name:        oxide.Name(d.GetMachineName()),
			PublicKey:   string(sshPublicKeyBytes),
		},
	})
	if err != nil {
		return nil, err
	}

	return oxideSSHKey, nil
}

// toRancherMachineState converts an Oxide instance state to a Rancher machine
// state.
// TODO: Confirm mappings are correct for what Rancher expects.
func toRancherMachineState(instanceState oxide.InstanceState) state.State {
	switch instanceState {
	case oxide.InstanceStateCreating, oxide.InstanceStateStarting:
		return state.Starting
	case oxide.InstanceStateRunning, oxide.InstanceStateMigrating:
		return state.Running
	case oxide.InstanceStateStopping, oxide.InstanceStateRebooting:
		return state.Stopping
	case oxide.InstanceStateStopped, oxide.InstanceStateRepairing:
		return state.Stopped
	case oxide.InstanceStateFailed, oxide.InstanceStateDestroyed:
		return state.Error
	default:
		return state.None
	}
}