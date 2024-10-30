package oxide

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/state"
	"github.com/oxidecomputer/oxide.go/oxide"
)

const (
	defaultSSHUser = "debian"
)

const (
	flagHost  = "oxide-host"
	flagToken = "oxide-token"

	flagProject       = "oxide-project"
	flagUserData      = "oxide-user-data"
	flagExtraUserData = "oxide-extra-user-data"

	flagDiskSize    = "oxide-disk-size"
	flagDiskImageId = "oxide-disk-image-id"
	flagMemory      = "oxide-memory"
	flagNCpus       = "oxide-number-of-cpus"

	flagSSHUser    = "oxide-ssh-user"
	flagSSHPubKeys = "oxide-ssh-pub-keys"

	flagVpc    = "oxide-vpc"
	flagSubnet = "oxide-subnet"
)

type Driver struct {
	*drivers.BaseDriver

	Host  string
	Token string

	Project  string
	UserData string

	ExtraUserData string

	DiskSize    int
	DiskImageId string
	Memory      int
	Ncpus       int

	VpcName    string
	SubnetName string

	SSHKeys []string

	Client      *oxide.Client
	OxideSSHKey *oxide.SshKey
}

// NewDriver returns a new driver instance.
func NewDriver(hostName, storePath string) drivers.Driver {
	d := Driver{
		BaseDriver: &drivers.BaseDriver{
			SSHUser:     defaultSSHUser,
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
	return &d
}

func (d *Driver) DriverName() string {
	return "oxide"
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			Name:   flagHost,
			Usage:  "Oxide host",
			EnvVar: "OXIDE_HOST",
		},
		mcnflag.StringFlag{
			Name:   flagToken,
			Usage:  "Oxide token",
			EnvVar: "OXIDE_TOKEN",
		},

		mcnflag.StringFlag{
			Name:   flagProject,
			Usage:  "Oxide project",
			EnvVar: "OXIDE_PROJECT",
		},
		mcnflag.StringFlag{
			Name:   flagUserData,
			Usage:  "Oxide user data (cloud init)",
			EnvVar: "OXIDE_USER_DATA",
		},
		mcnflag.StringFlag{
			Name:   flagExtraUserData,
			Usage:  "Oxide extra user data (cloud init)",
			EnvVar: "OXIDE_EXTRA_USER_DATA",
		},

		mcnflag.IntFlag{
			Name:   flagDiskSize,
			Usage:  "Oxide disk size",
			EnvVar: "OXIDE_DISK_SIZE",
			Value:  34359738368,
		},
		mcnflag.StringFlag{
			Name:   flagDiskImageId,
			Usage:  "Oxide disk image id",
			EnvVar: "OXIDE_DISK_IMAGE_ID",
			Value:  "c1bc5374-614b-479e-8352-ac4526c29f97",
		},
		mcnflag.IntFlag{
			Name:   flagMemory,
			Usage:  "Oxide memory",
			EnvVar: "OXIDE_MEMORY",
			Value:  4294967296,
		},
		mcnflag.IntFlag{
			Name:   flagNCpus,
			Usage:  "Oxide Number of CPUs",
			EnvVar: "OXIDE_NUMBER_OF_CPUS",
			Value:  2,
		},

		mcnflag.StringFlag{
			Name:   flagSSHUser,
			Usage:  "Oxide user",
			EnvVar: "OXIDE_SSH_USER",
			Value:  defaultSSHUser,
		},
		mcnflag.StringSliceFlag{
			Name:   flagSSHPubKeys,
			Usage:  "Oxide ssh public key IDs",
			EnvVar: "OXIDE_SSH_PUB_KEYS",
			Value:  []string{"34f34227-cf76-4b1e-85ae-17ecfc744f00"},
		},

		mcnflag.StringFlag{
			Name:   flagVpc,
			Usage:  "Oxide vpc",
			EnvVar: "OXIDE_VPC",
			Value:  "default",
		},
		mcnflag.StringFlag{
			Name:   flagSubnet,
			Usage:  "Oxide subnet",
			EnvVar: "OXIDE_SUBNET",
			Value:  "default",
		},
	}
}

func (d *Driver) SetConfigFromFlags(opts drivers.DriverOptions) (err error) {
	flags := []struct {
		target *string
		flag   string
	}{
		{&d.Host, flagHost},
		{&d.Token, flagToken},
		{&d.Project, flagProject},
	}

	for _, f := range flags {
		*f.target = opts.String(f.flag)
		if *f.target == "" {
			return requiredOptionError(f.flag)
		}
	}

	d.Host = opts.String(flagHost)
	d.Token = opts.String(flagToken)
	d.Project = opts.String(flagProject)
	d.UserData = opts.String(flagUserData)
	d.ExtraUserData = opts.String(flagExtraUserData)

	d.DiskSize = opts.Int(flagDiskSize)
	d.DiskImageId = opts.String(flagDiskImageId)
	d.Memory = opts.Int(flagMemory)
	d.Ncpus = opts.Int(flagNCpus)

	d.VpcName = opts.String(flagVpc)
	d.SubnetName = opts.String(flagSubnet)

	d.BaseDriver.SSHUser = opts.String(flagSSHUser)
	d.SSHKeys = opts.StringSlice(flagSSHPubKeys)

	// base driver flags
	d.BaseDriver.SSHPort = 22
	d.SetSwarmConfigFromFlags(opts)

	return
}

// PreCreateCheck is called to enforce pre-creation steps
func (d *Driver) PreCreateCheck() error {
	if len(d.userDataBase64())*3/4 > 32767 {
		return fmt.Errorf("user data is too large")
	}
	return nil
}

