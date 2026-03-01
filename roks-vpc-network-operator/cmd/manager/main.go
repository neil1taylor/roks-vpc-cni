package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	vpcv1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	cudnctrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/cudn"
	fipctrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/floatingip"
	gatewayctr "github.com/IBM/roks-vpc-network-operator/pkg/controller/gateway"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	nodectrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/node"
	routerctr "github.com/IBM/roks-vpc-network-operator/pkg/controller/router"
	udnctrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/udn"
	vlactrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/vlanattachment"
	vmctrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/vm"
	vnictrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/vni"
	subnetctrl "github.com/IBM/roks-vpc-network-operator/pkg/controller/vpcsubnet"
	"github.com/IBM/roks-vpc-network-operator/pkg/gc"
	"github.com/IBM/roks-vpc-network-operator/pkg/roks"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
	vmwebhook "github.com/IBM/roks-vpc-network-operator/pkg/webhook"

	// Import metrics package to register Prometheus metrics in init()
	_ "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Register VPC CRD types
	utilruntime.Must(vpcv1alpha1.AddToScheme(scheme))

	// TODO: Register OVN-Kubernetes types:
	// utilruntime.Must(ovnv1.AddToScheme(scheme))

	// TODO: Register KubeVirt types:
	// utilruntime.Must(kubevirtv1.AddToScheme(scheme))
}

func main() {
	opts := zap.Options{Development: true}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := ctrl.Log.WithName("setup")

	// ── Read configuration ──

	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	region := os.Getenv("VPC_REGION")
	clusterID := os.Getenv("CLUSTER_ID")
	resourceGroupID := os.Getenv("RESOURCE_GROUP_ID")
	vpcID := os.Getenv("VPC_ID")

	clusterMode := roks.ClusterMode(os.Getenv("CLUSTER_MODE")) // "roks" or "unmanaged"
	if clusterMode == "" {
		clusterMode = roks.ModeUnmanaged
	}

	// ── NNCP configuration ──

	nncpConfig := network.NNCPConfig{
		Enabled:      os.Getenv("NNCP_ENABLED") != "false", // enabled by default
		BridgeName:   os.Getenv("NNCP_BRIDGE_NAME"),
		SecondaryNIC: os.Getenv("NNCP_SECONDARY_NIC"),
	}
	if nncpConfig.BridgeName == "" {
		nncpConfig.BridgeName = "br-localnet"
	}
	if nncpConfig.SecondaryNIC == "" {
		nncpConfig.SecondaryNIC = "bond1"
	}
	if selectorStr := os.Getenv("NNCP_NODE_SELECTOR"); selectorStr != "" {
		nncpConfig.NodeSelector = parseNodeSelector(selectorStr)
	} else {
		nncpConfig.NodeSelector = map[string]string{"node-role.kubernetes.io/worker": ""}
	}

	if apiKey == "" || region == "" || clusterID == "" || vpcID == "" {
		logger.Error(nil, "Missing required environment variables",
			"required", "IBMCLOUD_API_KEY, VPC_REGION, CLUSTER_ID, VPC_ID")
		os.Exit(1)
	}

	// ── Create VPC client ──

	rawVPCClient, err := vpc.NewClient(vpc.ClientConfig{
		APIKey:          apiKey,
		Region:          region,
		ResourceGroupID: resourceGroupID,
		ClusterID:       clusterID,
		MaxConcurrent:   10,
	})
	if err != nil {
		logger.Error(err, "Failed to create VPC client")
		os.Exit(1)
	}
	vpcClient := vpc.NewInstrumentedClient(rawVPCClient)

	// ── Create ROKS client (for ROKS-managed VNI/VLAN resources) ──

	var roksClient roks.ROKSClient
	if clusterMode == roks.ModeROKS {
		roksClient, err = roks.NewClient(roks.ROKSClientConfig{
			ClusterID: clusterID,
			Region:    region,
		})
		if err != nil {
			logger.Error(err, "Failed to create ROKS client")
			os.Exit(1)
		}
		logger.Info("Running in ROKS mode — VNI/VLAN managed by ROKS platform")
	} else {
		logger.Info("Running in unmanaged mode — VNI/VLAN managed via VPC API")
	}

	// ── Create controller manager ──

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: ":8081",
		LeaderElection:         true,
		LeaderElectionID:       "roks-vpc-network-operator.ibm.com",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		logger.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	// ── Register existing controllers ──

	if err := (&cudnctrl.Reconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		VPC:        vpcClient,
		ClusterID:  clusterID,
		NNCPConfig: nncpConfig,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create CUDN controller")
		os.Exit(1)
	}

	if err := (&nodectrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ClusterID: clusterID,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create Node controller")
		os.Exit(1)
	}

	if err := (&udnctrl.Reconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		VPC:        vpcClient,
		ClusterID:  clusterID,
		NNCPConfig: nncpConfig,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create UDN controller")
		os.Exit(1)
	}

	if err := (&vmctrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ClusterID: clusterID,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VM controller")
		os.Exit(1)
	}

	// ── Register new CRD controllers ──

	if err := (&subnetctrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ClusterID: clusterID,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VPCSubnet controller")
		os.Exit(1)
	}

	if err := (&vnictrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ROKS:      roksClient, // nil on unmanaged clusters
		ClusterID: clusterID,
		Mode:      clusterMode,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VNI controller")
		os.Exit(1)
	}

	if err := (&vlactrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ROKS:      roksClient, // nil on unmanaged clusters
		ClusterID: clusterID,
		Mode:      clusterMode,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VLANAttachment controller")
		os.Exit(1)
	}

	if err := (&fipctrl.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ClusterID: clusterID,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create FloatingIP controller")
		os.Exit(1)
	}

	if err := (&gatewayctr.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		VPC:       vpcClient,
		ClusterID: clusterID,
		VPCID:     vpcID,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VPCGateway controller")
		os.Exit(1)
	}

	if err := (&routerctr.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create VPCRouter controller")
		os.Exit(1)
	}

	// ── Register webhook ──

	mgr.GetWebhookServer().Register("/mutate-virtualmachine", &webhook.Admission{
		Handler: &vmwebhook.VMMutatingWebhook{
			VPC:       vpcClient,
			K8s:       mgr.GetClient(),
			ClusterID: clusterID,
		},
	})

	// ── Health checks ──

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	// ── Start orphan GC ──

	orphanGC := &gc.OrphanCollector{
		K8sClient: mgr.GetClient(),
		VPC:       vpcClient,
		ClusterID: clusterID,
	}
	if err := mgr.Add(orphanGC); err != nil {
		logger.Error(err, "Unable to add orphan GC")
		os.Exit(1)
	}

	// ── Start manager ──

	logger.Info("Starting manager", "clusterID", clusterID, "region", region, "vpcID", vpcID)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "Manager exited with error")
		os.Exit(1)
	}
}

// parseNodeSelector parses "key1=val1,key2=val2" into a map.
func parseNodeSelector(s string) map[string]string {
	result := map[string]string{}
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			result[parts[0]] = ""
		}
	}
	return result
}

// Suppress unused import warnings during scaffolding
var _ = context.Background
var _ = fmt.Sprintf
