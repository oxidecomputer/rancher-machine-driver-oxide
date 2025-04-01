package main

import (
	"errors"

	"github.com/rancher/machine/libmachine/drivers"
	"github.com/rancher/machine/libmachine/mcnflag"
)

const (
	flagHost                      = "oxide-host"
	flagToken                     = "oxide-token"
	flagProject                   = "oxide-project"
	flagVCPUs                     = "oxide-vcpus"
	flagMemory                    = "oxide-memory"
	flagBootDiskSize              = "oxide-boot-disk-size"
	flagBootDiskImageID           = "oxide-boot-disk-image-id"
	flagAdditionalDisksSizes      = "oxide-additional-disks-sizes"
	flagVPC                       = "oxide-vpc"
	flagSubnet                    = "oxide-subnet"
	flagUserDataFile              = "oxide-user-data-file"
	flagSSHUser                   = "oxide-ssh-user"
	flagAdditionalSSHPublicKeyIDs = "oxide-additional-ssh-public-key-ids"
)

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
			Usage:  "Amount of memory, in bytes, to give the instance.",
			EnvVar: "OXIDE_MEMORY",
			Value:  "4 GiB",
		},

		// Boot disk.
		mcnflag.StringFlag{
			Name:   flagBootDiskSize,
			Usage:  "Size of the instance's boot disk, in bytes.",
			EnvVar: "OXIDE_BOOT_DISK_SIZE",
			Value:  "20 GiB",
		},
		mcnflag.StringFlag{
			Name:   flagBootDiskImageID,
			Usage:  "Image ID to use for the instance's boot disk.",
			EnvVar: "OXIDE_BOOT_DISK_IMAGE_ID",
		},

		// Additional Disk
		mcnflag.StringSliceFlag{
			Name:   flagAdditionalDisksSizes,
			Usage:  "Size of hte additional disk, in bytes.",
			EnvVar: "OXIDE_ADDITIONAL_DISK_SIZE",
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
			Usage:  "Path to file containing user data.",
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

// SetConfigFromFlags reads the CLI flags and sets necessary state on the
// driver for use by other methods.
func (d *Driver) SetConfigFromFlags(opts drivers.DriverOptions) error {
	d.Host = opts.String(flagHost)
	d.Token = opts.String(flagToken)
	d.Project = opts.String(flagProject)
	d.VCPUS = opts.Int(flagVCPUs)
	d.BootDiskImageID = opts.String(flagBootDiskImageID)
	d.AdditionalDiskSizesStrings = opts.StringSlice(flagAdditionalDisksSizes)
	d.VPC = opts.String(flagVPC)
	d.Subnet = opts.String(flagSubnet)
	d.UserDataFile = opts.String(flagUserDataFile)
	d.SSHUser = opts.String(flagSSHUser)
	d.AdditionalSSHPublicKeyIDs = opts.StringSlice(flagAdditionalSSHPublicKeyIDs)
	d.SSHPort = defaultSSHPort

	var err, fmtErr error

	d.Memory, fmtErr = HumanizeSize(opts.String(flagMemory))
	err = errors.Join(err, fmtErr)

	d.BootDiskSize, fmtErr = HumanizeSize(opts.String(flagBootDiskSize))
	err = errors.Join(err, fmtErr)

	d.AdditionalDisks = make([]AdditionalDisk, len(d.AdditionalDiskSizesStrings))
	for i, additionalDiskSizeString := range d.AdditionalDiskSizesStrings {
		d.AdditionalDisks[i], fmtErr = NewAdditionalDiskFromFlag(additionalDiskSizeString)
		err = errors.Join(err, fmtErr)
	}

	if d.Host == "" {
		err = errors.Join(err, errors.New(errRequiredOptionNotSet+flagHost))
	}

	if d.Token == "" {
		err = errors.Join(err, errors.New(errRequiredOptionNotSet+flagToken))
	}

	if d.Project == "" {
		err = errors.Join(err, errors.New(errRequiredOptionNotSet+flagProject))
	}

	if d.BootDiskImageID == "" {
		err = errors.Join(err, errors.New(errRequiredOptionNotSet+flagBootDiskImageID))
	}

	if err != nil {
		return err
	}

	return nil
}
