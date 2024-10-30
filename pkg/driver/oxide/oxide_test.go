package oxide

import (
	"testing"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/go-test/deep"
	"gotest.tools/v3/assert"
)

func TestNewDriver_Instance(t *testing.T) {
	d := NewDriver("bob", "store")
	if d == nil {
		t.Error("Expected driver to be created, got nil")
	}

	if diff := deep.Equal(d, &Driver{
		BaseDriver: &drivers.BaseDriver{
			SSHUser:     "debian",
			MachineName: "bob",
			StorePath:   "store",
		},
	}); diff != nil {
		t.Error(diff)
	}
}

func TestNewDriver_SimpleValidation(t *testing.T) {
	d := NewDriver("bob", "da-path")
	if d == nil {
		t.Error("Expected driver to be created, got nil")
	}

	assert.Equal(t, d.DriverName(), "oxide")
	assert.Equal(t, d.GetSSHUsername(), "debian")
	assert.Equal(t, d.GetSSHKeyPath(), "da-path/machines/bob/id_rsa")

	assert.Equal(t, len(d.GetCreateFlags()), 13)
}

func TestDriver_PreCreateCheck_Pass(t *testing.T) {
	d := NewDriver("bob", "da-path")
	err := d.PreCreateCheck()
	assert.NilError(t, err)
}

func TestDriver_PreCreateCheck_UserDataAlmostTooLarge(t *testing.T) {
	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)
	d.UserData = GetTestFile(t, "./userData/almostTooLarge.yaml")
	d.ExtraUserData = ""

	err := d.PreCreateCheck()
	assert.NilError(t, err)
}

func TestDriver_PreCreateCheck_UserDataTooLarge(t *testing.T) {
	d := NewDriver("bob", GetWorkingDir(t)).(*Driver)
	d.UserData = GetTestFile(t, "./userData/almostTooLarge.yaml")
	d.ExtraUserData = "#cloud-config"

	err := d.PreCreateCheck()
	if err == nil {
		t.Error("Expected error, got nil")
	}
	assert.Equal(t, err.Error(), "user data is too large")
}

func TestDriver_SetConfigFromFlags_Missing(t *testing.T) {
	options := NewMockDriverOptions(map[string]interface{}{})
	options.DefaultString = ""

	d := NewDriver("bob", "store")
	err := d.SetConfigFromFlags(options)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	e := err.(requiredOptionError)

	assert.Equal(t, e.Error(), "Oxide driver requires the \"oxide-host\" option.")
}

func TestDriver_SetConfigFromFlags(t *testing.T) {
	options := NewMockDriverOptions(map[string]interface{}{
		flagHost:          "example.com",
		flagToken:         "token123",
		flagProject:       "project1",
		flagUserData:      "userData",
		flagExtraUserData: "extraUserData",
		flagDiskSize:      100,
		flagDiskImageId:   "image123",
		flagMemory:        2048,
		flagNCpus:         2,
		flagVpc:           "vpc1",
		flagSubnet:        "subnet1",
		flagSSHUser:       "user",
		flagSSHPubKeys:    []string{"key1", "key2"},
		"swarm-master":    false,
		"swarm-host":      "swarm-host",
		"swarm-discovery": "swarm-discovery",
	})

	expected := &Driver{
		Host:          "example.com",
		Token:         "token123",
		Project:       "project1",
		UserData:      "userData",
		ExtraUserData: "extraUserData",
		DiskSize:      100,
		DiskImageId:   "image123",
		Memory:        2048,
		Ncpus:         2,
		VpcName:       "vpc1",
		SubnetName:    "subnet1",
		SSHKeys:       []string{"key1", "key2"},
		BaseDriver: &drivers.BaseDriver{
			MachineName:    "bob",
			StorePath:      "store",
			SSHUser:        "user",
			SSHPort:        22,
			SwarmDiscovery: "swarm-discovery",
			SwarmHost:      "swarm-host",
			SwarmMaster:    false,
		},
	}

	d := NewDriver("bob", "store")
	err := d.SetConfigFromFlags(options)
	assert.NilError(t, err)
	assert.DeepEqual(t, d, expected)
}
