package hvm

import (
	"amuz.es/src/spi-ca/cvmm/internal/model"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

type (
	Client interface {
		VmmPing(ctx context.Context) (*model.VmmPingResponse, error)
		VmmShutdown(ctx context.Context) error
		VmInfo(ctx context.Context) (*model.VmInfo, error)
		VmCounters(ctx context.Context) (*model.VmCounters, error)
		VmCreate(ctx context.Context, config model.VmConfig) error
		VmDelete(ctx context.Context) error
		VmBoot(ctx context.Context) error
		VmPause(ctx context.Context) error
		VmResume(ctx context.Context) error
		VmShutdown(ctx context.Context) error
		VmReboot(ctx context.Context) error
		VmPowerButton(ctx context.Context) error
		VmResize(ctx context.Context, config model.VmResize) error
		VmResizeZone(ctx context.Context, config model.VmResizeZone) error
		VmAddDevice(ctx context.Context, config model.DeviceConfig) (*model.PciDeviceInfo, error)
		VmRemoveDevice(ctx context.Context, config model.VmRemoveDevice) error
		VmAddDisk(ctx context.Context, config model.DiskConfig) (*model.PciDeviceInfo, error)
		VmAddFs(ctx context.Context, config model.FsConfig) (*model.PciDeviceInfo, error)
		VmAddPmem(ctx context.Context, config model.PmemConfig) (*model.PciDeviceInfo, error)
		VmAddNet(ctx context.Context, config model.NetConfig) (*model.PciDeviceInfo, error)
		VmAddVsock(ctx context.Context, config model.VsockConfig) (*model.PciDeviceInfo, error)
		VmAddVdpa(ctx context.Context, config model.VdpaConfig) (*model.PciDeviceInfo, error)
		VmSnapshot(ctx context.Context, config model.VmSnapshotConfig) error
		VmCoredump(ctx context.Context, config model.VmCoredumpData) error
		VmRestore(ctx context.Context, config model.RestoreConfig) error
		VmReceiveMigration(ctx context.Context, config model.ReceiveMigrationData) error
		VmSendMigration(ctx context.Context, config model.SendMigrationData) error
	}

	clientImpl struct {
		cli        http.Client
		socketPath string
		wg         sync.WaitGroup
	}
)

const (
	clientUrlVmmPing            = "http://localhost/api/v1/vmm.ping"
	clientUrlVmmShutdown        = "http://localhost/api/v1/vmm.shutdown"
	clientUrlVmInfo             = "http://localhost/api/v1/vm.info"
	clientUrlVmCounters         = "http://localhost/api/v1/vm.counters"
	clientUrlVmCreate           = "http://localhost/api/v1/vm.create"
	clientUrlVmDelete           = "http://localhost/api/v1/vm.delete"
	clientUrlVmBoot             = "http://localhost/api/v1/vm.boot"
	clientUrlVmPause            = "http://localhost/api/v1/vm.pause"
	clientUrlVmResume           = "http://localhost/api/v1/vm.resume"
	clientUrlVmShutdown         = "http://localhost/api/v1/vm.shutdown"
	clientUrlVmReboot           = "http://localhost/api/v1/vm.reboot"
	clientUrlVmPowerButton      = "http://localhost/api/v1/vm.power-button"
	clientUrlVmResize           = "http://localhost/api/v1/vm.resize"
	clientUrlVmResizeZone       = "http://localhost/api/v1/vm.resize-zone"
	clientUrlVmAddDevice        = "http://localhost/api/v1/vm.add-device"
	clientUrlVmRemoveDevice     = "http://localhost/api/v1/vm.remove-device"
	clientUrlVmAddDisk          = "http://localhost/api/v1/vm.add-disk"
	clientUrlVmAddFs            = "http://localhost/api/v1/vm.add-fs"
	clientUrlVmAddPmem          = "http://localhost/api/v1/vm.add-pmem"
	clientUrlVmAddNet           = "http://localhost/api/v1/vm.add-net"
	clientUrlVmAddVsock         = "http://localhost/api/v1/vm.add-vsock"
	clientUrlVmAddVdpa          = "http://localhost/api/v1/vm.add-vdpa"
	clientUrlVmSnapshot         = "http://localhost/api/v1/vm.snapshot"
	clientUrlVmCoredump         = "http://localhost/api/v1/vm.coredump"
	clientUrlVmRestore          = "http://localhost/api/v1/vm.restore"
	clientUrlVmReceiveMigration = "http://localhost/api/v1/vm.receive-migration"
	clientUrlVmSendMigration    = "http://localhost/api/v1/vm.send-migration"
)

var (
	ErrVmNotCreated         = errors.New("VM instance is not created")
	ErrVmNotStarted         = errors.New("VM instance is not started")
	ErrVmNotBooted          = errors.New("VM instance is not booted")
	ErrVmNotPaused          = errors.New("VM instance is not paused")
	ErrVmAlreadyCreated     = errors.New("VM instance is already")
	ErrRedirectionForbidded = errors.New("this client cannot redirect")
)

func NewClient(socketPath string) Client { return newClient(socketPath) }
func newClient(socketPath string) *clientImpl {
	c := clientImpl{
		socketPath: socketPath,
	}

	c.cli.Transport = &http.Transport{
		DialContext: c.dialContext,
	}
	c.cli.CheckRedirect = c.checkRedirect

	return &c
}

func (c *clientImpl) Close() {
	c.cli.CloseIdleConnections()
	c.wg.Wait()
}

// Ping the VMM to check for API server availability
func (c *clientImpl) VmmPing(ctx context.Context) (*model.VmmPingResponse, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientUrlVmmPing, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmmPing, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmmPing, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	default:
		return nil, fmt.Errorf("failed to execute VmmPing: http error(%d) %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	obj := model.VmmPingResponse{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

// Shuts the cloud-hypervisor VMM.
func (c *clientImpl) VmmShutdown(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmmShutdown, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmmShutdown, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmmShutdown, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("failed to execute VmmShutdown: http error(%d) %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}

// Returns general information about the cloud-hypervisor Virtual Machine (VM) instance.
func (c *clientImpl) VmInfo(ctx context.Context) (*model.VmInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientUrlVmInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmInfo, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmInfo, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	default:
		return nil, fmt.Errorf("failed to execute VmInfo: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.VmInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmInfo, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Get counters from the VM
func (c *clientImpl) VmCounters(ctx context.Context) (*model.VmCounters, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientUrlVmCounters, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmCounters, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmCounters, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	default:
		return nil, fmt.Errorf("failed to execute VmCounters: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.VmCounters{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmCounters, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Create the cloud-hypervisor Virtual Machine (VM) instance. The instance is not booted, only created.
func (c *clientImpl) VmCreate(ctx context.Context, config model.VmConfig) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmConfig: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmCreate, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmCreate, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmCreate, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("failed to execute VmCreate: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Delete the cloud-hypervisor Virtual Machine (VM) instance.
func (c *clientImpl) VmDelete(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmDelete, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmDelete, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmDelete, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("failed to execute VmDelete: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Boot the previously created VM instance.
func (c *clientImpl) VmBoot(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmBoot, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmBoot, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmBoot, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmBoot: %w", ErrVmNotCreated)
	default:
		return fmt.Errorf("failed to execute VmBoot: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Pause a previously booted VM instance.
func (c *clientImpl) VmPause(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmPause, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmPause, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmPause, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmPause: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmPause: %w", ErrVmNotBooted)
	default:
		return fmt.Errorf("failed to execute VmPause: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Resume a previously paused VM instance.
func (c *clientImpl) VmResume(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmResume, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmResume, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmResume, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmResume: %w", ErrVmNotBooted)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmResume: %w", ErrVmNotPaused)
	default:
		return fmt.Errorf("failed to execute VmResume: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Shut the VM instance down.
func (c *clientImpl) VmShutdown(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmShutdown, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmShutdown, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmShutdown, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmShutdown: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmShutdown: %w", ErrVmNotStarted)
	default:
		return fmt.Errorf("failed to execute VmShutdown: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Reboot the VM instance.
func (c *clientImpl) VmReboot(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmReboot, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmReboot, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmReboot, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmReboot: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmReboot: %w", ErrVmNotBooted)
	default:
		return fmt.Errorf("failed to execute VmReboot: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Trigger a power button in the VM.
func (c *clientImpl) VmPowerButton(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmPowerButton, nil)
	if err != nil {
		return fmt.Errorf("failed to execute VmPowerButton, http request creation failed : %w", err)
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmPowerButton, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmPowerButton: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmPowerButton: %w", ErrVmNotBooted)
	default:
		return fmt.Errorf("failed to execute VmPowerButton: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Resize the VM
func (c *clientImpl) VmResize(ctx context.Context, config model.VmResize) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmResize: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmResize, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmResize, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmResize, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmResize: %w", ErrVmNotCreated)
	default:
		return fmt.Errorf("failed to execute VmResize: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Resize a memory zone
func (c *clientImpl) VmResizeZone(ctx context.Context, config model.VmResizeZone) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmResizeZone: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmResizeZone, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmResizeZone, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmResizeZone, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusInternalServerError:
		return fmt.Errorf("failed to execute VmResizeZone")
	default:
		return fmt.Errorf("failed to execute VmResizeZone: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Add a new device to the VM
func (c *clientImpl) VmAddDevice(ctx context.Context, config model.DeviceConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddDevice: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddDevice, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDevice, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDevice, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("failed to execute VmAddDevice")
	default:
		return nil, fmt.Errorf("failed to execute VmResizeZone: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDevice, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Remove a device from the VM
func (c *clientImpl) VmRemoveDevice(ctx context.Context, config model.VmRemoveDevice) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmRemoveDevice: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmRemoveDevice, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmRemoveDevice, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmRemoveDevice, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmRemoveDevice")
	default:
		return fmt.Errorf("failed to execute VmRemoveDevice: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Add a new disk to the VM
func (c *clientImpl) VmAddDisk(ctx context.Context, config model.DiskConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddDisk: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddDisk, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDisk, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDisk, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddDisk")
	default:
		return nil, fmt.Errorf("failed to execute VmAddDisk: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddDisk, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Add a new virtio-fs device to the VM
func (c *clientImpl) VmAddFs(ctx context.Context, config model.FsConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddFs: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddFs, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddFs, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddFs, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddFs")
	default:
		return nil, fmt.Errorf("failed to execute VmAddFs: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddFs, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Add a new pmem device to the VM
func (c *clientImpl) VmAddPmem(ctx context.Context, config model.PmemConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddPmem: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddPmem, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddPmem, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddPmem, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddPmem")
	default:
		return nil, fmt.Errorf("failed to execute VmAddPmem: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddPmem, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Add a new network device to the VM
func (c *clientImpl) VmAddNet(ctx context.Context, config model.NetConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddNet: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddNet, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddNet, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddNet, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddNet")
	default:
		return nil, fmt.Errorf("failed to execute VmAddNet: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddNet, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Add a new vsock device to the VM
func (c *clientImpl) VmAddVsock(ctx context.Context, config model.VsockConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddVsock: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddVsock, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVsock, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVsock, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddVsock")
	default:
		return nil, fmt.Errorf("failed to execute VmAddVsock: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVsock, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Add a new vDPA device to the VM
func (c *clientImpl) VmAddVdpa(ctx context.Context, config model.VdpaConfig) (*model.PciDeviceInfo, error) {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode VmAddVdpa: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmAddVdpa, bytes.NewReader(reqBuf))
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVdpa, http request creation failed : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVdpa, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNoContent:
		return nil, nil
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("failed to execute VmAddVdpa")
	default:
		return nil, fmt.Errorf("failed to execute VmAddVdpa: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}

	obj := model.PciDeviceInfo{}
	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to execute VmAddVdpa, failed to decode JSON response : %w", err)
	}

	return &obj, nil
}

// Returns a VM snapshot
func (c *clientImpl) VmSnapshot(ctx context.Context, config model.VmSnapshotConfig) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmSnapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmSnapshot, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmSnapshot, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmSnapshot, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmSnapshot: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmSnapshot: %w", ErrVmNotBooted)
	default:
		return fmt.Errorf("failed to execute VmSnapshot: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Takes a VM coredump
func (c *clientImpl) VmCoredump(ctx context.Context, config model.VmCoredumpData) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmCoredump: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmCoredump, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmCoredump, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmCoredump, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmCoredump: %w", ErrVmNotCreated)
	case http.StatusMethodNotAllowed:
		return fmt.Errorf("failed to execute VmCoredump: %w", ErrVmNotBooted)
	default:
		return fmt.Errorf("failed to execute VmCoredump: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Restore a VM from a snapshot
func (c *clientImpl) VmRestore(ctx context.Context, config model.RestoreConfig) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmRestore: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmRestore, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmRestore, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmRestore, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("failed to execute VmRestore: %w", ErrVmAlreadyCreated)
	default:
		return fmt.Errorf("failed to execute VmRestore: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Receive a VM migration from URL
func (c *clientImpl) VmReceiveMigration(ctx context.Context, config model.ReceiveMigrationData) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmReceiveMigration: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmReceiveMigration, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmReceiveMigration, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmReceiveMigration, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusInternalServerError:
		return fmt.Errorf("failed to execute VmReceiveMigration: migration could not be received")
	default:
		return fmt.Errorf("failed to execute VmReceiveMigration: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

// Send a VM migration to URL
func (c *clientImpl) VmSendMigration(ctx context.Context, config model.SendMigrationData) error {
	c.wg.Add(1)
	defer c.wg.Done()

	reqBuf, err := json.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to encode VmSendMigration: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, clientUrlVmSendMigration, bytes.NewReader(reqBuf))
	if err != nil {
		return fmt.Errorf("failed to execute VmSendMigration, http request creation failed : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute VmSendMigration, https request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusInternalServerError:
		return fmt.Errorf("failed to execute VmSendMigration: migration could not be sent")
	default:
		return fmt.Errorf("failed to execute VmSendMigration: http error(%d) %s", resp.StatusCode, c.readResponseMessage(resp))
	}
}

func (c *clientImpl) dialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
}

func (c *clientImpl) checkRedirect(_ *http.Request, via []*http.Request) error {
	return ErrRedirectionForbidded
}

func (c *clientImpl) readResponseMessage(resp *http.Response) string {
	buf := strings.Builder{}
	_, _ = io.CopyN(&buf, resp.Body, resp.ContentLength)
	return buf.String()
}
