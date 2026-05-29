package main

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/oxidecomputer/oxide.go/oxide"
)

const (
	additionalDiskSize        = "size"
	additionalDiskLabel       = "label"
	additionalDiskBackendType = "type"
)

// additionalDiskParser parses a string describing an additional disk.
type additionalDiskParser struct {
	// additionalDisk is the final parsed and validated disk.
	additionalDisk

	// sizeStr is a temporary field used to store the input string for size.
	sizeStr string
}

// parse parses a string in the format `size=SIZE[,label=LABEL][,type=TYPE]` or
// `SIZE[[,LABEL],TYPE]` where :
//   - `SIZE` is the disk size in bytes. It supports a unit suffix (e.g., 20 GiB).
//   - `LABEL` is an arbitrary string used within the disk name for identification.
//   - `TYPE` is the Oxide `disk_backend`: `local` or `distributed`.
func (a *additionalDiskParser) parse(s string) error {
	if s == "" {
		return fmt.Errorf("invalid format empty string given, expected size[[,label],backend]")
	}

	// Default values.
	a.label = "additional"
	a.backend = oxide.DiskBackendTypeDistributed

	// Parse input string.
	var err error
	if !strings.Contains(s, "=") {
		err = a.parseLegacy(s)
	} else {
		err = a.parseKV(s)
	}
	if err != nil {
		return err
	}

	// Parse and validate values.
	a.size, err = humanize.ParseBytes(a.sizeStr)
	if err != nil {
		return fmt.Errorf("failed parsing size %q: %w", a.sizeStr, err)
	}

	if a.backend != oxide.DiskBackendTypeDistributed && a.backend != oxide.DiskBackendTypeLocal {
		return fmt.Errorf("invalid backend %q, expected %q or %q", a.backend, oxide.DiskBackendTypeDistributed, oxide.DiskBackendTypeLocal)
	}

	return nil
}

// parseLegacy parses strings in the format `SIZE[[,LABEL],TYPE]`.
//
// Deprecated: This format has been deprecated in favor of the KV format and it
// does not need to be updated to support new fields.
func (a *additionalDiskParser) parseLegacy(s string) error {
	fields := strings.Split(s, ",")
	switch len(fields) {
	case 3:
		if s := strings.TrimSpace(fields[2]); s != "" {
			a.backend = oxide.DiskBackendType(s)
		}
		fallthrough
	case 2:
		if s := strings.TrimSpace(fields[1]); s != "" {
			a.label = s
		}
		fallthrough
	case 1:
		a.sizeStr = strings.TrimSpace(fields[0])
	default:
		return fmt.Errorf("invalid format %q, expected size[[,label],backend]", s)
	}

	return nil
}

// parseKV parses strings in the format `size=SIZE[,label=LABEL][,type=TYPE]`.
func (a *additionalDiskParser) parseKV(s string) error {
	for p := range strings.SplitSeq(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		part := strings.Split(p, "=")
		if len(part) != 2 {
			return fmt.Errorf(`invalid format %q, expected "name=value"`, p)
		}

		name := strings.TrimSpace(part[0])
		value := strings.TrimSpace(part[1])

		switch name {
		case additionalDiskSize:
			a.sizeStr = value
		case additionalDiskLabel:
			if value != "" {
				a.label = value
			}
		case additionalDiskBackendType:
			if value != "" {
				a.backend = oxide.DiskBackendType(value)
			}
		default:
			return fmt.Errorf("invalid additional disk part: %q, expected %q, %q, or %q", name,
				additionalDiskSize,
				additionalDiskLabel,
				additionalDiskBackendType,
			)
		}
	}

	return nil
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

// parseAdditionalDisk parses an `AdditionalDisk` from a string.
func parseAdditionalDisk(s string) (additionalDisk, error) {
	a := additionalDiskParser{}
	if err := a.parse(s); err != nil {
		return additionalDisk{}, err
	}
	return a.additionalDisk, nil
}
