package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const primaryUDNLabel = "k8s.ovn.org/primary-user-defined-network"

// NamespaceHandler handles namespace API operations.
type NamespaceHandler struct {
	k8sClient kubernetes.Interface
}

// NewNamespaceHandler creates a new namespace handler.
func NewNamespaceHandler(k8sClient kubernetes.Interface) *NamespaceHandler {
	return &NamespaceHandler{k8sClient: k8sClient}
}

// ListNamespaces handles GET /api/v1/namespaces
// Optional query param: ?label=<key> to filter by label presence.
func (h *NamespaceHandler) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	if h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "kubernetes client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	nsList, err := h.k8sClient.CoreV1().Namespaces().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list namespaces", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list namespaces", "LIST_FAILED")
		return
	}

	labelFilter := r.URL.Query().Get("label")
	result := make([]model.NamespaceInfo, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		_, hasPrimary := ns.Labels[primaryUDNLabel]
		if labelFilter != "" {
			if _, ok := ns.Labels[labelFilter]; !ok {
				continue
			}
		}
		result = append(result, model.NamespaceInfo{
			Name:            ns.Name,
			HasPrimaryLabel: hasPrimary,
		})
	}

	WriteJSON(w, http.StatusOK, result)
}

// CreateNamespace handles POST /api/v1/namespaces
func (h *NamespaceHandler) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	if h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "kubernetes client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	var req model.CreateNamespaceRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "namespace name is required", "MISSING_NAME")
		return
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   req.Name,
			Labels: req.Labels,
		},
	}

	created, err := h.k8sClient.CoreV1().Namespaces().Create(r.Context(), ns, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create namespace", "name", req.Name, "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create namespace: %v", err), "CREATE_FAILED")
		return
	}

	_, hasPrimary := created.Labels[primaryUDNLabel]
	WriteJSON(w, http.StatusCreated, model.NamespaceInfo{
		Name:            created.Name,
		HasPrimaryLabel: hasPrimary,
	})
}
