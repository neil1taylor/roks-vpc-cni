package handler

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListClients handles GET /api/v1/vpn-gateways/:name/clients
func (h *VPNGatewayHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	vpnName, ns, err := h.resolveVPNGateway(r)
	if err != nil {
		WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
		return
	}

	// Auth check
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "list", "secrets", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden: requires list secrets permission", "FORBIDDEN")
		return
	}

	if h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "k8s client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	// List client cert secrets
	labelSelector := fmt.Sprintf("vpc.roks.ibm.com/vpngateway=%s,vpc.roks.ibm.com/client-config", vpnName)
	secrets, err := h.k8sClient.CoreV1().Secrets(ns).List(r.Context(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list client secrets", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list client secrets", "LIST_FAILED")
		return
	}

	// Read CRL secret to determine revoked serials
	revokedSerials := map[string]bool{}
	crlSecretName := vpnName + "-crl"
	crlSecret, crlErr := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), crlSecretName, metav1.GetOptions{})
	if crlErr == nil {
		revokedCSV := crlSecret.Annotations["vpc.roks.ibm.com/revoked-serials"]
		if revokedCSV != "" {
			for _, s := range strings.Split(revokedCSV, ",") {
				revokedSerials[strings.TrimSpace(s)] = true
			}
		}
	}

	clients := make([]model.IssuedClientResponse, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		clientName := secret.Labels["vpc.roks.ibm.com/client-config"]
		certPEM := secret.Data["tls.crt"]
		if len(certPEM) == 0 {
			continue
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			continue
		}
		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			continue
		}

		serialHex := fmt.Sprintf("%x", cert.SerialNumber)
		clients = append(clients, model.IssuedClientResponse{
			ClientName: clientName,
			SecretName: secret.Name,
			SerialHex:  serialHex,
			IssuedAt:   cert.NotBefore.UTC().Format(time.RFC3339),
			ExpiresAt:  cert.NotAfter.UTC().Format(time.RFC3339),
			Revoked:    revokedSerials[serialHex],
		})
	}

	WriteJSON(w, http.StatusOK, clients)
}

