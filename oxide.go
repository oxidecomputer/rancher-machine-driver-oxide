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
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/oxidecomputer/oxide.go/oxide"
	"github.com/rancher/machine/libmachine/drivers"
	"github.com/rancher/machine/libmachine/mcnflag"
	"github.com/rancher/machine/libmachine/ssh"
	"github.com/rancher/machine/libmachine/state"
)

const (
	defaultSSHUser      = "oxide"
	defaultSSHPort      = 22
	defaultDescription  = "Managed by the Oxide Rancher machine driver."
	defaultMemory       = "4 GiB"
	defaultBootDiskSize = "20 GiB"
)

const (
	flagHost              = "oxide-host"
	flagToken             = "oxide-token"
	flagProject           = "oxide-project"
	flagVCPUs             = "oxide-vcpus"
	flagMemory            = "oxide-memory"
	flagBootDiskSize      = "oxide-boot-disk-size"
	flagBootDiskImageID   = "oxide-boot-disk-image-id"
	flagAdditionalDisk    = "oxide-additional-disk"
	flagVPC               = "oxide-vpc"
	flagSubnet            = "oxide-subnet"
	flagUserDataFile      = "oxide-user-data-file"
	flagSSHUser           = "oxide-ssh-user"
	flagSSHPublicKey      = "oxide-ssh-public-key"
	flagAntiAffinityGroup = "oxide-anti-affinity-group"
	flagEphemeralIPAttach = "oxide-ephemeral-ip-attach"
	flagEphemeralIPPool   = "oxide-ephemeral-ip-pool"
	flagUserAgent         = "oxide-user-agent"
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
	Memory uint64

	// Size of the instance's boot disk, in bytes.
	BootDiskSize uint64

	// Image ID to use for the instance's boot disk.
	BootDiskImageID string

	// VPC for the instance.
	VPC string

	// Subnet for the instance.
	Subnet string

	// Should an ephemeralIP be assigned to the instance
	EphemeralIPAttach bool

	// pool for the ephemeral IP
	EphemeralIPPool string

	// Path to file containing user data for the instance.
	UserDataFile string

	// Additional SSH public keys Name or ID to inject into the instance.
	SSHPublicKeys []string

	// Anti-affinity groups the instance will be a member of. The values can be IDs
	// or names of anti-affinity groups.
	AntiAffinityGroups []string

	// Additional disks to attach to the instance.
	AdditionalDisks []AdditionalDisk

	// Custom user agent string for API requests.
	UserAgent string

	// ID of the created instance. Used to retrieve instance state during
	// `GetState` and to delete the instance during `Remove`.
	InstanceID string

	// ID of the instance's boot disk. Used to delete the boot disk during
	// `Remove`.
	BootDiskID string

	// ID of the generated SSH public key that's injected into the instance.
	// Used to delete the SSH public key during `Remove`.
	SSHPublicKeyID string

	// IDs of the additional disks attached to the instance. Used to delete the
	// additional disks during `Remove`.
	AdditionalDiskIDs []string

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

// createOxideClient creates an Oxide client from the machine driver
// configuration.
func (d *Driver) createOxideClient() (*oxide.Client, error) {
	opts := []oxide.ClientOption{
		oxide.WithHost(d.Host),
		oxide.WithToken(d.Token),
	}
	if d.UserAgent != "" {
		opts = append(opts, oxide.WithUserAgent(d.UserAgent))
	}
	return oxide.NewClient(opts...)
}

// Create creates the instance and any necessary dependencies (e.g., SSH keys,
// disks) and updates the machine driver with state for use by other methods.
// Create must start the instance otherwise the machine driver will time out
// waiting for the instance to start.
func (d *Driver) Create() error {
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	pubKey, err := d.createSSHKeyPair()
	if err != nil {
		return err
	}

	d.SSHPublicKeyID = pubKey.Id

	sshPublicKeys := []oxide.NameOrId{
		oxide.NameOrId(d.SSHPublicKeyID),
	}
	for _, sshPubKey := range d.SSHPublicKeys {
		sshPublicKeys = append(sshPublicKeys, oxide.NameOrId(sshPubKey))
	}

	var userData []byte
	if d.UserDataFile != "" {
		b, err := os.ReadFile(d.UserDataFile)
		if err != nil {
			return err
		}
		userData = b
	}

	disks := make([]oxide.InstanceDiskAttachment, len(d.AdditionalDisks))
	for i, additionalDisk := range d.AdditionalDisks {
		disks[i] = oxide.InstanceDiskAttachment{
			Value: &oxide.InstanceDiskAttachmentCreate{
				Description: defaultDescription,
				DiskBackend: oxide.DiskBackend{
					Value: &oxide.DiskBackendDistributed{
						DiskSource: oxide.DiskSource{
							Value: &oxide.DiskSourceBlank{
								BlockSize: oxide.BlockSize(4096),
							},
						},
					},
				},
				Name: oxide.Name(additionalDisk.Name(d.MachineName, i)),
				Size: oxide.ByteCount(additionalDisk.Size),
			},
		}
	}

	antiAffinityGroups := make([]oxide.NameOrId, 0, len(d.AntiAffinityGroups))
	for _, antiAffinityGroup := range d.AntiAffinityGroups {
		antiAffinityGroups = append(antiAffinityGroups, oxide.NameOrId(antiAffinityGroup))
	}

	externalIPs := make([]oxide.ExternalIpCreate, 0, 1)
	if d.EphemeralIPAttach {
		var poolSelector oxide.PoolSelector
		if d.EphemeralIPPool != "" {
			poolSelector = oxide.PoolSelector{
				Value: &oxide.PoolSelectorExplicit{
					Pool: oxide.NameOrId(d.EphemeralIPPool),
				},
			}
		} else {
			poolSelector = oxide.PoolSelector{
				Value: &oxide.PoolSelectorAuto{
					IpVersion: oxide.IpVersionV4,
				},
			}
		}

		extIP := oxide.ExternalIpCreate{
			Value: &oxide.ExternalIpCreateEphemeral{
				PoolSelector: poolSelector,
			},
		}
		externalIPs = append(externalIPs, extIP)
	}

	icp := oxide.InstanceCreateParams{
		Project: oxide.NameOrId(d.Project),
		Body: &oxide.InstanceCreate{
			AntiAffinityGroups: antiAffinityGroups,
			BootDisk: oxide.InstanceDiskAttachment{
				Value: &oxide.InstanceDiskAttachmentCreate{
					Description: defaultDescription,
					DiskBackend: oxide.DiskBackend{
						Value: &oxide.DiskBackendDistributed{
							DiskSource: oxide.DiskSource{
								Value: &oxide.DiskSourceImage{
									ImageId: d.BootDiskImageID,
								},
							},
						},
					},
					Name: oxide.Name("disk-" + d.GetMachineName()),
					Size: oxide.ByteCount(d.BootDiskSize),
				},
			},
			Disks:       disks,
			Description: defaultDescription,
			ExternalIps: externalIPs,
			Hostname:    oxide.Hostname(d.GetMachineName()),
			Memory:      oxide.ByteCount(d.Memory),
			Name:        oxide.Name(d.GetMachineName()),
			Ncpus:       oxide.InstanceCpuCount(d.VCPUS),
			NetworkInterfaces: oxide.InstanceNetworkInterfaceAttachment{
				Value: &oxide.InstanceNetworkInterfaceAttachmentCreate{
					Params: []oxide.InstanceNetworkInterfaceCreate{
						{
							Description: defaultDescription,
							Name:        oxide.Name("nic-" + d.GetMachineName()),
							SubnetName:  oxide.Name(d.Subnet),
							VpcName:     oxide.Name(d.VPC),
							IpConfig: oxide.PrivateIpStackCreate{
								Value: &oxide.PrivateIpStackCreateV4{
									Value: oxide.PrivateIpv4StackCreate{
										Ip: oxide.Ipv4Assignment{
											Value: &oxide.Ipv4AssignmentAuto{},
										},
									},
								},
							},
						},
					},
				},
			},
			SshPublicKeys: sshPublicKeys,
			UserData:      base64.StdEncoding.EncodeToString(userData),
		},
	}
	instance, err := d.oxideClient.InstanceCreate(context.TODO(), icp)
	if err != nil {
		return err
	}

	d.InstanceID = instance.Id
	d.BootDiskID = instance.BootDiskId

	inilp := oxide.InstanceNetworkInterfaceListParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	networkInterfaces, err := d.oxideClient.InstanceNetworkInterfaceListAllPages(context.TODO(), inilp)
	if err != nil {
		return err
	}

	if len(networkInterfaces) == 0 {
		return errors.New("no valid network interfaces found")
	}

	nic := networkInterfaces[0]
	switch v := nic.IpStack.Value.(type) {
	case oxide.PrivateIpStackV4:
		d.IPAddress = v.Value.Ip
	case *oxide.PrivateIpStackV4:
		d.IPAddress = v.Value.Ip
	case oxide.PrivateIpStackDualStack:
		d.IPAddress = v.Value.V4.Ip
	case *oxide.PrivateIpStackDualStack:
		d.IPAddress = v.Value.V4.Ip
	default:
		return errors.New(
			"no IPv4 address found on network interface",
		)
	}

	additionalDisks, err := d.oxideClient.InstanceDiskListAllPages(context.TODO(), oxide.InstanceDiskListParams{
		Instance: oxide.NameOrId(d.InstanceID),
	})
	if err != nil {
		return fmt.Errorf("failed listing disks for instance: %w", err)
	}

	d.AdditionalDiskIDs = make([]string, 0, len(d.AdditionalDisks))
	for _, additionalDisk := range additionalDisks {
		// The boot disk ID state is managed irrespective of the additional disks.
		if additionalDisk.Id == instance.BootDiskId {
			continue
		}
		d.AdditionalDiskIDs = append(d.AdditionalDiskIDs, additionalDisk.Id)
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
		mcnflag.StringFlag{
			Name:   flagMemory,
			Usage:  "Amount of memory, in bytes, to give the instance. Supports a unit suffix (e.g., 4 GiB).",
			EnvVar: "OXIDE_MEMORY",
			Value:  defaultMemory,
		},

		// Boot disk.
		mcnflag.StringFlag{
			Name:   flagBootDiskSize,
			Usage:  "Size of the instance's boot disk, in bytes. Supports a unit suffix (e.g., 20 GiB).",
			EnvVar: "OXIDE_BOOT_DISK_SIZE",
			Value:  defaultBootDiskSize,
		},
		mcnflag.StringFlag{
			Name:   flagBootDiskImageID,
			Usage:  "Image ID to use for the instance's boot disk.",
			EnvVar: "OXIDE_BOOT_DISK_IMAGE_ID",
		},

		// Additional disks.
		mcnflag.StringSliceFlag{
			Name:  flagAdditionalDisk,
			Usage: "Additional disks to attach to the instance in the format `SIZE[,LABEL]` where `SIZE` is the disk size in bytes and `LABEL` is an arbitrary string used within the disk name for identification. `SIZE` supports a unit suffix (e.g., 20 GiB).",
		},

		// Networking.
		mcnflag.StringFlag{
			Name:   flagVPC,
			Usage:  "VPC name for the instance's network interface.",
			EnvVar: "OXIDE_VPC",
			Value:  "default",
		},
		mcnflag.StringFlag{
			Name:   flagSubnet,
			Usage:  "Subnet name for the instance's network interface.",
			EnvVar: "OXIDE_SUBNET",
			Value:  "default",
		},
		mcnflag.BoolFlag{
			Name:   flagEphemeralIPAttach,
			Usage:  "Should an ephemeral IP address be allocated for the instance.",
			EnvVar: "OXIDE_EPHEMERAL_IP_ATTACH",
		},
		mcnflag.StringFlag{
			Name:   flagEphemeralIPPool,
			Usage:  "ephemeral IP pool that should be used for the instance or empty string to use the default (only used if oxide-ephemeral-ip is true)",
			EnvVar: "OXIDE_EPHEMERAL_IP_POOL",
			Value:  "",
		},

		// User data.
		mcnflag.StringFlag{
			Name:   flagUserDataFile,
			Usage:  "Path to file containing user data for the instance.",
			EnvVar: "OXIDE_USER_DATA_FILE",
		},

		// SSH information.
		mcnflag.StringFlag{
			Name:   flagSSHUser,
			Usage:  "User to use when connecting to the instance via SSH.",
			EnvVar: "OXIDE_SSH_USER",
		},
		mcnflag.StringSliceFlag{
			Name:   flagSSHPublicKey,
			Usage:  "Additional SSH public keys IDs to inject into the instance.",
			EnvVar: "OXIDE_ADDITIONAL_SSH_PUBLIC_KEY_IDS",
		},

		// Anti-affinity groups.
		mcnflag.StringSliceFlag{
			Name:  flagAntiAffinityGroup,
			Usage: "Anti-affinity groups the instance will be a member of. The values can be IDs or names of anti-affinity groups.",
		},

		// User agent.
		mcnflag.StringFlag{
			Name:   flagUserAgent,
			Usage:  "Custom user agent string for API requests.",
			EnvVar: "OXIDE_USER_AGENT",
			Value:  "Oxide Rancher Machine Driver",
		},
	}
}

// GetSSHHostname returns the IP address or DNS name of the instance.
// This IP address or DNS name must be accessible from Rancher.
func (d *Driver) GetSSHHostname() (string, error) {
	// Use the embedded BaseDriver's logic.
	return d.GetIP()
}

// GetState fetches the current state of the instance and returns it as
// a standardized state representation that Rancher can understand.
func (d *Driver) GetState() (state.State, error) {
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return state.None, err
		}
		d.oxideClient = client
	}

	instance, err := d.oxideClient.InstanceView(context.TODO(), oxide.InstanceViewParams{
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
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
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

	if err := d.oxideClient.CurrentUserSshKeyDelete(context.TODO(), oxide.CurrentUserSshKeyDeleteParams{
		SshKey: oxide.NameOrId(d.SSHPublicKeyID),
	}); err != nil {
		return err
	}

	if err := d.oxideClient.InstanceDelete(context.TODO(), oxide.InstanceDeleteParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}); err != nil {
		return err
	}

	if err := d.oxideClient.DiskDelete(context.TODO(), oxide.DiskDeleteParams{
		Disk: oxide.NameOrId(d.BootDiskID),
	}); err != nil {
		return err
	}

	for _, additionalDiskID := range d.AdditionalDiskIDs {
		if err := d.oxideClient.DiskDelete(context.TODO(), oxide.DiskDeleteParams{
			Disk: oxide.NameOrId(additionalDiskID),
		}); err != nil {
			return err
		}
	}

	return nil
}

