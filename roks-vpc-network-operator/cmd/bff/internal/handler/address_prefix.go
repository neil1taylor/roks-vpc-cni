package handler

import (
	"encoding/json"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// AddressPrefixHandler handles VPC address prefix operations.
type AddressPrefixHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewAddressPrefixHandler creates a new AddressPrefixHandler.
func NewAddressPrefixHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *AddressPrefixHandler {
	return &AddressPrefixHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListAddressPrefixes returns address prefixes for a VPC.
func (h *AddressPrefixHandler) ListAddressPrefixes(w http.ResponseWriter, r *http.Request) {
	vpcID := GetQueryParam(r, "vpc_id")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	prefixes, err := h.vpcClient.ListVPCAddressPrefixes(r.Context(), vpcID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error(), "VPC_API_ERROR")
		return
	}

	resp := make([]model.AddressPrefixResponse, len(prefixes))
	for i, p := range prefixes {
		resp[i] = model.AddressPrefixResponse{
			ID:        p.ID,
			Name:      p.Name,
			CIDR:      p.CIDR,
			Zone:      p.Zone,
			IsDefault: p.IsDefault,
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

// CreateAddressPrefix creates a new VPC address prefix.
func (h *AddressPrefixHandler) CreateAddressPrefix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VPCID string `json:"vpcId"`
		CIDR  string `json:"cidr"`
		Zone  string `json:"zone"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.VPCID == "" {
		req.VPCID = h.defaultVPCID
	}
	if req.VPCID == "" {
		WriteError(w, http.StatusBadRequest, "vpcId is required", "MISSING_VPC_ID")
		return
	}
	if req.CIDR == "" {
		WriteError(w, http.StatusBadRequest, "cidr is required", "MISSING_CIDR")
		return
	}
	if req.Zone == "" {
		WriteError(w, http.StatusBadRequest, "zone is required", "MISSING_ZONE")
		return
	}

	prefix, err := h.vpcClient.CreateVPCAddressPrefix(r.Context(), vpc.CreateAddressPrefixOptions{
		VPCID: req.VPCID,
		CIDR:  req.CIDR,
		Zone:  req.Zone,
		Name:  req.Name,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error(), "VPC_API_ERROR")
		return
	}

	WriteJSON(w, http.StatusCreated, model.AddressPrefixResponse{
		ID:        prefix.ID,
		Name:      prefix.Name,
		CIDR:      prefix.CIDR,
		Zone:      prefix.Zone,
		IsDefault: prefix.IsDefault,
	})
}