// RevokeClient handles DELETE /api/v1/vpn-gateways/:name/clients/:clientName
func (h *VPNGatewayHandler) RevokeClient(w http.ResponseWriter, r *http.Request) {
	vpnName, ns, err := h.resolveVPNGateway(r)
	if err != nil {
		WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
		return
	}

	// Extract client name from path
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/vpn-gateways/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 3 || parts[2] == "" {
		WriteError(w, http.StatusBadRequest, "missing client name", "MISSING_CLIENT_NAME")
		return
	}
	clientName := parts[2]

	// Auth check
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "secrets", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden: requires delete secrets permission", "FORBIDDEN")
		return
	}

	if h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "k8s client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	// Find client cert secret
	clientSecretName := fmt.Sprintf("%s-client-%s", vpnName, clientName)
	clientSecret, err := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), clientSecretName, metav1.GetOptions{})
	if err != nil {
		WriteError(w, http.StatusNotFound, fmt.Sprintf("client secret %q not found", clientSecretName), "CLIENT_NOT_FOUND")
		return
	}

	// Parse client cert to extract serial number
	certPEM := clientSecret.Data["tls.crt"]
	if len(certPEM) == 0 {
		WriteError(w, http.StatusInternalServerError, "client secret has no certificate", "NO_CERT")
		return
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		WriteError(w, http.StatusInternalServerError, "failed to decode client certificate", "CERT_DECODE_FAILED")
		return
	}
	clientCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to parse client certificate", "CERT_PARSE_FAILED")
		return
	}

	// Load CA cert + key
	vpnObj, vpnErr := h.getVPNGatewayObj(r, vpnName, ns)
	if vpnErr != nil || vpnObj == nil {
		WriteError(w, http.StatusInternalServerError, "failed to load vpn gateway", "VPN_LOAD_FAILED")
		return
	}

	caCert, caKey, caErr := h.loadCACertAndKey(r, vpnObj, ns)
	if caErr != nil {
		WriteError(w, http.StatusInternalServerError, caErr.Error(), "CA_LOAD_FAILED")
		return
	}

	// Read existing CRL entries
	crlSecretName := vpnName + "-crl"
	crlSecret, crlErr := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), crlSecretName, metav1.GetOptions{})
	var existingRevoked []x509.RevocationListEntry
	revokedSerials := ""

	if crlErr == nil {
		// Parse existing CRL if present
		if crlPEM, ok := crlSecret.Data["crl.pem"]; ok && len(crlPEM) > 0 {
			crlBlock, _ := pem.Decode(crlPEM)
			if crlBlock != nil {
				existingCRL, parseErr := x509.ParseRevocationList(crlBlock.Bytes)
				if parseErr == nil {
					existingRevoked = existingCRL.RevokedCertificateEntries
				}
			}
		}
		revokedSerials = crlSecret.Annotations["vpc.roks.ibm.com/revoked-serials"]
	}

	// Add new serial to revoked list
	newEntry := x509.RevocationListEntry{
		SerialNumber:   clientCert.SerialNumber,
		RevocationTime: time.Now().UTC(),
	}
	existingRevoked = append(existingRevoked, newEntry)

	// Generate updated CRL
	crlBytes, err := generateUpdatedCRL(caCert, caKey, existingRevoked)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to generate CRL", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to generate CRL", "CRL_GEN_FAILED")
		return
	}
	crlPEM := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlBytes})

	// Update revoked serials annotation
	serialHex := fmt.Sprintf("%x", clientCert.SerialNumber)
	if revokedSerials != "" {
		revokedSerials += "," + serialHex
	} else {
		revokedSerials = serialHex
	}

	// Update or create CRL secret
	if crlErr == nil {
		if crlSecret.Data == nil {
			crlSecret.Data = map[string][]byte{}
		}
		crlSecret.Data["crl.pem"] = crlPEM
		if crlSecret.Annotations == nil {
			crlSecret.Annotations = map[string]string{}
		}
		crlSecret.Annotations["vpc.roks.ibm.com/revoked-serials"] = revokedSerials
		_, err = h.k8sClient.CoreV1().Secrets(ns).Update(r.Context(), crlSecret, metav1.UpdateOptions{})
	} else {
		// Fallback: create CRL secret if it doesn't exist
		newCRLSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crlSecretName,
				Namespace: ns,
				Labels: map[string]string{
					"vpc.roks.ibm.com/vpngateway": vpnName,
					"vpc.roks.ibm.com/crl":        "true",
				},
				Annotations: map[string]string{
					"vpc.roks.ibm.com/revoked-serials": revokedSerials,
				},
			},
			Data: map[string][]byte{
				"crl.pem": crlPEM,
			},
		}
		_, err = h.k8sClient.CoreV1().Secrets(ns).Create(r.Context(), newCRLSecret, metav1.CreateOptions{})
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update CRL secret", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update CRL secret", "CRL_UPDATE_FAILED")
		return
	}

	// Delete the client cert secret
	err = h.k8sClient.CoreV1().Secrets(ns).Delete(r.Context(), clientSecretName, metav1.DeleteOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete client secret", "name", clientSecretName, "error", err)
		// Non-fatal — CRL is already updated, client will be rejected
	}

	slog.InfoContext(r.Context(), "revoked client certificate",
		"vpnGateway", vpnName, "client", clientName, "serial", serialHex)

	w.WriteHeader(http.StatusNoContent)
}

// resolveVPNGateway extracts the VPN gateway name and namespace from the request path.
func (h *VPNGatewayHandler) resolveVPNGateway(r *http.Request) (string, string, error) {
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/vpn-gateways/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("missing vpn gateway name")
	}
	vpnName := parts[0]

	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		// Cross-namespace search to find the gateway's namespace
		list, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			return "", "", err
		}
		for i := range list.Items {
			if list.Items[i].GetName() == vpnName {
				ns = list.Items[i].GetNamespace()
				break
			}
		}
		if ns == "" {
			return "", "", fmt.Errorf("vpn gateway %q not found", vpnName)
		}
	}

	return vpnName, ns, nil
}