// Restart restarts the instance without changing its configuration.
func (d *Driver) Restart() error {
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	irp := oxide.InstanceRebootParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceReboot(context.TODO(), irp); err != nil {
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
	d.BootDiskImageID = opts.String(flagBootDiskImageID)
	d.VPC = opts.String(flagVPC)
	d.Subnet = opts.String(flagSubnet)
	d.UserDataFile = opts.String(flagUserDataFile)
	d.SSHUser = opts.String(flagSSHUser)
	d.SSHPublicKeys = opts.StringSlice(flagSSHPublicKey)
	d.AntiAffinityGroups = opts.StringSlice(flagAntiAffinityGroup)
	d.SSHPort = defaultSSHPort
	d.EphemeralIPAttach = opts.Bool(flagEphemeralIPAttach)
	d.EphemeralIPPool = opts.String(flagEphemeralIPPool)
	d.UserAgent = opts.String(flagUserAgent)

	// Required flags.
	{
		var joinedRequiredFlagError error

		if d.Host == "" {
			joinedRequiredFlagError = errors.Join(joinedRequiredFlagError, NewRequiredFlagError(flagHost))
		}

		if d.Token == "" {
			joinedRequiredFlagError = errors.Join(joinedRequiredFlagError, NewRequiredFlagError(flagToken))
		}

		if d.Project == "" {
			joinedRequiredFlagError = errors.Join(joinedRequiredFlagError, NewRequiredFlagError(flagProject))
		}

		if d.BootDiskImageID == "" {
			joinedRequiredFlagError = errors.Join(joinedRequiredFlagError, NewRequiredFlagError(flagBootDiskImageID))
		}

		if joinedRequiredFlagError != nil {
			return joinedRequiredFlagError
		}
	}

	// Optional flags that need to be parsed. The logic is a bit verbose due to the
	// type conversion between the flag and the field in the Driver struct.
	{
		var joinedParseErr error

		memoryStr := opts.String(flagMemory)
		if memoryStr == "" {
			memoryStr = defaultMemory
		}
		memory, err := humanize.ParseBytes(memoryStr)
		if err != nil {
			joinedParseErr = errors.Join(joinedParseErr, NewFlagParseError(flagMemory, err))
		}
		d.Memory = memory

		bootDiskSizeStr := opts.String(flagBootDiskSize)
		if bootDiskSizeStr == "" {
			bootDiskSizeStr = defaultBootDiskSize
		}
		bootDiskSize, err := humanize.ParseBytes(bootDiskSizeStr)
		if err != nil {
			joinedParseErr = errors.Join(joinedParseErr, NewFlagParseError(flagBootDiskSize, err))
		}
		d.BootDiskSize = bootDiskSize

		d.AdditionalDisks = make([]AdditionalDisk, 0)
		for _, diskInfo := range opts.StringSlice(flagAdditionalDisk) {
			additionalDisk, err := ParseAdditionalDisk(diskInfo)
			if err != nil {
				joinedParseErr = errors.Join(joinedParseErr, NewFlagParseError(flagAdditionalDisk, err))
			}
			d.AdditionalDisks = append(d.AdditionalDisks, additionalDisk)
		}

		if joinedParseErr != nil {
			return joinedParseErr
		}
	}

	return nil
}

// Start starts the instance.
func (d *Driver) Start() error {
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	isp := oxide.InstanceStartParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceStart(context.TODO(), isp); err != nil {
		return err
	}

	return nil
}

// Stop stops the instance.
func (d *Driver) Stop() error {
	if d.oxideClient == nil {
		client, err := d.createOxideClient()
		if err != nil {
			return err
		}
		d.oxideClient = client
	}

	isp := oxide.InstanceStopParams{
		Instance: oxide.NameOrId(d.InstanceID),
	}
	if _, err := d.oxideClient.InstanceStop(context.TODO(), isp); err != nil {
		return err
	}

	return nil
}

// createSSHKeyPair creates a new SSH key pair, saves both the private and
// public key to the store path for the machine driver to use, and uploads the
// public key to Oxide to be injected into the instance.
func (d *Driver) createSSHKeyPair() (*oxide.SshKey, error) {
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
	return d.oxideClient.CurrentUserSshKeyCreate(context.TODO(), cuscp)
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

// AdditionalDisk represents a disk attached to an instance.
type AdditionalDisk struct {
	// Required. The size of the disk in bytes.
	Size uint64

	// An optional label to use in the disk name for ease of identification.
	Label string
}

// ParseAdditionalDisk parses an `AdditionalDisk` from a string in the format
// `SIZE[,LABEL]` where `SIZE` is the disk size in bytes and `LABEL` is an
// arbitrary string used within the disk name for identification. `SIZE`
// supports a unit suffix (e.g., 20 GiB).
func ParseAdditionalDisk(s string) (AdditionalDisk, error) {
	var sizeStr string
	label := "additional"

	fields := strings.Split(s, ",")
	switch len(fields) {
	case 2:
		sizeStr = fields[0]
		if fields[1] != "" {
			label = fields[1]
		}
	case 1:
		sizeStr = fields[0]
	default:
		return AdditionalDisk{}, fmt.Errorf("invalid format %q, expected size[,label]", s)
	}

	size, err := humanize.ParseBytes(sizeStr)
	if err != nil {
		return AdditionalDisk{}, fmt.Errorf("failed parsing size %q %w", sizeStr, err)
	}

	a := AdditionalDisk{
		Size:  size,
		Label: label,
	}

	return a, nil
}

// Name returns a string representing the disk name.
func (a AdditionalDisk) Name(machineName string, diskNumber int) string {
	return fmt.Sprintf("disk-%02d-%s-%s", diskNumber, a.Label, machineName)
}
