package oxide

import (
	"encoding/base64"
	"github.com/oxidecomputer/oxide.go/oxide"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
	"gotest.tools/v3/assert"
)

func TestDriver_SetupClient_simplistic(t *testing.T) {
	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)
	d.Host = "where"
	d.Token = "quarter"
	expected := &oxide.SshKey{
		Id: "expected",
	}

	d.OxideSSHKey = expected

	err := d.SetupClient()
	assert.NilError(t, err)

	if d.Client == nil {
		t.Error("Expected client to be created, got nil")
	}
}

func TestDriver_SshSetup_alreadyHasKey(t *testing.T) {
	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)
	d.Host = "where"
	d.Token = "quarter"
	expected := &oxide.SshKey{
		Id: "expected",
	}

	d.OxideSSHKey = expected

	actual, err := d.SshSetup()
	assert.NilError(t, err)

	assert.Equal(t, actual, expected)
}

func TestDriver_SshSetup_createNewKey(t *testing.T) {
	t.Skip("need to mock Client")
	var err error
	err = os.MkdirAll(filepath.Join(GetWorkingDir(t), "/machines/bob"), 0755)
	assert.NilError(t, err)

	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)

	d.Host = "where"
	d.Token = "quarter"

	_, err = d.SshSetup()
	assert.NilError(t, err)
}

func TestDriver_diskName(t *testing.T) {
	d := NewDriver("bob", "store").(*Driver)

	assert.Equal(t, d.diskName(), "bob-disk")
}

func TestDriver_privateSSHKeyPath(t *testing.T) {
	d := NewDriver("bob", "store").(*Driver)

	assert.Equal(t, d.privateSSHKeyPath(), "store/machines/bob/id_rsa")
}

func TestDriver_publicSSHKeyPath(t *testing.T) {
	d := NewDriver("bob", "store").(*Driver)

	assert.Equal(t, d.publicSSHKeyPath(), "store/machines/bob/id_rsa.pub")
}

func TestDriver_userDataBase64_empty(t *testing.T) {
	d := NewDriver("bob", "da-path").(*Driver)

	actual, err := base64.StdEncoding.DecodeString(d.userDataBase64())
	assert.NilError(t, err)

	assert.Equal(t, string(actual), "\n")
}

func TestDriver_userDataBase64_simple(t *testing.T) {
	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)
	d.UserData = GetTestFile(t, "./userData/basic.yaml")

	d.ExtraUserData = `#cloud-config
packages:
  - please
  - denied
`

	actual, err := base64.StdEncoding.DecodeString(d.userDataBase64())
	assert.NilError(t, err)

	if diff := deep.Equal(string(actual), `#cloud-config
package_update: true
package_upgrade: true

#cloud-config
packages:
  - please
  - denied
`); diff != nil {
		t.Error(diff)
	}
}
