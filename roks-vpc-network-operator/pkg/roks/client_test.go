package roks

import (
	"context"
	"errors"
	"testing"
)

func TestStubClient_AllMethodsReturnErrROKSAPINotAvailable(t *testing.T) {
	client := NewStubClient("test")
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "ListVNIs",
			fn: func() error {
				_, err := client.ListVNIs(ctx)
				return err
			},
		},
		{
			name: "GetVNI",
			fn: func() error {
				_, err := client.GetVNI(ctx, "vni-123")
				return err
			},
		},
		{
			name: "GetVNIByVM",
			fn: func() error {
				_, err := client.GetVNIByVM(ctx, "default", "my-vm")
				return err
			},
		},
		{
			name: "ListVLANAttachments",
			fn: func() error {
				_, err := client.ListVLANAttachments(ctx)
				return err
			},
		},
		{
			name: "GetVLANAttachment",
			fn: func() error {
				_, err := client.GetVLANAttachment(ctx, "att-123")
				return err
			},
		},
		{
			name: "ListVLANAttachmentsByNode",
			fn: func() error {
				_, err := client.ListVLANAttachmentsByNode(ctx, "node-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tt.name)
			}
			if !errors.Is(err, ErrROKSAPINotAvailable) {
				t.Errorf("%s: expected error to wrap ErrROKSAPINotAvailable, got: %v", tt.name, err)
			}
		})
	}
}

func TestStubClient_IsAvailable_ReturnsFalse(t *testing.T) {
	client := NewStubClient("test")
	if client.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() to return false for stub client")
	}
}

func TestNewClient_ReturnsStub(t *testing.T) {
	client, err := NewClient(ROKSClientConfig{})
	if err != nil {
		t.Fatalf("NewClient() returned unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	if client.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() to return false for client from NewClient (stub)")
	}
}

func TestMockROKSClient_DefaultAvailable(t *testing.T) {
	mock := NewMockROKSClient()
	if !mock.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() to return true for default MockROKSClient")
	}
}

func TestMockROKSClient_CustomBehavior(t *testing.T) {
	mock := NewMockROKSClient()

	expectedVNIs := []ROKSVNI{
		{ID: "vni-1", Name: "test-vni-1"},
		{ID: "vni-2", Name: "test-vni-2"},
	}

	mock.ListVNIsFn = func(ctx context.Context) ([]ROKSVNI, error) {
		return expectedVNIs, nil
	}

	vnis, err := mock.ListVNIs(context.Background())
	if err != nil {
		t.Fatalf("ListVNIs() returned unexpected error: %v", err)
	}
	if len(vnis) != 2 {
		t.Errorf("expected 2 VNIs, got %d", len(vnis))
	}
	if vnis[0].ID != "vni-1" {
		t.Errorf("expected first VNI ID 'vni-1', got %q", vnis[0].ID)
	}

	if mock.CallCount("ListVNIs") != 1 {
		t.Errorf("expected ListVNIs call count 1, got %d", mock.CallCount("ListVNIs"))
	}

	// Call again and verify count increments
	_, _ = mock.ListVNIs(context.Background())
	if mock.CallCount("ListVNIs") != 2 {
		t.Errorf("expected ListVNIs call count 2, got %d", mock.CallCount("ListVNIs"))
	}
}

func TestClusterMode_Constants(t *testing.T) {
	if ModeROKS != "roks" {
		t.Errorf("expected ModeROKS == \"roks\", got %q", ModeROKS)
	}
	if ModeUnmanaged != "unmanaged" {
		t.Errorf("expected ModeUnmanaged == \"unmanaged\", got %q", ModeUnmanaged)
	}
}
