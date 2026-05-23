package main

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/oxidecomputer/oxide.go/oxide"
)

type additionalDiskPart int

const (
	additionalDiskPartSizeName        = "size"
	additionalDiskPartLabelName       = "label"
	additionalDiskPartBackendTypeName = "type"
)

const (
	additionalDiskPartSize additionalDiskPart = iota
	additionalDiskPartLabel
	additionalDiskPartBackendType
)

var additionDiskPartMap = map[string]additionalDiskPart{
	additionalDiskPartSizeName:        additionalDiskPartSize,
	additionalDiskPartLabelName:       additionalDiskPartLabel,
	additionalDiskPartBackendTypeName: additionalDiskPartBackendType,
}

// additionalDisk represents a disk attached to an instance.
type additionalDisk struct {
	// Required. The size of the disk in bytes.
	size uint64

	// An optional label to use in the disk name for ease of identification.
	label string

	// The type of backend to use: local or distributed
	backend oxide.DiskBackendType
}

// parseAdditionalDisk parses an `AdditionalDisk` from a string in the format
// size=SIZE[,label=LABEL][,type=TYPE] or
// `SIZE[[,LABEL],TYPE]` where `SIZE` is the disk size in bytes, `LABEL` is an
// arbitrary string used within the disk name for identification, and `TYPE` is the
// Oxide disk_backend: `local` or `distributed`. `SIZE` supports a unit suffix (e.g., 20 GiB).
func parseAdditionalDisk(s string) (additionalDisk, error) {
	var sizeStr string
	label := "additional"
	backend := oxide.DiskBackendTypeDistributed

	if s == "" {
		return additionalDisk{}, fmt.Errorf("invalid format empty string given, expected size[[,label],backend]")
	}

	fields, err := parseAdditionalDiskParts(s)
	if err != nil {
		return additionalDisk{}, err
	}
	switch len(fields) {
	case 3:
		if fields[2] != "" {
			backend = oxide.DiskBackendType(fields[2])
		}
		fallthrough
	case 2:
		if fields[1] != "" {
			label = fields[1]
		}
		fallthrough
	case 1:
		sizeStr = fields[0]
	default:
		return additionalDisk{}, fmt.Errorf("invalid format %q, expected size[[,label],backend]", s)
	}

	if backend != oxide.DiskBackendTypeDistributed && backend != oxide.DiskBackendTypeLocal {
		return additionalDisk{}, fmt.Errorf("invalid backend %q, expected %q or %q", backend, oxide.DiskBackendTypeDistributed, oxide.DiskBackendTypeLocal)
	}

	size, err := humanize.ParseBytes(sizeStr)
	if err != nil {
		return additionalDisk{}, fmt.Errorf("failed parsing size %q %w", sizeStr, err)
	}

	a := additionalDisk{
		size:    size,
		label:   label,
		backend: backend,
	}

	return a, nil
}

// return {size, label, backend}
func parseAdditionalDiskParts(s string) (parts []string, err error) {
	// old way
	if !strings.Contains(s, "=") {
		for _, part := range strings.Split(s, ",") {
			parts = append(parts, strings.TrimSpace(part))
		}
		return
	}

	// new way
	parts = make([]string, 3)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		part := strings.Split(p, "=")
		if len(part) != 2 {
			err = fmt.Errorf("invalid format %q, expected \"name=value\"", p)
			return
		}
		name := strings.TrimSpace(part[0])
		value := strings.TrimSpace(part[1])
		index, ok := additionDiskPartMap[name]
		if !ok {
			err = fmt.Errorf("invalid additional disk part: %q, expected %q, %q, or %q", name,
				additionalDiskPartSizeName,
				additionalDiskPartLabelName,
				additionalDiskPartBackendTypeName,
			)
		}
		parts[index] = value
	}

	return
}

// name returns a string representing the disk name.
func (a *additionalDisk) name(machineName string, diskNumber int) string {
	name := a.innerName(machineName, diskNumber)

	if len(name) <= 63 {
		return name
	}

	// the machineName is always of hte form this-is-a-long-name-<randsuffix>-<randsuffix>
	// and we want to make sure to keep the random suffix, so that we don't have name collisions
	toRemove := len(name) - 63
	chunks := strings.Split(machineName, "-")
	suffix := strings.Join(chunks[len(chunks)-2:], "-")
	machineName = strings.Join(chunks[:len(chunks)-2], "-")
	machineName = strings.Join([]string{
		machineName[:len(machineName)-toRemove],
		suffix,
	}, "-")

	return a.innerName(machineName, diskNumber)
}

// the name is disk-## because there was a bug w/ oxide where only the first 20 chars were used to distinguish disks
// ref: https://docs.oxide.computer/guides/troubleshooting#_disk_rejected_due_to_20_char_name_match
func (a *additionalDisk) innerName(machineName string, diskNumber int) string {
	return fmt.Sprintf("disk-%02d-%s-%s", diskNumber, a.label, machineName)
}

func (a *additionalDisk) createInstanceDiskAttachment(i int, machineName string) oxide.InstanceDiskAttachment {
	diskBackend := oxide.DiskBackend{
		Value: &oxide.DiskBackendDistributed{
			DiskSource: oxide.DiskSource{
				Value: &oxide.DiskSourceBlank{
					BlockSize: oxide.BlockSize(4096),
				},
			},
		},
	}

	if a.backend == oxide.DiskBackendTypeLocal {
		diskBackend = oxide.DiskBackend{
			Value: &oxide.DiskBackendLocal{},
		}
	}

	return oxide.InstanceDiskAttachment{
		Value: &oxide.InstanceDiskAttachmentCreate{
			Description: defaultDescription,
			DiskBackend: diskBackend,
			Name:        oxide.Name(a.name(machineName, i)),
			Size:        oxide.ByteCount(a.size),
		},
	}
}
