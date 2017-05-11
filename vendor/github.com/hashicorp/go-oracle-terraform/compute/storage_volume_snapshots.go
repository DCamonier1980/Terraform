package compute

import (
	"fmt"
	"strings"
)

const (
	StorageVolumeSnapshotDescription   = "storage volume snapshot"
	StorageVolumeSnapshotContainerPath = "/storage/snapshot/"
	StorageVolumeSnapshotResourcePath  = "/storage/snapshot"

	WaitForSnapshotCreateTimeout = 1200
	WaitForSnapshotDeleteTimeout = 1500

	// Collocated Snapshot Property
	SnapshotPropertyCollocated = "/oracle/private/storage/snapshot/collocated"
)

// StorageVolumeSnapshotClient is a client for the Storage Volume Snapshot functions of the Compute API.
type StorageVolumeSnapshotClient struct {
	ResourceClient
}

func (c *Client) StorageVolumeSnapshots() *StorageVolumeSnapshotClient {
	return &StorageVolumeSnapshotClient{
		ResourceClient: ResourceClient{
			Client:              c,
			ResourceDescription: StorageVolumeSnapshotDescription,
			ContainerPath:       StorageVolumeSnapshotContainerPath,
			ResourceRootPath:    StorageVolumeSnapshotResourcePath,
		},
	}
}

// StorageVolumeSnapshotInfo represents the information retrieved from the service about a storage volume snapshot
type StorageVolumeSnapshotInfo struct {
	// Account to use for snapshots
	Account string `json:"account"`

	// Description of the snapshot
	Description string `json:"description"`

	// The name of the machine image that's used in the boot volume from which this snapshot is taken
	MachineImageName string `json:"machineimage_name"`

	// Name of the snapshot
	Name string `json:"name"`

	// String indicating whether the parent volume is bootable or not
	ParentVolumeBootable string `json:"parent_volume_bootable"`

	// Platform the snapshot is compatible with
	Platform string `json:"platform"`

	// String determining whether the snapshot is remote or collocated
	Property string `json:"property"`

	// The size of the snapshot in GB
	Size string `json:"size"`

	// The ID of the snapshot. Generated by the server
	SnapshotID string `json:"snapshot_id"`

	// The timestamp of the storage snapshot
	SnapshotTimestamp string `json:"snapshot_timestamp"`

	// Timestamp for when the operation started
	StartTimestamp string `json:"start_timestamp"`

	// Status of the snapshot
	Status string `json:"status"`

	// Status Detail of the storage snapshot
	StatusDetail string `json:"status_detail"`

	// Indicates the time that the current view of the storage volume snapshot was generated.
	StatusTimestamp string `json:"status_timestamp"`

	// Array of tags for the snapshot
	Tags []string `json:"tags,omitempty"`

	// Uniform Resource Identifier
	URI string `json:"uri"`

	// Name of the parent storage volume for the snapshot
	Volume string `json:"volume"`
}

// CreateStorageVolumeSnapshotInput represents the body of an API request to create a new storage volume snapshot
type CreateStorageVolumeSnapshotInput struct {
	// Description of the snapshot
	// Optional
	Description string `json:"description,omitempty"`

	// Name of the snapshot
	// Optional, will be generated if not specified
	Name string `json:"name,omitempty"`

	// Whether or not the parent volume is bootable
	// Optional
	ParentVolumeBootable string `json:"parent_volume_bootable,omitempty"`

	// Whether collocated or remote
	// Optional, will be remote if unspecified
	Property string `json:"property,omitempty"`

	// Array of tags for the snapshot
	// Optional
	Tags []string `json:"tags,omitempty"`

	// Name of the volume to create the snapshot from
	// Required
	Volume string `json:"volume"`

	// Timeout (in seconds) to wait for snapshot to be completed. Will use default if unspecified
	Timeout int
}

