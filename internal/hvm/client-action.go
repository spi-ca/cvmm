package hvm

import (
	"errors"
)

// ClientAction is an enum-like value used when translating cvmm input or cloud-hypervisor state.
type ClientAction string

const (
	// ClientActionInvalid is the zero value used when parsing fails.
	ClientActionInvalid ClientAction = ""
	// ClientActionVmmPing calls the VMM ping endpoint.
	ClientActionVmmPing ClientAction = "vmm-ping"
	// ClientActionVmmShutdown calls the VMM shutdown endpoint.
	ClientActionVmmShutdown ClientAction = "vmm-shutdown"
	// ClientActionVmmNmi calls the VMM NMI endpoint.
	ClientActionVmmNmi ClientAction = "vmm-nmi"
	// ClientActionVmInfo retrieves VM information.
	ClientActionVmInfo ClientAction = "vm-info"
	// ClientActionVmCounters retrieves VM counters.
	ClientActionVmCounters ClientAction = "vm-counters"
	// ClientActionVmCreate sends a VM create request from stdin YAML.
	ClientActionVmCreate ClientAction = "vm-create"
	// ClientActionVmDelete deletes the VM.
	ClientActionVmDelete ClientAction = "vm-delete"
	// ClientActionVmBoot boots the created VM.
	ClientActionVmBoot ClientAction = "vm-boot"
	// ClientActionVmPause pauses the VM.
	ClientActionVmPause ClientAction = "vm-pause"
	// ClientActionVmResume resumes the VM.
	ClientActionVmResume ClientAction = "vm-resume"
	// ClientActionVmShutdown requests guest shutdown.
	ClientActionVmShutdown ClientAction = "vm-shutdown"
	// ClientActionVmReboot requests guest reboot.
	ClientActionVmReboot ClientAction = "vm-reboot"
	// ClientActionVmPowerButton sends a power-button event.
	ClientActionVmPowerButton ClientAction = "vm-power-button"
	// ClientActionVmResize sends VM resize data from stdin YAML.
	ClientActionVmResize ClientAction = "vm-resize"
	// ClientActionVmResizeZone sends memory-zone resize data from stdin YAML.
	ClientActionVmResizeZone ClientAction = "vm-resize-zone"
	// ClientActionVmAddDevice hot-adds a device from stdin YAML.
	ClientActionVmAddDevice ClientAction = "vm-add-device"
	// ClientActionVmAddUserDevice hot-adds a user device from stdin YAML.
	ClientActionVmAddUserDevice ClientAction = "vm-add-user-device"
	// ClientActionVmRemoveDevice removes a device from stdin YAML.
	ClientActionVmRemoveDevice ClientAction = "vm-remove-device"
	// ClientActionVmAddDisk hot-adds a disk from stdin YAML.
	ClientActionVmAddDisk ClientAction = "vm-add-disk"
	// ClientActionVmAddFs hot-adds a virtio-fs device from stdin YAML.
	ClientActionVmAddFs ClientAction = "vm-add-fs"
	// ClientActionVmAddPmem hot-adds persistent memory from stdin YAML.
	ClientActionVmAddPmem ClientAction = "vm-add-pmem"
	// ClientActionVmAddNet hot-adds a network device from stdin YAML.
	ClientActionVmAddNet ClientAction = "vm-add-net"
	// ClientActionVmAddVsock hot-adds a vsock device from stdin YAML.
	ClientActionVmAddVsock ClientAction = "vm-add-vsock"
	// ClientActionVmAddVdpa hot-adds a VDPA device from stdin YAML.
	ClientActionVmAddVdpa ClientAction = "vm-add-vdpa"
	// ClientActionVmSnapshot requests a snapshot using stdin YAML.
	ClientActionVmSnapshot ClientAction = "vm-snapshot"
	// ClientActionVmCoredump requests a coredump using stdin YAML.
	ClientActionVmCoredump ClientAction = "vm-coredump"
	// ClientActionVmRestore restores a VM using stdin YAML.
	ClientActionVmRestore ClientAction = "vm-restore"
	// ClientActionVmReceiveMigration starts receive migration using stdin YAML.
	ClientActionVmReceiveMigration ClientAction = "vm-receive-migration"
	// ClientActionVmSendMigration sends migration using stdin YAML.
	ClientActionVmSendMigration ClientAction = "vm-send-migration"
)

// ClientActions returns the supported CLI client actions in declaration order.
func ClientActions() []ClientAction {
	return []ClientAction{
		ClientActionVmmPing,
		ClientActionVmmShutdown,
		ClientActionVmmNmi,
		ClientActionVmInfo,
		ClientActionVmCounters,
		ClientActionVmCreate,
		ClientActionVmDelete,
		ClientActionVmBoot,
		ClientActionVmPause,
		ClientActionVmResume,
		ClientActionVmShutdown,
		ClientActionVmReboot,
		ClientActionVmPowerButton,
		ClientActionVmResize,
		ClientActionVmResizeZone,
		ClientActionVmAddDevice,
		ClientActionVmAddUserDevice,
		ClientActionVmRemoveDevice,
		ClientActionVmAddDisk,
		ClientActionVmAddFs,
		ClientActionVmAddPmem,
		ClientActionVmAddNet,
		ClientActionVmAddVsock,
		ClientActionVmAddVdpa,
		ClientActionVmSnapshot,
		ClientActionVmCoredump,
		ClientActionVmRestore,
		ClientActionVmReceiveMigration,
		ClientActionVmSendMigration,
	}
}