func (d *Driver) Create() (err error) {
	var ctx = context.Background()
	var networkInterfaces []oxide.InstanceNetworkInterface
	var sshKey *oxide.SshKey

	if err = d.SetupClient(); err != nil {
		return
	}

	if sshKey, err = d.SshSetup(); err != nil {
		return
	}

	sshKeys := []oxide.NameOrId{
		oxide.NameOrId(sshKey.Id),
	}

	for _, key := range d.SSHKeys {
		sshKeys = append(sshKeys, oxide.NameOrId(key))
	}

	if _, err = d.Client.InstanceCreate(ctx,
		oxide.InstanceCreateParams{
			Project: oxide.NameOrId(d.Project),
			Body: &oxide.InstanceCreate{
				Description: "built by rancher-machine driver",
				BootDisk: &oxide.InstanceDiskAttachment{
					Description: "docker-machine disk",
					DiskSource: oxide.DiskSource{
						ImageId: d.DiskImageId,
						Type:    oxide.DiskSourceTypeImage,
					},
					Name: oxide.Name(d.diskName()),
					Size: oxide.ByteCount(d.DiskSize),
					Type: oxide.InstanceDiskAttachmentTypeCreate,
				},
				Disks:    []oxide.InstanceDiskAttachment{},
				Hostname: oxide.Hostname(d.MachineName),
				Memory:   oxide.ByteCount(d.Memory),
				Name:     oxide.Name(d.MachineName),
				Ncpus:    oxide.InstanceCpuCount(d.Ncpus),
				NetworkInterfaces: oxide.InstanceNetworkInterfaceAttachment{
					Type: oxide.InstanceNetworkInterfaceAttachmentTypeCreate,
					Params: []oxide.InstanceNetworkInterfaceCreate{
						{
							Description: "created by foo in rancher-machine driver",
							Name:        "huh",
							SubnetName:  oxide.Name(d.SubnetName),
							VpcName:     oxide.Name(d.VpcName),
						},
					},
				},
				UserData:      d.userDataBase64(),
				SshPublicKeys: sshKeys,
			},
		},
	); err != nil {
		return
	}

	if networkInterfaces, err = d.Client.InstanceNetworkInterfaceListAllPages(ctx,
		oxide.InstanceNetworkInterfaceListParams{
			Project:  oxide.NameOrId(d.Project),
			Instance: oxide.NameOrId(d.MachineName),
		},
	); err != nil {
		return
	}

	for _, nic := range networkInterfaces {
		// TODO: not a foreach
		d.IPAddress = nic.Ip
		log.Print("d.IPAddress", d.IPAddress)
	}

	return
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

// GetSSHUsername returns username for use with ssh
func (d *Driver) GetSSHUsername() string {
	if d.BaseDriver.SSHUser == "" {
		d.BaseDriver.SSHUser = defaultSSHUser
	}
	return d.BaseDriver.SSHUser
}

func (d *Driver) GetURL() (rv string, err error) {
	var ip string

	if err = drivers.MustBeRunning(d); err != nil {
		return
	}

	if ip, err = d.GetIP(); err != nil {
		return
	}

	return fmt.Sprintf("tcp://%s", net.JoinHostPort(ip, "2376")), nil
}

func (d *Driver) GetState() (machineState state.State, err error) {
	var ctx = context.Background()
	var instance *oxide.Instance

	err = d.SetupClient()

	if err != nil {
		return
	}

	instance, err = d.Client.InstanceView(ctx, oxide.InstanceViewParams{
		Project:  oxide.NameOrId(d.Project),
		Instance: oxide.NameOrId(d.MachineName),
	})

	if err != nil {
		return
	}

	machineState = OxideToDockerMachineState(instance.RunState)

	return
}

func (d *Driver) Kill() (err error) {
	return d.Stop()
}

func (d *Driver) Remove() (err error) {
	var ctx = context.Background()
	var sshKey *oxide.SshKey

	if err = d.SetupClient(); err != nil {
		return
	}

	if err = d.Stop(); err != nil {
		log.Print(err)
	}

	if err = d.Client.InstanceDelete(ctx, oxide.InstanceDeleteParams{
		Project:  oxide.NameOrId(d.Project),
		Instance: oxide.NameOrId(d.MachineName),
	}); err != nil {
		return
	}

	if err = d.Client.DiskDelete(ctx, oxide.DiskDeleteParams{
		Project: oxide.NameOrId(d.Project),
		Disk:    oxide.NameOrId(d.diskName()),
	}); err != nil {
		return
	}

	if sshKey, err = d.SshSetup(); err != nil {
		return
	}

	err = d.Client.CurrentUserSshKeyDelete(ctx, oxide.CurrentUserSshKeyDeleteParams{
		SshKey: oxide.NameOrId(sshKey.Id),
	})

	return
}

func (d *Driver) Restart() (err error) {
	var ctx = context.Background()

	if err = d.SetupClient(); err != nil {
		return
	}

	_, err = d.Client.InstanceReboot(ctx, oxide.InstanceRebootParams{
		Project:  oxide.NameOrId(d.Project),
		Instance: oxide.NameOrId(d.MachineName),
	})

	return
}

func (d *Driver) Start() (err error) {
	var ctx = context.Background()

	if err = d.SetupClient(); err != nil {
		return
	}

	_, err = d.Client.InstanceStart(ctx, oxide.InstanceStartParams{
		Project:  oxide.NameOrId(d.Project),
		Instance: oxide.NameOrId(d.MachineName),
	})

	return
}

func (d *Driver) Stop() (err error) {
	var ctx = context.Background()

	if err = d.SetupClient(); err != nil {
		return
	}

	_, err = d.Client.InstanceStop(ctx, oxide.InstanceStopParams{
		Project:  oxide.NameOrId(d.Project),
		Instance: oxide.NameOrId(d.MachineName),
	})

	return
}
