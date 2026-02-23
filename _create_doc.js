const fs = require("fs");
const { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, Header, Footer, AlignmentType, LevelFormat, HeadingLevel, BorderStyle, WidthType, ShadingType, PageNumber, PageBreak } = require("docx");

const CW = 9360;
const BLUE = "1F4E79", DG = "333333", GR = "666666", LG = "F2F2F2", WH = "FFFFFF";
const bd = { style: BorderStyle.SINGLE, size: 1, color: "CCCCCC" };
const bds = { top: bd, bottom: bd, left: bd, right: bd };
const nb = { style: BorderStyle.NONE, size: 0, color: WH };
const nbs = { top: nb, bottom: nb, left: nb, right: nb };
const cm = { top: 80, bottom: 80, left: 120, right: 120 };

const h1 = t => new Paragraph({ heading: HeadingLevel.HEADING_1, children: [new TextRun(t)] });
const h2 = t => new Paragraph({ heading: HeadingLevel.HEADING_2, children: [new TextRun(t)] });
const h3 = t => new Paragraph({ heading: HeadingLevel.HEADING_3, children: [new TextRun(t)] });
const p = t => new Paragraph({ spacing: { after: 120 }, children: [new TextRun({ font: "Arial", size: 22, color: DG, text: t })] });
const B = t => new TextRun({ font: "Arial", size: 22, bold: true, color: DG, text: t });
const N = t => new TextRun({ font: "Arial", size: 22, color: DG, text: t });
const rp = r => new Paragraph({ spacing: { after: 120 }, children: r });
const bul = (t, r="b1") => new Paragraph({ numbering: { reference: r, level: 0 }, children: [N(t)] });
const num = (t, r="n1") => new Paragraph({ numbering: { reference: r, level: 0 }, children: [N(t)] });
const sp = (s=200) => new Paragraph({ spacing: { after: s }, children: [] });
const cd = lines => lines.map(l => new Paragraph({ spacing: { after: 30 }, children: [new TextRun({ font: "Courier New", size: 17, color: DG, text: l })] }));
const PB = () => new Paragraph({ children: [new PageBreak()] });
const hC = (t,w) => new TableCell({ borders: bds, width: { size: w, type: WidthType.DXA }, shading: { fill: BLUE, type: ShadingType.CLEAR }, margins: cm, children: [new Paragraph({ children: [new TextRun({ font: "Arial", size: 20, bold: true, color: WH, text: t })] })] });
const dC = (t,w,s) => new TableCell({ borders: bds, width: { size: w, type: WidthType.DXA }, shading: s ? { fill: s, type: ShadingType.CLEAR } : undefined, margins: cm, children: [new Paragraph({ children: [new TextRun({ font: "Arial", size: 20, color: DG, text: t })] })] });

const nc = [];
for (let i=1;i<=15;i++) nc.push({ reference:"n"+i, levels:[{level:0,format:LevelFormat.DECIMAL,text:"%1.",alignment:AlignmentType.LEFT,style:{paragraph:{indent:{left:720,hanging:360}}}}]});
nc.push({reference:"b1",levels:[{level:0,format:LevelFormat.BULLET,text:"\u2022",alignment:AlignmentType.LEFT,style:{paragraph:{indent:{left:720,hanging:360}}}}]});
nc.push({reference:"b2",levels:[{level:0,format:LevelFormat.BULLET,text:"\u2022",alignment:AlignmentType.LEFT,style:{paragraph:{indent:{left:720,hanging:360}}}}]});

const pp = { page: { size: { width: 12240, height: 15840 }, margin: { top: 1440, right: 1440, bottom: 1440, left: 1440 } } };

