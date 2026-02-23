// Package roks provides an abstraction layer for ROKS-managed VPC resources.
//
// On ROKS clusters, certain VPC resources (VNIs and VLAN attachments) are managed
// by the ROKS platform and cannot be accessed directly via the VPC API. Instead,
// they must be managed through a dedicated ROKS API.
//
// This package defines the ROKSClient interface that will be implemented when the
// ROKS API becomes available. Until then, a stub implementation returns
// ErrROKSAPINotAvailable for all operations.
//
// On unmanaged (non-ROKS) clusters, these resources are accessed directly via
// the VPC API and this package is not used.
package roks

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrROKSAPINotAvailable is returned by the stub client when the ROKS API
// is not yet implemented.
var ErrROKSAPINotAvailable = errors.New("ROKS API for VNI/VLAN management is not yet available")

// ClusterMode indicates whether the operator is running on a ROKS-managed
// or unmanaged cluster. This determines which API path is used for VNIs
// and VLAN attachments.
type ClusterMode string

const (
	// ModeROKS indicates a ROKS-managed cluster where VNIs and VLAN
	// attachments are managed by the ROKS platform.
	ModeROKS ClusterMode = "roks"

	// ModeUnmanaged indicates a non-ROKS cluster where VNIs and VLAN
	// attachments are managed directly via the VPC API.
	ModeUnmanaged ClusterMode = "unmanaged"
)

// ── VNI types (ROKS-managed) ──

// ROKSVNI represents a Virtual Network Interface as returned by the ROKS API.
// The exact fields may differ from the VPC API representation once the ROKS
// API is finalized. This type provides the abstraction layer.
type ROKSVNI struct {
	// ID is the ROKS-assigned VNI identifier.
	ID string

	// VPCVNIID is the underlying VPC VNI ID (if exposed by ROKS API).
	VPCVNIID string

	// Name is the VNI name.
	Name string

	// MACAddress is the MAC address assigned to the VNI.
	MACAddress string

	// PrimaryIPv4 is the primary IP address on the VNI.
	PrimaryIPv4 string

	// SubnetID is the VPC subnet the VNI belongs to.
	SubnetID string

	// SecurityGroupIDs lists the security groups attached to this VNI.
	SecurityGroupIDs []string

	// VMName is the name of the VirtualMachine using this VNI, if known.
	VMName string

	// VMNamespace is the namespace of the VirtualMachine, if known.
	VMNamespace string

	// Status is the VNI status as reported by ROKS.
	Status string

	// CreatedAt is when the VNI was created.
	CreatedAt time.Time
}

// ── VLAN Attachment types (ROKS-managed) ──

// ROKSVLANAttachment represents a VLAN attachment on a bare metal server
// as returned by the ROKS API.
type ROKSVLANAttachment struct {
	// ID is the ROKS-assigned attachment identifier.
	ID string

	// VPCAttachmentID is the underlying VPC attachment ID (if exposed).
	VPCAttachmentID string

	// BMServerID is the bare metal server this VLAN is attached to.
	BMServerID string

	// NodeName is the Kubernetes node name for the bare metal server.
	NodeName string

	// VLANID is the VLAN tag.
	VLANID int64

	// SubnetID is the VPC subnet associated with this attachment.
	SubnetID string

	// Status is the attachment status.
	Status string

	// CreatedAt is when the attachment was created.
	CreatedAt time.Time
}

// ── ROKSClient interface ──