// getVPNGatewayObj fetches the VPCVPNGateway CR.
func (h *VPNGatewayHandler) getVPNGatewayObj(r *http.Request, vpnName, ns string) (*unstructured.Unstructured, error) {
	return h.dynClient.Resource(vpcVPNGatewayGVR).Namespace(ns).Get(r.Context(), vpnName, metav1.GetOptions{})
}

// loadCACertAndKey loads the CA certificate and private key from the VPN gateway's referenced secrets.
func (h *VPNGatewayHandler) loadCACertAndKey(r *http.Request, vpnObj *unstructured.Unstructured, ns string) (*x509.Certificate, crypto.Signer, error) {
	caSecretName, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "ca", "name")
	caSecretKey, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "ca", "key")
	if caSecretName == "" || caSecretKey == "" {
		return nil, nil, fmt.Errorf("VPN gateway missing CA secret reference")
	}

	caSecret, err := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), caSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA secret: %w", err)
	}
	caCertPEM := caSecret.Data[caSecretKey]
	if len(caCertPEM) == 0 {
		return nil, nil, fmt.Errorf("CA secret has no data at key %s", caSecretKey)
	}

	// Parse CA cert
	certBlock, _ := pem.Decode(caCertPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Read CA key
	caKeySecretName, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "caKey", "name")
	caKeySecretKey, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "caKey", "key")
	var caKeyPEM []byte
	if caKeySecretName != "" && caKeySecretKey != "" {
		caKeySecret, keyErr := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), caKeySecretName, metav1.GetOptions{})
		if keyErr != nil {
			return nil, nil, fmt.Errorf("failed to read CA key secret: %w", keyErr)
		}
		caKeyPEM = caKeySecret.Data[caKeySecretKey]
	} else {
		caKeyPEM = caSecret.Data["ca.key"]
	}
	if len(caKeyPEM) == 0 {
		return nil, nil, fmt.Errorf("CA private key not found")
	}

	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key PEM")
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			parsedKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse CA private key")
			}
		}
	}

	signer, ok := parsedKey.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("CA private key does not implement crypto.Signer")
	}

	return caCert, signer, nil
}

// generateUpdatedCRL creates a new CRL with the given revoked certificates.
func generateUpdatedCRL(caCert *x509.Certificate, caKey crypto.Signer, revokedCerts []x509.RevocationListEntry) ([]byte, error) {
	template := &x509.RevocationList{
		RevokedCertificateEntries: revokedCerts,
		Number:                    big.NewInt(time.Now().UnixNano()),
		ThisUpdate:                time.Now().UTC(),
		NextUpdate:                time.Now().UTC().Add(365 * 24 * time.Hour),
	}

	return x509.CreateRevocationList(rand.Reader, template, caCert, caKey)
}

// ensureCRLSecretOnGenerate is called after generating a client config to ensure the CRL secret exists.
// The operator normally creates it, but this is a fallback for older operator versions.
func (h *VPNGatewayHandler) ensureCRLSecretOnGenerate(ctx context.Context, vpnName, ns string) {
	crlSecretName := vpnName + "-crl"
	_, err := h.k8sClient.CoreV1().Secrets(ns).Get(ctx, crlSecretName, metav1.GetOptions{})
	if err == nil {
		return // already exists
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crlSecretName,
			Namespace: ns,
			Labels: map[string]string{
				"vpc.roks.ibm.com/vpngateway": vpnName,
				"vpc.roks.ibm.com/crl":        "true",
			},
		},
		Data: map[string][]byte{},
	}
	_, createErr := h.k8sClient.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if createErr != nil {
		slog.WarnContext(ctx, "failed to create CRL secret fallback", "secret", crlSecretName, "error", createErr)
	}
}
