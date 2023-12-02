package hvm

import (
	"errors"
)

type ClientAction string

const (
	ClientActionInvalid            ClientAction = ""
	ClientActionVmmPing            ClientAction = "vmm-ping"
	ClientActionVmmShutdown        ClientAction = "vmm-shutdown"
	ClientActionVmInfo             ClientAction = "vm-info"
	ClientActionVmCounters         ClientAction = "vm-counters"
	ClientActionVmCreate           ClientAction = "vm-create"
	ClientActionVmDelete           ClientAction = "vm-delete"
	ClientActionVmBoot             ClientAction = "vm-boot"
	ClientActionVmPause            ClientAction = "vm-pause"
	ClientActionVmResume           ClientAction = "vm-resume"
	ClientActionVmShutdown         ClientAction = "vm-shutdown"
	ClientActionVmReboot           ClientAction = "vm-reboot"
	ClientActionVmPowerButton      ClientAction = "vm-power-button"
	ClientActionVmResize           ClientAction = "vm-resize"
	ClientActionVmResizeZone       ClientAction = "vm-resize-zone"
	ClientActionVmAddDevice        ClientAction = "vm-add-device"
	ClientActionVmAddUserDevice    ClientAction = "vm-add-user-device"
	ClientActionVmRemoveDevice     ClientAction = "vm-remove-device"
	ClientActionVmAddDisk          ClientAction = "vm-add-disk"
	ClientActionVmAddFs            ClientAction = "vm-add-fs"
	ClientActionVmAddPmem          ClientAction = "vm-add-pmem"
	ClientActionVmAddNet           ClientAction = "vm-add-net"
	ClientActionVmAddVsock         ClientAction = "vm-add-vsock"
	ClientActionVmAddVdpa          ClientAction = "vm-add-vdpa"
	ClientActionVmSnapshot         ClientAction = "vm-snapshot"
	ClientActionVmCoredump         ClientAction = "vm-coredump"
	ClientActionVmRestore          ClientAction = "vm-restore"
	ClientActionVmReceiveMigration ClientAction = "vm-receive-migration"
	ClientActionVmSendMigration    ClientAction = "vm-send-migration"
)

func ClientActions() []ClientAction {
	return []ClientAction{
		ClientActionVmmPing,
		ClientActionVmmShutdown,
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
func ClientActionNameOf(value string) (ClientAction, error) {
	switch value {
	case string(ClientActionVmmPing):
		return ClientActionVmmPing, nil
	case string(ClientActionVmmShutdown):
		return ClientActionVmmShutdown, nil
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
		return ClientActionVmAddUserDevice, nil
	case string(ClientActionVmAddUserDevice):
		return ClientActionVmAddDevice, nil
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

func (ts ClientAction) IsValid() bool {
	switch ts {
	case ClientActionVmmPing:
		return true
	case ClientActionVmmShutdown:
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

func (ts ClientAction) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

func (ts ClientAction) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

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