const doc = new Document({
  styles: { default: { document: { run: { font: "Arial", size: 22 } } }, paragraphStyles: [
    { id:"Heading1",name:"Heading 1",basedOn:"Normal",next:"Normal",quickFormat:true,run:{size:36,bold:true,font:"Arial",color:BLUE},paragraph:{spacing:{before:360,after:200},outlineLevel:0}},
    { id:"Heading2",name:"Heading 2",basedOn:"Normal",next:"Normal",quickFormat:true,run:{size:28,bold:true,font:"Arial",color:BLUE},paragraph:{spacing:{before:240,after:160},outlineLevel:1}},
    { id:"Heading3",name:"Heading 3",basedOn:"Normal",next:"Normal",quickFormat:true,run:{size:24,bold:true,font:"Arial",color:DG},paragraph:{spacing:{before:200,after:120},outlineLevel:2}},
  ]},
  numbering: { config: nc },
  sections: [
    // COVER
    { properties: pp, children: [
      sp(2400),
      new Paragraph({alignment:AlignmentType.CENTER,spacing:{after:200},children:[new TextRun({font:"Arial",size:56,bold:true,color:BLUE,text:"Design Document"})]}),
      sp(120),
      new Paragraph({alignment:AlignmentType.CENTER,spacing:{after:200},children:[new TextRun({font:"Arial",size:32,color:DG,text:"ROKS VPC Network Operator"})]}),
      new Paragraph({alignment:AlignmentType.CENTER,spacing:{after:100},children:[new TextRun({font:"Arial",size:24,color:GR,text:"Automated VPC Resource Lifecycle for OpenShift Virtualization on IBM Cloud"})]}),
      sp(600),
      new Table({width:{size:5400,type:WidthType.DXA},columnWidths:[2000,3400],alignment:AlignmentType.CENTER,
        rows:[["Version","1.0 \u2014 Draft"],["Date","February 2026"],["Author","Neil Taylor"],["Status","For Review"]].map(([k,v])=>new TableRow({children:[
          new TableCell({borders:nbs,width:{size:2000,type:WidthType.DXA},margins:cm,children:[new Paragraph({children:[new TextRun({font:"Arial",size:20,bold:true,color:GR,text:k})]})]}),
          new TableCell({borders:nbs,width:{size:3400,type:WidthType.DXA},margins:cm,children:[new Paragraph({children:[new TextRun({font:"Arial",size:20,color:DG,text:v})]})]}),
        ]}))}),
    ]},
    // MAIN
    { properties: pp,
      headers:{default:new Header({children:[new Paragraph({alignment:AlignmentType.RIGHT,children:[new TextRun({font:"Arial",size:18,color:GR,text:"ROKS VPC Network Operator \u2014 Design Document"})]})]})},
      footers:{default:new Footer({children:[new Paragraph({alignment:AlignmentType.CENTER,children:[new TextRun({font:"Arial",size:18,color:GR,text:"Page "}),new TextRun({font:"Arial",size:18,color:GR,children:[PageNumber.CURRENT]})]})]})} ,
      children: [
        h1("1. Executive Summary"),
        h2("1.1 Problem Statement"),
        p("IBM Cloud ROKS now supports OVNKubernetes and OpenShift 4.20, with upcoming support for OVN LocalNets. This enables KubeVirt virtual machines running on bare metal workers to be placed directly on VPC subnets, giving each VM a first-class VPC network identity with its own MAC address, reserved IP, security groups, and optional floating IP."),
        p("However, achieving this today requires manual coordination across two separate interfaces: the Kubernetes/OpenShift API (for CUDNs and VirtualMachine CRs) and the IBM Cloud VPC API (for subnets, Virtual Network Interfaces, VLAN attachments, reserved IPs, floating IPs, security groups, and ACLs). This is error-prone, slow, and does not scale."),
        h2("1.2 Proposed Solution"),
        p("A standalone Kubernetes operator that watches ClusterUserDefinedNetwork and VirtualMachine custom resources and automatically provisions, reconciles, and garbage-collects the required IBM Cloud VPC resources. A mutating admission webhook transparently injects VPC-allocated MAC addresses into VM specs at creation time."),
        h2("1.3 Key Design Decisions"),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[3120,6240],rows:[
          new TableRow({children:[hC("Decision",3120),hC("Rationale",6240)]}),
          new TableRow({children:[dC("Kubernetes Operator (not CNI)",3120,LG),dC("VPC API calls are slow/async. CNI is synchronous and would block pod startup. Operator uses reconciliation with retry.",6240,LG)]}),
          new TableRow({children:[dC("Mutating Webhook for MAC injection",3120),dC("Transparent to admin. No race conditions. VM born complete. No wrapper CRDs.",6240)]}),
          new TableRow({children:[dC("Three reconciliation loops",3120,LG),dC("CUDN (subnet + VLAN attachments), Node (VLAN on join/remove), VM (VNI + IP + FIP lifecycle).",6240,LG)]}),
          new TableRow({children:[dC("Annotations on CUDN",3120),dC("Keeps upstream CRD untouched. No extra objects. Can graduate to CRD later.",6240)]}),
          new TableRow({children:[dC("Full lifecycle with finalizers",3120,LG),dC("Creates and GCs all VPC resources. Finalizers prevent orphans. Retry handles failures.",6240,LG)]}),
          new TableRow({children:[dC("1:1 CUDN-to-Subnet",3120),dC("One CUDN = one VPC subnet in one zone. Multi-zone uses multiple CUDNs.",6240)]}),
          new TableRow({children:[dC("Admin-managed SGs/ACLs",3120,LG),dC("Admin pre-creates and references via annotations. Operator attaches, not manages.",6240,LG)]}),
          new TableRow({children:[dC("Service ID API key auth",3120),dC("Same as CSI driver. Stored as K8s Secret with minimum IAM roles.",6240)]}),
        ]}),
        PB(),
        h1("2. Architecture Overview"),
        h2("2.1 Infrastructure Context"),
        p("The operator runs in a ROKS cluster where workers are bare metal servers. Each has a primary PCI network interface (uplink). VLAN interfaces attach dynamically to PCI interfaces without server restart. OVN-Kubernetes LocalNet bridges the cluster to VPC subnets via VLAN-tagged traffic."),
        h2("2.2 Data Path"),
        p("VM-to-VPC data path:"),
        num("KubeVirt VM sends traffic with VPC-assigned MAC on its localnet interface.","n1"),
        num("OVN br-int routes via localnet port to physical bridge through patch ports.","n1"),
        num("Physical OVS bridge VLAN-tags the frame with the CUDN VLAN ID.","n1"),
        num("Bare metal PCI uplink forwards VLAN-tagged frame to VPC fabric.","n1"),
        num("VPC fabric matches MAC to floating VNI, applies SG rules, routes packet.","n1"),
        sp(80),
        p("Return path: VPC routes to VNI via reserved IP, VLAN attachment delivers to bare metal, OVN delivers to VM."),
        h2("2.3 Components"),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[2340,2340,4680],rows:[
          new TableRow({children:[hC("Component",2340),hC("Type",2340),hC("Responsibility",4680)]}),
          new TableRow({children:[dC("CUDN Reconciler",2340,LG),dC("Controller",2340,LG),dC("Watches LocalNet CUDNs. Creates VPC subnets + VLAN attachments on all BM nodes.",4680,LG)]}),
          new TableRow({children:[dC("Node Reconciler",2340),dC("Controller",2340),dC("Watches Nodes. Ensures new BM nodes get VLAN attachments for all CUDNs.",4680)]}),
          new TableRow({children:[dC("VM Reconciler",2340,LG),dC("Controller",2340,LG),dC("Watches VMs. Manages VNI lifecycle, reserved IPs, FIPs.",4680,LG)]}),
          new TableRow({children:[dC("VM Webhook",2340),dC("Mutating Webhook",2340),dC("Intercepts VM CREATE. Creates VNI, injects MAC+IP.",4680)]}),
          new TableRow({children:[dC("VPC Client",2340,LG),dC("Library",2340,LG),dC("VPC API wrapper with retry, rate limiting, idempotency.",4680,LG)]}),
          new TableRow({children:[dC("Orphan GC",2340),dC("Periodic",2340),dC("Finds VPC resources with no K8s object. Deletes orphans.",4680)]}),
        ]}),
        PB(),
        h1("3. Bare Metal Networking Model"),
        h2("3.1 PCI and VLAN Interfaces"),
        p("PCI interfaces are created at provisioning and cannot change while running. VLAN interfaces are dynamic. The operator creates one VLAN attachment per CUDN per node:"),
        bul("floatable: true \u2014 Enables VM live migration between bare metal hosts.","b1"),
        bul("VLAN ID from CUDN annotation \u2014 Matches OVN LocalNet tagging. VPC routes to correct interface.","b1"),
        bul("Associated with CUDN VPC subnet \u2014 Traffic enters the correct subnet.","b1"),
        h2("3.2 Floating VNIs"),
        p("Each VM gets a dedicated floating VNI \u2014 the VM identity in the VPC (MAC, IP, SGs, FIP):"),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[2800,6560],rows:[
          new TableRow({children:[hC("Setting",2800),hC("Purpose",6560)]}),
          new TableRow({children:[dC("auto_delete: false",2800,LG),dC("VNI persists across host reprovisioning and live migration.",6560,LG)]}),
          new TableRow({children:[dC("allow_ip_spoofing: true",2800),dC("VM MAC differs from VLAN interface MAC. Spoofing must be allowed.",6560)]}),
          new TableRow({children:[dC("enable_infrastructure_nat: false",2800,LG),dC("VM needs its own routable IP, not hidden behind host NAT.",6560,LG)]}),
        ]}),
        h2("3.3 MAC as Identity Anchor"),
        p("VPC auto-generates a MAC on VNI creation. The operator injects this MAC into the VM spec via webhook. The VM boots with this MAC on its localnet interface. VPC matches traffic by MAC to the VNI, applying SGs and routing via the reserved IP."),
        p("The reserved IP is injected via cloud-init network config, or served via DHCP from the MAC-to-IP binding."),
        PB(),
        h1("4. CUDN Annotation Schema"),
        h2("4.1 Required Annotations"),
        ...cd(["apiVersion: k8s.ovn.org/v1","kind: ClusterUserDefinedNetwork","metadata:","  name: vm-network-1","  annotations:","    vpc.roks.ibm.com/zone: \"us-south-1\"","    vpc.roks.ibm.com/cidr: \"10.240.64.0/24\"","    vpc.roks.ibm.com/vpc-id: \"r006-xxxx\"","    vpc.roks.ibm.com/vlan-id: \"100\"","    vpc.roks.ibm.com/security-group-ids: \"r006-sg1,r006-sg2\"","    vpc.roks.ibm.com/acl-id: \"r006-acl1\"","spec:","  topology: LocalNet"]),
        sp(80),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[3200,1200,4960],rows:[
          new TableRow({children:[hC("Annotation",3200),hC("Req",1200),hC("Description",4960)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/zone",3200,LG),dC("Yes",1200,LG),dC("VPC zone. Validated against cluster.",4960,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/cidr",3200),dC("Yes",1200),dC("Subnet CIDR. Validated for non-overlap.",4960)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/vpc-id",3200,LG),dC("Yes",1200,LG),dC("VPC ID for subnet creation.",4960,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/vlan-id",3200),dC("Yes",1200),dC("VLAN ID. Must be unique per CUDN.",4960)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/security-group-ids",3200,LG),dC("Yes",1200,LG),dC("Comma-separated SG IDs.",4960,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/acl-id",3200),dC("Yes",1200),dC("Network ACL for the subnet.",4960)]}),
        ]}),
        h2("4.2 Status Annotations (Operator-written)"),
        ...cd(["vpc.roks.ibm.com/subnet-id: \"0717-xxxx\"","vpc.roks.ibm.com/subnet-status: \"active\"","vpc.roks.ibm.com/vlan-attachments: \"node1:att1,node2:att2\""]),
        PB(),
        h1("5. VM Annotations"),
        h2("5.1 Admin (Optional)"),
        ...cd(["vpc.roks.ibm.com/fip: \"true\"  # Request floating IP"]),
        h2("5.2 Operator-managed"),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[3600,5760],rows:[
          new TableRow({children:[hC("Annotation",3600),hC("Description",5760)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/vni-id",3600,LG),dC("VNI ID bound to this VM.",5760,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/mac-address",3600),dC("VPC-generated MAC (also in interface spec).",5760)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/reserved-ip",3600,LG),dC("Private IP on VPC subnet.",5760,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/reserved-ip-id",3600),dC("Reserved IP resource ID.",5760)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/fip-id",3600,LG),dC("Floating IP ID (if requested).",5760,LG)]}),
          new TableRow({children:[dC("vpc.roks.ibm.com/fip-address",3600),dC("Public FIP address (if requested).",5760)]}),
        ]}),
        PB(),
        h1("6. Reconciler Specifications"),
        h2("6.1 CUDN Reconciler"),
        rp([B("Watches: "),N("ClusterUserDefinedNetwork with topology: LocalNet")]),
        h3("Creation Flow"),
        num("Validate annotations.","n2"),num("Validate zone matches cluster.","n2"),num("Validate CIDR non-overlap.","n2"),num("Validate VLAN ID uniqueness.","n2"),num("Validate SGs and ACL exist.","n2"),num("Add finalizer vpc.roks.ibm.com/cudn-cleanup.","n2"),num("Create VPC subnet (POST /subnets).","n2"),num("Tag subnet with cluster ID + CUDN name.","n2"),num("Create VLAN attachment on each BM node (floatable, VLAN ID, subnet).","n2"),num("Write status annotations.","n2"),
        h3("Deletion Flow"),
        num("Block if VMs reference this CUDN.","n3"),num("Delete VLAN attachments on all nodes.","n3"),num("Delete VPC subnet.","n3"),num("Remove finalizer.","n3"),
        h2("6.2 Node Reconciler"),
        rp([B("Watches: "),N("Node objects (bare metal)")]),
        h3("Node Join"),
        num("List all LocalNet CUDNs.","n4"),num("Create VLAN attachment on new node for each.","n4"),num("Update CUDN status.","n4"),
        h3("Node Removal"),
        p("Delete VLAN attachments. Update CUDN status."),
        h2("6.3 VM Reconciler"),
        rp([B("Watches: "),N("VMs with operator annotations")]),
        h3("Drift Detection"),
        p("Periodic verification that VPC resources exist. Emits warnings if deleted out-of-band."),
        h3("Deletion Flow"),
        num("Finalizer fires.","n5"),num("Delete FIP if present.","n5"),num("Delete VNI (auto-deletes reserved IP).","n5"),num("Remove finalizer.","n5"),
        PB(),
        h1("7. Mutating Admission Webhook"),
        h2("7.1 Flow"),
        num("Intercept VM CREATE.","n6"),num("Find LocalNet CUDN references in VM spec.","n6"),num("Pass-through if none.","n6"),num("Look up CUDN for subnet ID.","n6"),num("Create floating VNI (auto_delete:false, ip_spoofing:true, infra_nat:false, SGs). Tag with cluster/ns/vm.","n6"),num("Read MAC + reserved IP from response.","n6"),num("Create FIP if annotated.","n6"),num("Inject macAddress into interface spec + IP into cloud-init.","n6"),num("Set annotations + finalizer.","n6"),num("Return mutated response.","n6"),
        h2("7.2 Error Handling"),
        bul("VPC failure: reject admission, admin retries.","b2"),
        bul("Idempotency: VM ns/name as VNI tag; reuse if exists.","b2"),
        bul("Orphans: GC job cleans VNIs with no matching VM.","b2"),
        h2("7.3 Timeout"),
        p("VNI creation: 1\u20133s. Webhook: 15s. K8s API server: 30s."),
        PB(),
        h1("8. VPC API Reference"),
        h2("8.1 Create Subnet"),
        ...cd(["POST /v1/subnets","{","  \"name\": \"roks-{cluster}-{cudn}\",","  \"vpc\": { \"id\": \"{vpc-id}\" },","  \"zone\": { \"name\": \"{zone}\" },","  \"ipv4_cidr_block\": \"{cidr}\",","  \"network_acl\": { \"id\": \"{acl}\" }","}"]),
        h2("8.2 Create VLAN Attachment"),
        ...cd(["POST /v1/bare_metal_servers/{bm}/network_attachments","{","  \"interface_type\": \"vlan\",","  \"vlan\": {vlan-id},","  \"virtual_network_interface\": {","    \"subnet\": { \"id\": \"{subnet}\" },","    \"allow_ip_spoofing\": true,","    \"enable_infrastructure_nat\": false","  },","  \"allow_to_float\": true","}"]),
        h2("8.3 Create Floating VNI"),
        ...cd(["POST /v1/virtual_network_interfaces","{","  \"name\": \"roks-{cluster}-{ns}-{vm}\",","  \"subnet\": { \"id\": \"{subnet}\" },","  \"primary_ip\": { \"auto_delete\": true },","  \"allow_ip_spoofing\": true,","  \"enable_infrastructure_nat\": false,","  \"auto_delete\": false,","  \"security_groups\": [{ \"id\": \"{sg}\" }]","}"]),
        rp([B("Response: "),N("id, mac_address, primary_ip.address, primary_ip.id")]),
        h2("8.4 Create Floating IP"),
        ...cd(["POST /v1/floating_ips","{","  \"name\": \"roks-{cluster}-{ns}-{vm}-fip\",","  \"zone\": { \"name\": \"{zone}\" },","  \"target\": { \"id\": \"{vni}\" }","}"]),
        h2("8.5 Delete"),
        ...cd(["DELETE /v1/floating_ips/{fip}","DELETE /v1/virtual_network_interfaces/{vni}","DELETE /v1/bare_metal_servers/{bm}/network_attachments/{att}","DELETE /v1/subnets/{subnet}"]),
        PB(),
        h1("9. Administrator Workflow"),
        h2("9.1 Setup"),
        num("Install operator (Helm/OLM).","n7"),num("Configure VPC API key secret.","n7"),num("Pre-create SGs and ACLs.","n7"),
        h2("9.2 Create Network"),
        p("Apply CUDN with annotations. Operator provisions subnet + VLAN attachments automatically."),
        h2("9.3 Deploy VM"),
        ...cd(["apiVersion: kubevirt.io/v1","kind: VirtualMachine","metadata:","  name: my-vm","  annotations:","    vpc.roks.ibm.com/fip: \"true\"","spec:","  running: true","  template:","    spec:","      networks:","      - name: vpc-net","        multus:","          networkName: vm-network-1","      domain:","        devices:","          interfaces:","          - name: vpc-net","            bridge: {}","      volumes:","      - name: cloudinit","        cloudInitNoCloud:","          userData: |","            #cloud-config","            password: changeme"]),
        sp(60),
        p("Webhook injects MAC + cloud-init IP transparently."),
        h2("9.4 Cleanup"),
        p("kubectl delete vm \u2192 VPC resources cleaned via finalizer. kubectl delete cudn \u2192 blocked if VMs exist, then deletes subnet + VLAN attachments."),
        PB(),
        h1("10. Code Structure"),
        ...cd(["roks-vpc-network-operator/","\u251C cmd/manager/main.go","\u251C pkg/","\u2502 \u251C controller/{cudn,node,vm}/reconciler.go","\u2502 \u251C webhook/vm_mutating.go","\u2502 \u251C vpc/{client,subnet,vni,vlan_attachment,floating_ip,ratelimiter}.go","\u2502 \u251C annotations/keys.go","\u2502 \u251C finalizers/finalizers.go","\u2502 \u2514 gc/orphan_collector.go","\u251C config/{webhook,rbac,samples}/","\u251C deploy/helm/","\u251C Dockerfile, Makefile, go.mod"]),
        sp(80),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[3120,2200,4040],rows:[
          new TableRow({children:[hC("Dependency",3120),hC("Version",2200),hC("Purpose",4040)]}),
          new TableRow({children:[dC("controller-runtime",3120,LG),dC(">= 0.18",2200,LG),dC("Controller framework + webhook server",4040,LG)]}),
          new TableRow({children:[dC("IBM VPC Go SDK",3120),dC("latest",2200),dC("github.com/IBM/vpc-go-sdk",4040)]}),
          new TableRow({children:[dC("KubeVirt client-go",3120,LG),dC(">= 1.3",2200,LG),dC("VirtualMachine types",4040,LG)]}),
          new TableRow({children:[dC("OVN-K8s types",3120),dC("4.20+",2200),dC("CUDN types",4040)]}),
        ]}),
        sp(80),
        new Table({width:{size:CW,type:WidthType.DXA},columnWidths:[3120,3120,3120],rows:[
          new TableRow({children:[hC("Resource",3120),hC("API Group",3120),hC("Verbs",3120)]}),
          new TableRow({children:[dC("clusteruserdefinednetworks",3120,LG),dC("k8s.ovn.org",3120,LG),dC("get,list,watch,update,patch",3120,LG)]}),
          new TableRow({children:[dC("virtualmachines",3120),dC("kubevirt.io",3120),dC("get,list,watch,update,patch",3120)]}),
          new TableRow({children:[dC("nodes",3120,LG),dC("core",3120,LG),dC("get,list,watch",3120,LG)]}),
          new TableRow({children:[dC("secrets",3120),dC("core",3120),dC("get",3120)]}),
          new TableRow({children:[dC("events",3120,LG),dC("core",3120,LG),dC("create,patch",3120,LG)]}),
        ]}),
        PB(),
        h1("11. Operational Concerns"),
        h2("11.1 Rate Limiting"),
        p("Token bucket on VPC client. Webhook semaphore caps at 10 concurrent calls. Controller-runtime provides queue rate limiting."),
        h2("11.2 Drift Detection"),
        p("Every 5 minutes, verify VPC resources exist. Report drift as events, not auto-correct."),
        h2("11.3 Orphan GC"),
        p("Every 10 minutes. Grace period 15 minutes. Deletes tagged VPC resources with no K8s object."),
        h2("11.4 Scaling"),
        p("N nodes \u00D7 M CUDNs = VLAN attachments + 1 VNI/VM. Batched with rate limiting."),
        h2("11.5 Live Migration"),
        p("floatable VLAN attachments + non-auto-delete VNIs = transparent KubeVirt live migration."),
        h2("11.6 Observability"),
        bul("K8s events on CUDN/VM objects."),
        bul("Prometheus: vpc_api_calls_total, vpc_api_errors_total, vpc_api_latency_seconds."),
        bul("Structured logging with VPC resource IDs."),
        PB(),
        h1("12. Sample Manifests"),
        h2("12.1 Webhook Config"),
        ...cd(["apiVersion: admissionregistration.k8s.io/v1","kind: MutatingWebhookConfiguration","metadata:","  name: roks-vpc-network-operator-webhook","webhooks:","- name: vm-vpc-inject.roks.ibm.com","  admissionReviewVersions: [\"v1\"]","  sideEffects: None","  timeoutSeconds: 30","  clientConfig:","    service:","      name: roks-vpc-network-operator-webhook","      namespace: roks-vpc-network-operator","      path: /mutate-virtualmachine","  rules:","  - apiGroups: [\"kubevirt.io\"]","    apiVersions: [\"v1\"]","    resources: [\"virtualmachines\"]","    operations: [\"CREATE\"]","  failurePolicy: Fail"]),
        h2("12.2 Example CUDN"),
        ...cd(["apiVersion: k8s.ovn.org/v1","kind: ClusterUserDefinedNetwork","metadata:","  name: vm-production-network","  annotations:","    vpc.roks.ibm.com/zone: \"us-south-1\"","    vpc.roks.ibm.com/cidr: \"10.240.64.0/24\"","    vpc.roks.ibm.com/vpc-id: \"r006-abc123\"","    vpc.roks.ibm.com/vlan-id: \"100\"","    vpc.roks.ibm.com/security-group-ids: \"r006-sg-web,r006-sg-mgmt\"","    vpc.roks.ibm.com/acl-id: \"r006-acl-prod\"","spec:","  topology: LocalNet"]),
        h2("12.3 Example VM"),
        ...cd(["apiVersion: kubevirt.io/v1","kind: VirtualMachine","metadata:","  name: web-server-1","  namespace: production","  annotations:","    vpc.roks.ibm.com/fip: \"true\"","spec:","  running: true","  template:","    spec:","      networks:","      - name: vpc-net","        multus:","          networkName: vm-production-network","      domain:","        resources:","          requests: { memory: 4Gi, cpu: \"2\" }","        devices:","          interfaces:","          - name: vpc-net","            bridge: {}","      volumes:","      - name: rootdisk","        containerDisk:","          image: quay.io/containerdisks/ubuntu:22.04","      - name: cloudinit","        cloudInitNoCloud:","          userData: |","            #cloud-config","            password: changeme"]),
        PB(),
        h1("13. Future Considerations"),
        bul("CRD graduation if annotations grow complex."),
        bul("CIDR auto-allocation via IPAM."),
        bul("SG lifecycle as K8s-native objects."),
        bul("Multi-zone CUDN (subnet per zone)."),
        bul("DNS integration (Custom Resolver records)."),
        bul("Quota pre-checks before VPC resource creation."),
      ],
    },
  ],
});

const OUT = "/sessions/compassionate-hopeful-davinci/mnt/roks_vpc_cni/ROKS_VPC_Network_Operator_Design.docx";
Packer.toBuffer(doc).then(buf => { fs.writeFileSync(OUT, buf); console.log("OK "+buf.length+" bytes"); }).catch(e => { console.error(e); process.exit(1); });
