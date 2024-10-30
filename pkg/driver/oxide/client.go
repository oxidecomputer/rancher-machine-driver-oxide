package oxide

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/docker/machine/libmachine/ssh"
	"github.com/oxidecomputer/oxide.go/oxide"
)

func (d *Driver) SetupClient() (err error) {
	if d.Client != nil {
		return
	}

	var cl *oxide.Client
	var cfg = oxide.Config{
		Host:  d.Host,
		Token: d.Token,
	}

	if cl, err = oxide.NewClient(&cfg); err != nil {
		return
	}

	d.Client = cl

	_, err = d.SshSetup()

	return
}

func (d *Driver) SshSetup() (sshKey *oxide.SshKey, err error) {
	var ctx = context.Background()
	var keys []oxide.SshKey
	var thisKey string
	var byteKey []byte

	if d.OxideSSHKey != nil {
		sshKey = d.OxideSSHKey
		return
	}

	if _, err = os.Stat(d.privateSSHKeyPath()); os.IsNotExist(err) {
		if err = ssh.GenerateSSHKey(d.privateSSHKeyPath()); err != nil {
			return
		}
	}

	if byteKey, err = os.ReadFile(d.publicSSHKeyPath()); err != nil {
		return
	}

	tmp := strings.Split(string(byteKey), " ")
	if len(tmp) < 2 {
		return nil, errors.New("invalid pubkey")
	}
	thisKey = tmp[0] + " " + tmp[1]

	if keys, err = d.Client.CurrentUserSshKeyListAllPages(ctx, oxide.CurrentUserSshKeyListParams{}); err != nil {
		return
	}

	found := false

	for _, key := range keys {
		if key.PublicKey == thisKey {
			found = true
			sshKey = &key
		}
	}

	if !found {
		if sshKey, err = d.Client.CurrentUserSshKeyCreate(ctx, oxide.CurrentUserSshKeyCreateParams{
			Body: &oxide.SshKeyCreate{
				Description: "built by foo in rancher-machine driver",
				Name:        oxide.Name(d.MachineName),
				PublicKey:   thisKey,
			},
		}); err != nil {
			return
		}
	}

	d.OxideSSHKey = sshKey

	return
}

func (d *Driver) diskName() string {
	return fmt.Sprintf("%s-disk", d.MachineName)
}

func (d *Driver) privateSSHKeyPath() string {
	return d.GetSSHKeyPath()
}

func (d *Driver) publicSSHKeyPath() string {
	return d.GetSSHKeyPath() + ".pub"
}

func (d *Driver) userDataBase64() string {
	dater, _ := os.ReadFile(d.UserData)
	appendDater := d.ExtraUserData
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s\n%s",
		dater,
		appendDater,
	)))
}
