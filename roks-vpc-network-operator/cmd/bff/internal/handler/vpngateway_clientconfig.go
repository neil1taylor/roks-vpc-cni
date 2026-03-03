package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var clientNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// GenerateClientConfig handles POST /api/v1/vpn-gateways/:name/client-config
func (h *VPNGatewayHandler) GenerateClientConfig(w http.ResponseWriter, r *http.Request) {
	// Extract VPN gateway name from path: /api/v1/vpn-gateways/<name>/client-config
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/vpn-gateways/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 2 || parts[0] == "" {
		WriteError(w, http.StatusBadRequest, "missing vpn gateway name", "MISSING_NAME")
		return
	}
	vpnName := parts[0]

	// Auth check
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "secrets", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden: requires create secrets permission", "FORBIDDEN")
		return
	}

	// Parse request body
	var req model.ClientConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}
	if req.ClientName == "" || !clientNameRegex.MatchString(req.ClientName) {
		WriteError(w, http.StatusBadRequest, "clientName must be alphanumeric with optional hyphens", "INVALID_CLIENT_NAME")
		return
	}

	if h.dynClient == nil || h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "clients not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	// Fetch the VPCVPNGateway CR
	ns := r.URL.Query().Get("namespace")
	var vpnObj *unstructured.Unstructured
	if ns != "" {
		vpnObj, err = h.dynClient.Resource(vpcVPNGatewayGVR).Namespace(ns).Get(r.Context(), vpnName, metav1.GetOptions{})
	} else {
		// Cross-namespace search
		list, listErr := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if listErr != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCVPNGateways", "error", listErr)
			WriteError(w, http.StatusInternalServerError, "failed to find vpn gateway", "LIST_FAILED")
			return
		}
		for i := range list.Items {
			if list.Items[i].GetName() == vpnName {
				vpnObj = &list.Items[i]
				ns = vpnObj.GetNamespace()
				break
			}
		}
		if vpnObj == nil {
			err = fmt.Errorf("not found")
		}
	}
	if err != nil || vpnObj == nil {
		WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
		return
	}
	if ns == "" {
		ns = vpnObj.GetNamespace()
	}

	// Verify protocol is openvpn
	protocol, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "protocol")
	if protocol != "openvpn" {
		WriteError(w, http.StatusBadRequest, "client config generation is only supported for openvpn protocol", "WRONG_PROTOCOL")
		return
	}

	// Read CA cert from referenced secret
	caSecretName, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "ca", "name")
	caSecretKey, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "ca", "key")
	if caSecretName == "" || caSecretKey == "" {
		WriteError(w, http.StatusBadRequest, "VPN gateway missing CA secret reference", "MISSING_CA")
		return
	}

	caSecret, err := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), caSecretName, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to read CA secret", "secret", caSecretName, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to read CA secret", "CA_SECRET_READ_FAILED")
		return
	}
	caCertPEM := caSecret.Data[caSecretKey]
	if len(caCertPEM) == 0 {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("CA secret %s has no data at key %s", caSecretName, caSecretKey), "CA_CERT_MISSING")
		return
	}

	// Read CA private key: try spec.openVPN.caKey first, then fallback to "ca.key" in same secret
	caKeySecretName, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "caKey", "name")
	caKeySecretKey, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "caKey", "key")
	var caKeyPEM []byte
	if caKeySecretName != "" && caKeySecretKey != "" {
		caKeySecret, keyErr := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), caKeySecretName, metav1.GetOptions{})
		if keyErr != nil {
			slog.ErrorContext(r.Context(), "failed to read CA key secret", "secret", caKeySecretName, "error", keyErr)
			WriteError(w, http.StatusInternalServerError, "failed to read CA key secret", "CA_KEY_SECRET_READ_FAILED")
			return
		}
		caKeyPEM = caKeySecret.Data[caKeySecretKey]
	} else {
		// Fallback: look for "ca.key" in the same secret as the CA cert
		caKeyPEM = caSecret.Data["ca.key"]
	}
	if len(caKeyPEM) == 0 {
		WriteError(w, http.StatusBadRequest, "CA private key not found. Set spec.openVPN.caKey or include ca.key in the CA secret", "CA_KEY_MISSING")
		return
	}

	// Parse CA cert and key
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		WriteError(w, http.StatusInternalServerError, "failed to decode CA certificate PEM", "CA_CERT_DECODE_FAILED")
		return
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to parse CA certificate", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to parse CA certificate", "CA_CERT_PARSE_FAILED")
		return
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		WriteError(w, http.StatusInternalServerError, "failed to decode CA private key PEM", "CA_KEY_DECODE_FAILED")
		return
	}
	caKey, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		// Try PKCS1 as fallback
		caKey, err = x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
		if err != nil {
			// Try EC key
			caKey, err = x509.ParseECPrivateKey(caKeyBlock.Bytes)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to parse CA private key", "error", err)
				WriteError(w, http.StatusInternalServerError, "failed to parse CA private key", "CA_KEY_PARSE_FAILED")
				return
			}
		}
	}

	// Generate client key pair (RSA 2048)
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to generate client key", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to generate client key", "KEY_GEN_FAILED")
		return
	}

	// Create client certificate
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	clientCertTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: req.ClientName,
		},
		NotBefore: time.Now().Add(-10 * time.Minute),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientCertTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to sign client certificate", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to sign client certificate", "CERT_SIGN_FAILED")
		return
	}

	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	// Store client cert+key as K8s Secret
	secretName := fmt.Sprintf("%s-client-%s", vpnName, req.ClientName)
	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
			Labels: map[string]string{
				"vpc.roks.ibm.com/vpngateway":    vpnName,
				"vpc.roks.ibm.com/client-config":  req.ClientName,
			},
		},
		Data: map[string][]byte{
			"tls.crt": clientCertPEM,
			"tls.key": clientKeyPEM,
		},
		Type: corev1.SecretTypeTLS,
	}
	_, err = h.k8sClient.CoreV1().Secrets(ns).Create(r.Context(), clientSecret, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create client secret", "name", secretName, "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create client secret: %v", err), "SECRET_CREATE_FAILED")
		return
	}

	// Read tunnel endpoint and OpenVPN config from the VPN gateway
	tunnelEndpoint, _, _ := unstructured.NestedString(vpnObj.Object, "status", "tunnelEndpoint")
	listenPort, _, _ := unstructured.NestedInt64(vpnObj.Object, "spec", "openVPN", "listenPort")
	if listenPort == 0 {
		listenPort = 1194
	}
	proto, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "proto")
	if proto == "" {
		proto = "udp"
	}
	cipher, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "cipher")
	if cipher == "" {
		cipher = "AES-256-GCM"
	}

	// Build .ovpn config
	var sb strings.Builder
	sb.WriteString("client\n")
	sb.WriteString("dev tun\n")
	sb.WriteString(fmt.Sprintf("proto %s\n", proto))
	if tunnelEndpoint != "" {
		sb.WriteString(fmt.Sprintf("remote %s %d\n", tunnelEndpoint, listenPort))
	} else {
		sb.WriteString(fmt.Sprintf("remote REPLACE_WITH_ENDPOINT %d\n", listenPort))
	}
	sb.WriteString("resolv-retry infinite\n")
	sb.WriteString("nobind\n")
	sb.WriteString("persist-key\n")
	sb.WriteString("persist-tun\n")
	sb.WriteString("remote-cert-tls server\n")
	sb.WriteString(fmt.Sprintf("cipher %s\n", cipher))
	sb.WriteString("verb 3\n")
	sb.WriteString("\n")

	// Inline CA cert
	sb.WriteString("<ca>\n")
	sb.Write(caCertPEM)
	if !strings.HasSuffix(string(caCertPEM), "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("</ca>\n\n")

	// Inline client cert
	sb.WriteString("<cert>\n")
	sb.Write(clientCertPEM)
	if !strings.HasSuffix(string(clientCertPEM), "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("</cert>\n\n")

	// Inline client key
	sb.WriteString("<key>\n")
	sb.Write(clientKeyPEM)
	if !strings.HasSuffix(string(clientKeyPEM), "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("</key>\n")

	// Optionally include tls-auth
	tlsAuthSecretName, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "tlsAuth", "name")
	tlsAuthSecretKey, _, _ := unstructured.NestedString(vpnObj.Object, "spec", "openVPN", "tlsAuth", "key")
	if tlsAuthSecretName != "" && tlsAuthSecretKey != "" {
		taSecret, taErr := h.k8sClient.CoreV1().Secrets(ns).Get(r.Context(), tlsAuthSecretName, metav1.GetOptions{})
		if taErr == nil {
			taPEM := taSecret.Data[tlsAuthSecretKey]
			if len(taPEM) > 0 {
				sb.WriteString("\nkey-direction 1\n")
				sb.WriteString("<tls-auth>\n")
				sb.Write(taPEM)
				if !strings.HasSuffix(string(taPEM), "\n") {
					sb.WriteString("\n")
				}
				sb.WriteString("</tls-auth>\n")
			}
		}
	}

	resp := model.ClientConfigResponse{
		ClientName: req.ClientName,
		SecretName: secretName,
		OVPNConfig: sb.String(),
	}
	WriteJSON(w, http.StatusOK, resp)
}