// CreateStorageVolumeSnapshot creates a snapshot based on the supplied information struct
func (c *StorageVolumeSnapshotClient) CreateStorageVolumeSnapshot(input *CreateStorageVolumeSnapshotInput) (*StorageVolumeSnapshotInfo, error) {
	if input.Name != "" {
		input.Name = c.getQualifiedName(input.Name)
	}
	input.Volume = c.getQualifiedName(input.Volume)

	var storageSnapshotInfo StorageVolumeSnapshotInfo
	if err := c.createResource(&input, &storageSnapshotInfo); err != nil {
		return nil, err
	}

	timeout := WaitForSnapshotCreateTimeout
	if input.Timeout != 0 {
		timeout = input.Timeout
	}

	// The name of the snapshot could have been generated. Use the response name as input
	return c.waitForStorageSnapshotAvailable(storageSnapshotInfo.Name, timeout)
}

// GetStorageVolumeSnapshotInput represents the body of an API request to get information on a storage volume snapshot
type GetStorageVolumeSnapshotInput struct {
	// Name of the snapshot
	Name string `json:"name"`
}

// GetStorageVolumeSnapshot makes an API request to populate information on a storage volume snapshot
func (c *StorageVolumeSnapshotClient) GetStorageVolumeSnapshot(input *GetStorageVolumeSnapshotInput) (*StorageVolumeSnapshotInfo, error) {
	var storageSnapshot StorageVolumeSnapshotInfo
	input.Name = c.getQualifiedName(input.Name)
	if err := c.getResource(input.Name, &storageSnapshot); err != nil {
		if WasNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}
	return c.success(&storageSnapshot)
}

// DeleteStorageVolumeSnapshotInput represents the body of an API request to delete a storage volume snapshot
type DeleteStorageVolumeSnapshotInput struct {
	// Name of the snapshot to delete
	Name string `json:"name"`

	// Timeout in seconds to wait for deletion, will use default if unspecified
	Timeout int
}

// DeleteStoragevolumeSnapshot makes an API request to delete a storage volume snapshot
func (c *StorageVolumeSnapshotClient) DeleteStorageVolumeSnapshot(input *DeleteStorageVolumeSnapshotInput) error {
	input.Name = c.getQualifiedName(input.Name)

	if err := c.deleteResource(input.Name); err != nil {
		return err
	}

	timeout := WaitForSnapshotDeleteTimeout
	if input.Timeout != 0 {
		timeout = input.Timeout
	}

	return c.waitForStorageSnapshotDeleted(input.Name, timeout)
}

func (c *StorageVolumeSnapshotClient) success(result *StorageVolumeSnapshotInfo) (*StorageVolumeSnapshotInfo, error) {
	c.unqualify(&result.Name)
	c.unqualify(&result.Volume)

	sizeInGigaBytes, err := sizeInGigaBytes(result.Size)
	if err != nil {
		return nil, err
	}
	result.Size = sizeInGigaBytes

	return result, nil
}

// Waits for a storage snapshot to become available
func (c *StorageVolumeSnapshotClient) waitForStorageSnapshotAvailable(name string, timeout int) (*StorageVolumeSnapshotInfo, error) {
	var result *StorageVolumeSnapshotInfo

	err := c.waitFor(
		fmt.Sprintf("storage volume snapshot %s to become available", c.getQualifiedName(name)),
		timeout,
		func() (bool, error) {
			req := &GetStorageVolumeSnapshotInput{
				Name: name,
			}
			res, err := c.GetStorageVolumeSnapshot(req)
			if err != nil {
				return false, err
			}

			if res != nil {
				result = res
				if strings.ToLower(result.Status) == "completed" {
					return true, nil
				} else if strings.ToLower(result.Status) == "error" {
					return false, fmt.Errorf("Snapshot '%s' failed to create successfully. Status: %s Status Detail: %s", result.Name, result.Status, result.StatusDetail)
				}
			}

			return false, nil
		})

	return result, err
}

// Waits for a storage snapshot to be deleted
func (c *StorageVolumeSnapshotClient) waitForStorageSnapshotDeleted(name string, timeout int) error {
	return c.waitFor(
		fmt.Sprintf("storage volume snapshot %s to be deleted", c.getQualifiedName(name)),
		timeout,
		func() (bool, error) {
			req := &GetStorageVolumeSnapshotInput{
				Name: name,
			}
			res, err := c.GetStorageVolumeSnapshot(req)
			if res == nil {
				return true, nil
			}

			if err != nil {
				return false, err
			}

			return res == nil, nil
		})
}
