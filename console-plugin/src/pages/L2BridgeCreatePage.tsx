import React, { useState } from 'react';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  Breadcrumb,
  BreadcrumbItem,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  ActionGroup,
  Button,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
  Switch,
  Title,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateL2BridgeRequest } from '../api/types';
import { useGateways, useNetworkDefinitions } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const L2BridgeCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const { gateways, loading: gatewaysLoading } = useGateways();
  const { networks, loading: networksLoading } = useNetworkDefinitions();

  // Core fields
  const [name, setName] = useState('');
  const [bridgeType, setBridgeType] = useState('gretap-wireguard');
  const [gateway, setGateway] = useState('');
  const [network, setNetwork] = useState('');
  const [networkKind, setNetworkKind] = useState('ClusterUserDefinedNetwork');
  const [remoteEndpoint, setRemoteEndpoint] = useState('');

  // WireGuard fields
  const [wgSecretName, setWgSecretName] = useState('');
  const [wgSecretKey, setWgSecretKey] = useState('privatekey');
  const [peerPublicKey, setPeerPublicKey] = useState('');
  const [listenPort, setListenPort] = useState('51820');
  const [tunnelAddrLocal, setTunnelAddrLocal] = useState('');
  const [tunnelAddrRemote, setTunnelAddrRemote] = useState('');

  // L2VPN fields
  const [nsxHost, setNsxHost] = useState('');
  const [l2vpnServiceID, setL2vpnServiceID] = useState('');
  const [credSecretName, setCredSecretName] = useState('');
  const [credSecretKey, setCredSecretKey] = useState('password');

  // EVPN fields
  const [localASN, setLocalASN] = useState('');
  const [peerASN, setPeerASN] = useState('');
  const [vni, setVni] = useState('');
  const [routeReflector, setRouteReflector] = useState('');
  const [frrImage, setFrrImage] = useState('');

  // Common
  const [tunnelMTU, setTunnelMTU] = useState('1400');
  const [mssClamp, setMssClamp] = useState(true);
  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const isTypeSpecificValid = (): boolean => {
    if (bridgeType === 'gretap-wireguard') {
      return (
        wgSecretName.trim() !== '' &&
        wgSecretKey.trim() !== '' &&
        peerPublicKey.trim() !== '' &&
        tunnelAddrLocal.trim() !== '' &&
        tunnelAddrRemote.trim() !== ''
      );
    }
    if (bridgeType === 'l2vpn') {
      return (
        nsxHost.trim() !== '' &&
        l2vpnServiceID.trim() !== '' &&
        credSecretName.trim() !== '' &&
        credSecretKey.trim() !== ''
      );
    }
    if (bridgeType === 'evpn-vxlan') {
      return (
        localASN.trim() !== '' &&
        peerASN.trim() !== '' &&
        vni.trim() !== ''
      );
    }
    return false;
  };

  const isValid =
    name.trim() !== '' &&
    bridgeType !== '' &&
    gateway !== '' &&
    network !== '' &&
    remoteEndpoint.trim() !== '' &&
    isTypeSpecificValid();

  const handleNetworkChange = (_e: React.FormEvent<HTMLSelectElement>, value: string) => {
    // value is encoded as "kind/name"
    const sepIdx = value.indexOf('/');
    if (sepIdx > 0) {
      setNetworkKind(value.substring(0, sepIdx));
      setNetwork(value.substring(sepIdx + 1));
    } else {
      setNetwork(value);
    }
  };

  const handleSubmit = async () => {
    if (!isValid) return;
    setSubmitting(true);
    setSubmitError('');

    const req: CreateL2BridgeRequest = {
      name: name.trim(),
      type: bridgeType,
      gatewayRef: gateway,
      networkRef: { name: network, kind: networkKind },
      remoteEndpoint: remoteEndpoint.trim(),
      tunnelMTU: parseInt(tunnelMTU, 10) || 1400,
      mssClamp,
    };

    if (bridgeType === 'gretap-wireguard') {
      req.wireguard = {
        privateKeySecretName: wgSecretName.trim(),
        privateKeySecretKey: wgSecretKey.trim(),
        peerPublicKey: peerPublicKey.trim(),
        listenPort: parseInt(listenPort, 10) || 51820,
        tunnelAddressLocal: tunnelAddrLocal.trim(),
        tunnelAddressRemote: tunnelAddrRemote.trim(),
      };
    } else if (bridgeType === 'l2vpn') {
      req.l2vpn = {
        nsxManagerHost: nsxHost.trim(),
        l2vpnServiceID: l2vpnServiceID.trim(),
        credentialsSecretName: credSecretName.trim(),
        credentialsSecretKey: credSecretKey.trim(),
      };
    } else if (bridgeType === 'evpn-vxlan') {
      req.evpn = {
        asn: parseInt(localASN, 10),
        peerASN: parseInt(peerASN, 10),
        vni: parseInt(vni, 10),
        routeReflector: routeReflector.trim() || undefined,
        frrImage: frrImage.trim() || undefined,
      };
    }

    const resp = await apiClient.createL2Bridge(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setSubmitting(false);
    } else {
      navigate('/vpc-networking/l2-bridges');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/l2-bridges">L2 Bridges</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '16px' }}>Create VPCL2Bridge</Title>
        <Text component={TextVariants.p} style={{ marginTop: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
          A VPCL2Bridge extends a Layer 2 overlay network to a remote site using GRETAP+WireGuard, NSX-T L2VPN, or EVPN-VXLAN tunneling.
        </Text>
      </PageSection>

      <PageSection>
        <Card>
          <CardBody>
            {submitError && (
              <Alert variant="danger" title={submitError} isInline style={{ marginBottom: '1rem' }} />
            )}
            <Form>
              {/* Name */}
              <FormGroup label="Name" isRequired fieldId="l2b-name">
                <TextInput id="l2b-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Kubernetes resource name. Use lowercase letters, numbers, and hyphens.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Type */}
              <FormGroup label="Type" isRequired fieldId="l2b-type">
                <FormSelect id="l2b-type" value={bridgeType} onChange={(_e, v) => setBridgeType(v)}>
                  <FormSelectOption value="gretap-wireguard" label="GRETAP + WireGuard" />
                  <FormSelectOption value="l2vpn" label="NSX-T L2VPN" />
                  <FormSelectOption value="evpn-vxlan" label="EVPN-VXLAN" />
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>GRETAP+WireGuard: encrypted Linux-to-Linux site-to-site L2. NSX-T L2VPN: extend into VMware environments. EVPN-VXLAN: standards-based multi-vendor L2 with BGP control plane.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Gateway */}
              <FormGroup label="Gateway" isRequired fieldId="l2b-gateway">
                <FormSelect
                  id="l2b-gateway"
                  value={gateway}
                  onChange={(_e, v) => setGateway(v)}
                  isDisabled={gatewaysLoading}
                >
                  <FormSelectOption value="" label="Select a gateway" isPlaceholder />
                  {gateways?.map((gw) => (
                    <FormSelectOption key={gw.name} value={gw.name} label={`${gw.name} (${gw.namespace})`} />
                  ))}
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>VPCGateway that provides the uplink for this bridge's tunnel endpoint</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Network */}
              <FormGroup label="Network" isRequired fieldId="l2b-network">
                <FormSelect
                  id="l2b-network"
                  value={network ? `${networkKind}/${network}` : ''}
                  onChange={handleNetworkChange}
                  isDisabled={networksLoading}
                >
                  <FormSelectOption value="" label="Select a network" isPlaceholder />
                  {networks?.map((n) => (
                    <FormSelectOption
                      key={`${n.kind}-${n.name}`}
                      value={`${n.kind}/${n.name}`}
                      label={`${n.name} (${n.kind === 'ClusterUserDefinedNetwork' ? 'CUDN' : 'UDN'} - ${n.topology})`}
                    />
                  ))}
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>The overlay network to extend across the bridge</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Remote Endpoint */}
              <FormGroup label="Remote Endpoint" isRequired fieldId="l2b-remote">
                <TextInput
                  id="l2b-remote"
                  value={remoteEndpoint}
                  onChange={(_e, v) => setRemoteEndpoint(v)}
                  isRequired
                  placeholder="e.g. 203.0.113.10"
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>IP address or hostname of the remote tunnel peer</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* GRETAP + WireGuard section */}
              {bridgeType === 'gretap-wireguard' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>WireGuard Configuration</Title>

                  <FormGroup label="WG Secret Name" isRequired fieldId="l2b-wg-secret-name">
                    <TextInput
                      id="l2b-wg-secret-name"
                      value={wgSecretName}
                      onChange={(_e, v) => setWgSecretName(v)}
                      isRequired
                      placeholder="e.g. wg-private-key"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Kubernetes Secret containing the WireGuard private key</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="WG Secret Key" isRequired fieldId="l2b-wg-secret-key">
                    <TextInput
                      id="l2b-wg-secret-key"
                      value={wgSecretKey}
                      onChange={(_e, v) => setWgSecretKey(v)}
                      isRequired
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Key within the Secret that holds the private key data</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Peer Public Key" isRequired fieldId="l2b-wg-peer-pubkey">
                    <TextInput
                      id="l2b-wg-peer-pubkey"
                      value={peerPublicKey}
                      onChange={(_e, v) => setPeerPublicKey(v)}
                      isRequired
                      placeholder="e.g. aB3d...base64..."
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Base64-encoded WireGuard public key of the remote peer</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Listen Port" fieldId="l2b-wg-port">
                    <TextInput
                      id="l2b-wg-port"
                      type="number"
                      value={listenPort}
                      onChange={(_e, v) => setListenPort(v)}
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>UDP port for WireGuard (default 51820)</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Tunnel Address Local" isRequired fieldId="l2b-wg-addr-local">
                    <TextInput
                      id="l2b-wg-addr-local"
                      value={tunnelAddrLocal}
                      onChange={(_e, v) => setTunnelAddrLocal(v)}
                      isRequired
                      placeholder="e.g. 10.99.0.1/30"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Local inner tunnel address in CIDR notation</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Tunnel Address Remote" isRequired fieldId="l2b-wg-addr-remote">
                    <TextInput
                      id="l2b-wg-addr-remote"
                      value={tunnelAddrRemote}
                      onChange={(_e, v) => setTunnelAddrRemote(v)}
                      isRequired
                      placeholder="e.g. 10.99.0.2/30"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Remote inner tunnel address in CIDR notation</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* L2VPN section */}
              {bridgeType === 'l2vpn' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>NSX-T L2VPN Configuration</Title>

                  <FormGroup label="NSX Manager Host" isRequired fieldId="l2b-nsx-host">
                    <TextInput
                      id="l2b-nsx-host"
                      value={nsxHost}
                      onChange={(_e, v) => setNsxHost(v)}
                      isRequired
                      placeholder="e.g. nsx-manager.example.com"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Hostname or IP address of the NSX-T Manager</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="L2VPN Service ID" isRequired fieldId="l2b-l2vpn-service-id">
                    <TextInput
                      id="l2b-l2vpn-service-id"
                      value={l2vpnServiceID}
                      onChange={(_e, v) => setL2vpnServiceID(v)}
                      isRequired
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>ID of the L2VPN service on the NSX-T Manager</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Credentials Secret Name" isRequired fieldId="l2b-cred-secret-name">
                    <TextInput
                      id="l2b-cred-secret-name"
                      value={credSecretName}
                      onChange={(_e, v) => setCredSecretName(v)}
                      isRequired
                      placeholder="e.g. nsx-credentials"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Kubernetes Secret containing the NSX-T Manager credentials</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Credentials Secret Key" isRequired fieldId="l2b-cred-secret-key">
                    <TextInput
                      id="l2b-cred-secret-key"
                      value={credSecretKey}
                      onChange={(_e, v) => setCredSecretKey(v)}
                      isRequired
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Key within the Secret that holds the password</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* EVPN-VXLAN section */}
              {bridgeType === 'evpn-vxlan' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>EVPN-VXLAN Configuration</Title>

                  <FormGroup label="Local ASN" isRequired fieldId="l2b-evpn-local-asn">
                    <TextInput
                      id="l2b-evpn-local-asn"
                      type="number"
                      value={localASN}
                      onChange={(_e, v) => setLocalASN(v)}
                      isRequired
                      placeholder="e.g. 65001"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Local BGP Autonomous System Number</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Peer ASN" isRequired fieldId="l2b-evpn-peer-asn">
                    <TextInput
                      id="l2b-evpn-peer-asn"
                      type="number"
                      value={peerASN}
                      onChange={(_e, v) => setPeerASN(v)}
                      isRequired
                      placeholder="e.g. 65002"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Remote peer BGP Autonomous System Number</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="VNI" isRequired fieldId="l2b-evpn-vni">
                    <TextInput
                      id="l2b-evpn-vni"
                      type="number"
                      value={vni}
                      onChange={(_e, v) => setVni(v)}
                      isRequired
                      placeholder="e.g. 10100"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>VXLAN Network Identifier for the L2 segment</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Route Reflector" fieldId="l2b-evpn-rr">
                    <TextInput
                      id="l2b-evpn-rr"
                      value={routeReflector}
                      onChange={(_e, v) => setRouteReflector(v)}
                      placeholder="e.g. 10.0.0.1"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Optional BGP route reflector address</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="FRR Image" fieldId="l2b-evpn-frr-image">
                    <TextInput
                      id="l2b-evpn-frr-image"
                      value={frrImage}
                      onChange={(_e, v) => setFrrImage(v)}
                      placeholder="e.g. quay.io/frrouting/frr:9.1.0"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Optional custom FRRouting container image</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* Common: Tunnel MTU */}
              <FormGroup label="Tunnel MTU" fieldId="l2b-mtu">
                <TextInput
                  id="l2b-mtu"
                  type="number"
                  value={tunnelMTU}
                  onChange={(_e, v) => setTunnelMTU(v)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Maximum transmission unit for the tunnel interface (default 1400)</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Common: MSS Clamping */}
              <FormGroup label="MSS Clamping" fieldId="l2b-mss-clamp">
                <Switch
                  id="l2b-mss-clamp"
                  label="Enabled"
                  labelOff="Disabled"
                  isChecked={mssClamp}
                  onChange={(_e, checked) => setMssClamp(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Clamp TCP MSS to prevent fragmentation over the tunnel</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <ActionGroup>
                <Button variant="primary" onClick={handleSubmit} isLoading={submitting} isDisabled={!isValid || submitting}>
                  Create
                </Button>
                <Button variant="link" onClick={() => navigate('/vpc-networking/l2-bridges')} isDisabled={submitting}>
                  Cancel
                </Button>
              </ActionGroup>
            </Form>
          </CardBody>
        </Card>
      </PageSection>
    </VPCNetworkingShell>
  );
};

L2BridgeCreatePage.displayName = 'L2BridgeCreatePage';
export default L2BridgeCreatePage;