// ClientActionNameOf parses a CLI action name into a ClientAction value.
func ClientActionNameOf(value string) (ClientAction, error) {
	switch value {
	case string(ClientActionVmmPing):
		return ClientActionVmmPing, nil
	case string(ClientActionVmmShutdown):
		return ClientActionVmmShutdown, nil
	case string(ClientActionVmmNmi):
		return ClientActionVmmNmi, nil
	case string(ClientActionVmInfo):
		return ClientActionVmInfo, nil
	case string(ClientActionVmCounters):
		return ClientActionVmCounters, nil
	case string(ClientActionVmCreate):
		return ClientActionVmCreate, nil
	case string(ClientActionVmDelete):
		return ClientActionVmDelete, nil
	case string(ClientActionVmBoot):
		return ClientActionVmBoot, nil
	case string(ClientActionVmPause):
		return ClientActionVmPause, nil
	case string(ClientActionVmResume):
		return ClientActionVmResume, nil
	case string(ClientActionVmShutdown):
		return ClientActionVmShutdown, nil
	case string(ClientActionVmReboot):
		return ClientActionVmReboot, nil
	case string(ClientActionVmPowerButton):
		return ClientActionVmPowerButton, nil
	case string(ClientActionVmResize):
		return ClientActionVmResize, nil
	case string(ClientActionVmResizeZone):
		return ClientActionVmResizeZone, nil
	case string(ClientActionVmAddDevice):
		return ClientActionVmAddDevice, nil
	case string(ClientActionVmAddUserDevice):
		return ClientActionVmAddUserDevice, nil
	case string(ClientActionVmRemoveDevice):
		return ClientActionVmRemoveDevice, nil
	case string(ClientActionVmAddDisk):
		return ClientActionVmAddDisk, nil
	case string(ClientActionVmAddFs):
		return ClientActionVmAddFs, nil
	case string(ClientActionVmAddPmem):
		return ClientActionVmAddPmem, nil
	case string(ClientActionVmAddNet):
		return ClientActionVmAddNet, nil
	case string(ClientActionVmAddVsock):
		return ClientActionVmAddVsock, nil
	case string(ClientActionVmAddVdpa):
		return ClientActionVmAddVdpa, nil
	case string(ClientActionVmSnapshot):
		return ClientActionVmSnapshot, nil
	case string(ClientActionVmCoredump):
		return ClientActionVmCoredump, nil
	case string(ClientActionVmRestore):
		return ClientActionVmRestore, nil
	case string(ClientActionVmReceiveMigration):
		return ClientActionVmReceiveMigration, nil
	case string(ClientActionVmSendMigration):
		return ClientActionVmSendMigration, nil
	default:
		return ClientActionInvalid, errors.New("unable to find ClientAction")
	}
}

// IsValid reports whether the receiver is one of the supported values.
func (ts ClientAction) IsValid() bool {
	switch ts {
	case ClientActionVmmPing:
		return true
	case ClientActionVmmShutdown:
		return true
	case ClientActionVmmNmi:
		return true
	case ClientActionVmInfo:
		return true
	case ClientActionVmCounters:
		return true
	case ClientActionVmCreate:
		return true
	case ClientActionVmDelete:
		return true
	case ClientActionVmBoot:
		return true
	case ClientActionVmPause:
		return true
	case ClientActionVmResume:
		return true
	case ClientActionVmShutdown:
		return true
	case ClientActionVmReboot:
		return true
	case ClientActionVmPowerButton:
		return true
	case ClientActionVmResize:
		return true
	case ClientActionVmResizeZone:
		return true
	case ClientActionVmAddDevice:
		return true
	case ClientActionVmAddUserDevice:
		return true
	case ClientActionVmRemoveDevice:
		return true
	case ClientActionVmAddDisk:
		return true
	case ClientActionVmAddFs:
		return true
	case ClientActionVmAddPmem:
		return true
	case ClientActionVmAddNet:
		return true
	case ClientActionVmAddVsock:
		return true
	case ClientActionVmAddVdpa:
		return true
	case ClientActionVmSnapshot:
		return true
	case ClientActionVmCoredump:
		return true
	case ClientActionVmRestore:
		return true
	case ClientActionVmReceiveMigration:
		return true
	case ClientActionVmSendMigration:
		return true
	default:
		return false
	}
}

// String returns the CLI action token for a valid ClientAction.
func (ts ClientAction) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

// MarshalText returns the textual representation used by encoders.
func (ts ClientAction) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

// UnmarshalText parses a textual value into the receiver.
func (ts *ClientAction) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	new_status, err := ClientActionNameOf(string(b))
	if err != nil {
		return err
	}

	*ts = new_status
	return nil
}
