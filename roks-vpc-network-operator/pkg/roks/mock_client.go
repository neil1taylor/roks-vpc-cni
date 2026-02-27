package roks

import (
	"context"
	"sync"
)

// Compile-time interface check.
var _ ROKSClient = (*MockROKSClient)(nil)

// MockROKSClient implements ROKSClient for testing.
// Each method can be overridden via function fields. If a function field is nil,
// the method returns a sensible default (empty slice, nil pointer, nil error).
// All calls are tracked in a thread-safe counter accessible via CallCount().
type MockROKSClient struct {
	mu    sync.Mutex
	calls map[string]int

	// VNI operations
	ListVNIsFn  func(ctx context.Context) ([]ROKSVNI, error)
	GetVNIFn    func(ctx context.Context, vniID string) (*ROKSVNI, error)
	GetVNIByVMFn func(ctx context.Context, namespace, vmName string) (*ROKSVNI, error)

	// VLAN Attachment operations
	ListVLANAttachmentsFn       func(ctx context.Context) ([]ROKSVLANAttachment, error)
	GetVLANAttachmentFn         func(ctx context.Context, attachmentID string) (*ROKSVLANAttachment, error)
	ListVLANAttachmentsByNodeFn func(ctx context.Context, nodeName string) ([]ROKSVLANAttachment, error)

	// Health / connectivity
	IsAvailableFn func(ctx context.Context) bool
}

// NewMockROKSClient creates a MockROKSClient with default behavior.
// IsAvailable returns true by default; all other methods return zero-value
// results with nil error unless overridden.
func NewMockROKSClient() *MockROKSClient {
	return &MockROKSClient{
		calls: make(map[string]int),
	}
}

func (m *MockROKSClient) trackCall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[name]++
}

// CallCount returns the number of times the named method was called.
func (m *MockROKSClient) CallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[method]
}

// ── VNI operations ──

func (m *MockROKSClient) ListVNIs(ctx context.Context) ([]ROKSVNI, error) {
	m.trackCall("ListVNIs")
	if m.ListVNIsFn != nil {
		return m.ListVNIsFn(ctx)
	}
	return nil, nil
}

func (m *MockROKSClient) GetVNI(ctx context.Context, vniID string) (*ROKSVNI, error) {
	m.trackCall("GetVNI")
	if m.GetVNIFn != nil {
		return m.GetVNIFn(ctx, vniID)
	}
	return nil, nil
}

func (m *MockROKSClient) GetVNIByVM(ctx context.Context, namespace, vmName string) (*ROKSVNI, error) {
	m.trackCall("GetVNIByVM")
	if m.GetVNIByVMFn != nil {
		return m.GetVNIByVMFn(ctx, namespace, vmName)
	}
	return nil, nil
}

// ── VLAN Attachment operations ──

func (m *MockROKSClient) ListVLANAttachments(ctx context.Context) ([]ROKSVLANAttachment, error) {
	m.trackCall("ListVLANAttachments")
	if m.ListVLANAttachmentsFn != nil {
		return m.ListVLANAttachmentsFn(ctx)
	}
	return nil, nil
}

func (m *MockROKSClient) GetVLANAttachment(ctx context.Context, attachmentID string) (*ROKSVLANAttachment, error) {
	m.trackCall("GetVLANAttachment")
	if m.GetVLANAttachmentFn != nil {
		return m.GetVLANAttachmentFn(ctx, attachmentID)
	}
	return nil, nil
}

func (m *MockROKSClient) ListVLANAttachmentsByNode(ctx context.Context, nodeName string) ([]ROKSVLANAttachment, error) {
	m.trackCall("ListVLANAttachmentsByNode")
	if m.ListVLANAttachmentsByNodeFn != nil {
		return m.ListVLANAttachmentsByNodeFn(ctx, nodeName)
	}
	return nil, nil
}

// ── Health / connectivity ──

func (m *MockROKSClient) IsAvailable(ctx context.Context) bool {
	m.trackCall("IsAvailable")
	if m.IsAvailableFn != nil {
		return m.IsAvailableFn(ctx)
	}
	// Default: available
	return true
}