// ROKSClient defines the interface for managing VPC resources that are
// controlled by the ROKS platform. This interface will be implemented
// when the ROKS API becomes available.
//
// TODO(roks-api): Implement this interface when the ROKS API is ready.
// The method signatures below are provisional and may need to be adjusted
// based on the final ROKS API design.
type ROKSClient interface {
	// VNI operations

	// ListVNIs returns all VNIs managed by ROKS for this cluster.
	ListVNIs(ctx context.Context) ([]ROKSVNI, error)

	// GetVNI returns a specific VNI by its ROKS ID.
	GetVNI(ctx context.Context, vniID string) (*ROKSVNI, error)

	// GetVNIByVM returns the VNI associated with a specific VM.
	GetVNIByVM(ctx context.Context, namespace, vmName string) (*ROKSVNI, error)

	// VLAN Attachment operations

	// ListVLANAttachments returns all VLAN attachments managed by ROKS for this cluster.
	ListVLANAttachments(ctx context.Context) ([]ROKSVLANAttachment, error)

	// GetVLANAttachment returns a specific VLAN attachment by ID.
	GetVLANAttachment(ctx context.Context, attachmentID string) (*ROKSVLANAttachment, error)

	// ListVLANAttachmentsByNode returns VLAN attachments for a specific node.
	ListVLANAttachmentsByNode(ctx context.Context, nodeName string) ([]ROKSVLANAttachment, error)

	// Health / connectivity

	// IsAvailable returns true if the ROKS API is reachable and ready.
	IsAvailable(ctx context.Context) bool
}

// ── ROKSClientConfig ──

// ROKSClientConfig holds configuration for creating a ROKS API client.
// TODO(roks-api): Update fields when ROKS API auth/endpoint details are known.
type ROKSClientConfig struct {
	// ClusterID is the ROKS cluster ID.
	ClusterID string

	// Region is the IBM Cloud region.
	Region string

	// APIEndpoint is the ROKS API endpoint URL.
	// TODO(roks-api): Set when known. May be auto-discovered from cluster metadata.
	APIEndpoint string

	// AuthToken is the authentication token for the ROKS API.
	// TODO(roks-api): Determine auth mechanism (IAM token, service account, etc.)
	AuthToken string
}

// ── Stub implementation ──

// stubClient is the default implementation used until the ROKS API is available.
// All methods return ErrROKSAPINotAvailable.
type stubClient struct {
	clusterID string
}

// NewStubClient creates a ROKSClient stub that returns ErrROKSAPINotAvailable
// for all operations. Use this on ROKS clusters until the real API is ready.
func NewStubClient(clusterID string) ROKSClient {
	return &stubClient{clusterID: clusterID}
}

// NewClient creates a ROKSClient.
// TODO(roks-api): Implement real client initialization when the API is available.
// For now, this always returns a stub client.
func NewClient(cfg ROKSClientConfig) (ROKSClient, error) {
	// TODO(roks-api): Replace with real client implementation:
	//
	// client, err := roksapi.NewClient(&roksapi.ClientOptions{
	//     ClusterID: cfg.ClusterID,
	//     Region:    cfg.Region,
	//     Endpoint:  cfg.APIEndpoint,
	//     Auth:      cfg.AuthToken,
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("failed to create ROKS client: %w", err)
	// }
	// return client, nil

	return NewStubClient(cfg.ClusterID), nil
}

func (s *stubClient) ListVNIs(ctx context.Context) ([]ROKSVNI, error) {
	return nil, fmt.Errorf("ListVNIs on cluster %s: %w", s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) GetVNI(ctx context.Context, vniID string) (*ROKSVNI, error) {
	return nil, fmt.Errorf("GetVNI(%s) on cluster %s: %w", vniID, s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) GetVNIByVM(ctx context.Context, namespace, vmName string) (*ROKSVNI, error) {
	return nil, fmt.Errorf("GetVNIByVM(%s/%s) on cluster %s: %w", namespace, vmName, s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) ListVLANAttachments(ctx context.Context) ([]ROKSVLANAttachment, error) {
	return nil, fmt.Errorf("ListVLANAttachments on cluster %s: %w", s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) GetVLANAttachment(ctx context.Context, attachmentID string) (*ROKSVLANAttachment, error) {
	return nil, fmt.Errorf("GetVLANAttachment(%s) on cluster %s: %w", attachmentID, s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) ListVLANAttachmentsByNode(ctx context.Context, nodeName string) ([]ROKSVLANAttachment, error) {
	return nil, fmt.Errorf("ListVLANAttachmentsByNode(%s) on cluster %s: %w", nodeName, s.clusterID, ErrROKSAPINotAvailable)
}

func (s *stubClient) IsAvailable(ctx context.Context) bool {
	return false
}
