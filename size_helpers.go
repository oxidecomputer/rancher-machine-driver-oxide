package main

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/oxidecomputer/oxide.go/oxide"
)

const defaultAdditionalDiskLabel = "additional"

type AdditionalDisk struct {
	Label string
	Size  int
}

func (d AdditionalDisk) DiskName(machineName string, i int) oxide.Name {
	return oxide.Name(fmt.Sprintf(
		"disk-%d-%s-%s",
		i,
		oxide.Name(d.Label),
		machineName,
	))
}

func NewAdditionalDiskFromFlag(flag string) (disk AdditionalDisk, err error) {
	var size, label string
	sizeAndLabel := strings.Split(flag, ",")
	if len(sizeAndLabel) == 2 {
		size, label = sizeAndLabel[0], sizeAndLabel[1]
	} else if len(sizeAndLabel) == 1 {
		size, label = sizeAndLabel[0], defaultAdditionalDiskLabel
	} else {
		err = fmt.Errorf("invalid additional disk definition: %s", flag)
		return
	}

	disk.Label = label
	disk.Size, err = HumanizeSize(size)

	return
}

func HumanizeSize(sizeStr string) (size int, err error) {
	var sizeUInt64 uint64
	if sizeUInt64, err = humanize.ParseBytes(sizeStr); err != nil {
		return
	}

	// TODO: verify arbitrary size restrictions... doesn't make sense to me for values in an oxide rack :)
	if int(sizeUInt64) > humanize.PiByte {
		err = fmt.Errorf("invalid size definition, size too large")
	}

	size = int(sizeUInt64)

	return
}
