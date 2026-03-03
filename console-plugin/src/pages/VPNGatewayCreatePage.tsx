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
import { CreateVPNGatewayRequest } from '../api/types';
import { useGateways } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

interface TunnelEntry {
  name: string;
  remoteEndpoint: string;
  remoteNetworks: string;
  peerPublicKey: string;
  tunnelAddressLocal: string;
  tunnelAddressRemote: string;
  presharedKeySecret: string;
  presharedKeySecretKey: string;
}

const emptyTunnel = (): TunnelEntry => ({
  name: '',
  remoteEndpoint: '',
  remoteNetworks: '',
  peerPublicKey: '',
  tunnelAddressLocal: '',
  tunnelAddressRemote: '',
  presharedKeySecret: '',
  presharedKeySecretKey: '',
});

const VPNGatewayCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const { gateways, loading: gatewaysLoading } = useGateways();

  // Core fields
  const [name, setName] = useState('');
  const [protocol, setProtocol] = useState('wireguard');
  const [gateway, setGateway] = useState('');

  // WireGuard global config
  const [wgSecretName, setWgSecretName] = useState('');
  const [wgSecretKey, setWgSecretKey] = useState('privatekey');
  const [listenPort, setListenPort] = useState('51820');

  // OpenVPN global config
  const [ovpnCaSecret, setOvpnCaSecret] = useState('');
  const [ovpnCaKey, setOvpnCaKey] = useState('ca.crt');
  const [ovpnCertSecret, setOvpnCertSecret] = useState('');
  const [ovpnCertKey, setOvpnCertKey] = useState('server.crt');
  const [ovpnKeySecret, setOvpnKeySecret] = useState('');
  const [ovpnKeyKey, setOvpnKeyKey] = useState('server.key');
  const [ovpnPort, setOvpnPort] = useState('1194');
  const [ovpnProto, setOvpnProto] = useState('udp');
  const [ovpnCipher, setOvpnCipher] = useState('AES-256-GCM');
  const [ovpnClientSubnet, setOvpnClientSubnet] = useState('');

  // Tunnels
  const [tunnels, setTunnels] = useState<TunnelEntry[]>([emptyTunnel()]);

  // Local Networks (push routes to clients)
  const [localNetworks, setLocalNetworks] = useState<string[]>(['']);

  // Remote Access
  const [remoteAccessEnabled, setRemoteAccessEnabled] = useState(false);
  const [addressPool, setAddressPool] = useState('');
  const [dnsServers, setDnsServers] = useState('');
  const [maxClients, setMaxClients] = useState('10');

  // MTU
  const [tunnelMTU, setTunnelMTU] = useState('1420');
  const [mssClamp, setMssClamp] = useState(true);

  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const updateTunnel = (index: number, field: keyof TunnelEntry, value: string) => {
    setTunnels((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], [field]: value };
      return updated;
    });
  };

  const addTunnel = () => setTunnels((prev) => [...prev, emptyTunnel()]);

  const removeTunnel = (index: number) => {
    if (tunnels.length <= 1) return;
    setTunnels((prev) => prev.filter((_, i) => i !== index));
  };

  const isTunnelValid = (t: TunnelEntry): boolean => {
    if (!t.name.trim() || !t.remoteEndpoint.trim() || !t.remoteNetworks.trim()) return false;
    if (protocol === 'wireguard' && !t.peerPublicKey.trim()) return false;
    if (protocol === 'ipsec' && (!t.presharedKeySecret.trim() || !t.presharedKeySecretKey.trim())) return false;
    return true;
  };

  const isValid =
    name.trim() !== '' &&
    gateway !== '' &&
    tunnels.every(isTunnelValid) &&
    (protocol !== 'wireguard' || (wgSecretName.trim() !== '' && wgSecretKey.trim() !== '')) &&
    (protocol !== 'openvpn' || (ovpnCaSecret.trim() !== '' && ovpnCertSecret.trim() !== '' && ovpnKeySecret.trim() !== ''));

  const handleSubmit = async () => {
    if (!isValid) return;
    setSubmitting(true);
    setSubmitError('');

    const req: CreateVPNGatewayRequest = {
      name: name.trim(),
      protocol,
      gatewayRef: gateway,
      tunnels: tunnels.map((t) => ({
        name: t.name.trim(),
        remoteEndpoint: t.remoteEndpoint.trim(),
        remoteNetworks: t.remoteNetworks.split(',').map((s) => s.trim()).filter(Boolean),
        peerPublicKey: t.peerPublicKey.trim() || undefined,
        tunnelAddressLocal: t.tunnelAddressLocal.trim() || undefined,
        tunnelAddressRemote: t.tunnelAddressRemote.trim() || undefined,
        presharedKeySecret: t.presharedKeySecret.trim() || undefined,
        presharedKeySecretKey: t.presharedKeySecretKey.trim() || undefined,
      })),
      mtu: {
        tunnelMTU: parseInt(tunnelMTU, 10) || 1420,
        mssClamp,
      },
    };

    if (protocol === 'wireguard') {
      req.wireGuard = {
        privateKeySecret: wgSecretName.trim(),
        privateKeySecretKey: wgSecretKey.trim(),
        listenPort: parseInt(listenPort, 10) || 51820,
      };
    }

    if (protocol === 'openvpn') {
      req.openVPN = {
        caSecret: ovpnCaSecret.trim(),
        caSecretKey: ovpnCaKey.trim(),
        certSecret: ovpnCertSecret.trim(),
        certSecretKey: ovpnCertKey.trim(),
        keySecret: ovpnKeySecret.trim(),
        keySecretKey: ovpnKeyKey.trim(),
        listenPort: parseInt(ovpnPort, 10) || 1194,
        proto: ovpnProto,
        cipher: ovpnCipher.trim(),
        clientSubnet: ovpnClientSubnet.trim() || undefined,
      };
    }

    // Local networks
    const filteredNetworks = localNetworks.map((n) => n.trim()).filter(Boolean);
    if (filteredNetworks.length > 0) {
      req.localNetworks = filteredNetworks.map((cidr) => ({ cidr }));
    }

    // Remote access
    if (remoteAccessEnabled) {
      req.remoteAccess = {
        enabled: true,
        addressPool: addressPool.trim() || undefined,
        dnsServers: dnsServers.split(',').map((s) => s.trim()).filter(Boolean),
        maxClients: parseInt(maxClients, 10) || 10,
      };
    }

    const resp = await apiClient.createVPNGateway(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setSubmitting(false);
    } else {
      navigate('/vpc-networking/vpn-gateways');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/vpn-gateways">VPN Gateways</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '16px' }}>Create VPCVPNGateway</Title>
        <Text component={TextVariants.p} style={{ marginTop: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
          A VPCVPNGateway establishes encrypted VPN tunnels to remote sites using WireGuard, IPsec/StrongSwan, or OpenVPN.
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
              <FormGroup label="Name" isRequired fieldId="vpn-name">
                <TextInput id="vpn-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Kubernetes resource name. Use lowercase letters, numbers, and hyphens.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Protocol */}
              <FormGroup label="Protocol" isRequired fieldId="vpn-protocol">
                <FormSelect id="vpn-protocol" value={protocol} onChange={(_e, v) => setProtocol(v)}>
                  <FormSelectOption value="wireguard" label="WireGuard" />
                  <FormSelectOption value="ipsec" label="IPsec (StrongSwan)" />
                  <FormSelectOption value="openvpn" label="OpenVPN" />
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>WireGuard is lightweight and fast with modern cryptography. IPsec (StrongSwan) offers broader compatibility with legacy VPN devices.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Gateway */}
              <FormGroup label="Gateway" isRequired fieldId="vpn-gateway">
                <FormSelect
                  id="vpn-gateway"
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
                  <HelperText><HelperTextItem>VPCGateway that provides the FIP tunnel endpoint</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* WireGuard global config */}
              {protocol === 'wireguard' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>WireGuard Configuration</Title>

                  <FormGroup label="Private Key Secret" isRequired fieldId="vpn-wg-secret">
                    <TextInput
                      id="vpn-wg-secret"
                      value={wgSecretName}
                      onChange={(_e, v) => setWgSecretName(v)}
                      isRequired
                      placeholder="e.g. wg-private-key"
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Kubernetes Secret containing the WireGuard private key</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Secret Key" isRequired fieldId="vpn-wg-secret-key">
                    <TextInput
                      id="vpn-wg-secret-key"
                      value={wgSecretKey}
                      onChange={(_e, v) => setWgSecretKey(v)}
                      isRequired
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>The key within the Kubernetes Secret that holds the private key data (default: &quot;privatekey&quot;)</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Listen Port" fieldId="vpn-wg-port">
                    <TextInput
                      id="vpn-wg-port"
                      type="number"
                      value={listenPort}
                      onChange={(_e, v) => setListenPort(v)}
                    />
                    <FormHelperText>
                      <HelperText><HelperTextItem>UDP port for WireGuard to listen on (default: 51820). Must be unique per VPN gateway.</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>
                </>
              )}

              {protocol === 'openvpn' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>OpenVPN Configuration</Title>

                  <FormGroup label="CA Secret" isRequired fieldId="vpn-ovpn-ca">
                    <TextInput id="vpn-ovpn-ca" value={ovpnCaSecret} onChange={(_e, v) => setOvpnCaSecret(v)} isRequired placeholder="e.g. ovpn-ca" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the CA certificate</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="CA Secret Key" fieldId="vpn-ovpn-ca-key">
                    <TextInput id="vpn-ovpn-ca-key" value={ovpnCaKey} onChange={(_e, v) => setOvpnCaKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: ca.crt)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Server Cert Secret" isRequired fieldId="vpn-ovpn-cert">
                    <TextInput id="vpn-ovpn-cert" value={ovpnCertSecret} onChange={(_e, v) => setOvpnCertSecret(v)} isRequired placeholder="e.g. ovpn-cert" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the server certificate</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="Cert Secret Key" fieldId="vpn-ovpn-cert-key">
                    <TextInput id="vpn-ovpn-cert-key" value={ovpnCertKey} onChange={(_e, v) => setOvpnCertKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: server.crt)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Server Key Secret" isRequired fieldId="vpn-ovpn-key">
                    <TextInput id="vpn-ovpn-key" value={ovpnKeySecret} onChange={(_e, v) => setOvpnKeySecret(v)} isRequired placeholder="e.g. ovpn-key" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the server private key</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="Key Secret Key" fieldId="vpn-ovpn-key-key">
                    <TextInput id="vpn-ovpn-key-key" value={ovpnKeyKey} onChange={(_e, v) => setOvpnKeyKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: server.key)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Listen Port" fieldId="vpn-ovpn-port">
                    <TextInput id="vpn-ovpn-port" type="number" value={ovpnPort} onChange={(_e, v) => setOvpnPort(v)} />
                    <FormHelperText><HelperText><HelperTextItem>OpenVPN listen port (default: 1194)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Protocol" fieldId="vpn-ovpn-proto">
                    <FormSelect id="vpn-ovpn-proto" value={ovpnProto} onChange={(_e, v) => setOvpnProto(v)}>
                      <FormSelectOption value="udp" label="UDP (Recommended)" />
                      <FormSelectOption value="tcp" label="TCP" />
                    </FormSelect>
                    <FormHelperText><HelperText><HelperTextItem>Transport protocol. UDP is faster; TCP can traverse restrictive firewalls.</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Cipher" fieldId="vpn-ovpn-cipher">
                    <TextInput id="vpn-ovpn-cipher" value={ovpnCipher} onChange={(_e, v) => setOvpnCipher(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Data channel cipher (default: AES-256-GCM)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Client Subnet" fieldId="vpn-ovpn-client-subnet">
                    <TextInput id="vpn-ovpn-client-subnet" value={ovpnClientSubnet} onChange={(_e, v) => setOvpnClientSubnet(v)} placeholder="e.g. 10.8.0.0/24" />
                    <FormHelperText><HelperText><HelperTextItem>IP pool for remote-access clients. Leave empty for site-to-site only.</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* Tunnels */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Tunnels</Title>

              {tunnels.map((tunnel, idx) => (
                <Card key={idx} isCompact style={{ marginBottom: '16px', padding: '16px' }}>
                  <CardBody>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                      <Title headingLevel="h4">Tunnel {idx + 1}</Title>
                      {tunnels.length > 1 && (
                        <Button variant="link" isDanger onClick={() => removeTunnel(idx)}>Remove</Button>
                      )}
                    </div>

                    <FormGroup label="Tunnel Name" isRequired fieldId={`t-${idx}-name`}>
                      <TextInput id={`t-${idx}-name`} value={tunnel.name} onChange={(_e, v) => updateTunnel(idx, 'name', v)} isRequired />
                      <FormHelperText>
                        <HelperText><HelperTextItem>Unique identifier for this tunnel. Used as the WireGuard peer name or IPsec connection name.</HelperTextItem></HelperText>
                      </FormHelperText>
                    </FormGroup>

                    <FormGroup label="Remote Endpoint" isRequired fieldId={`t-${idx}-endpoint`}>
                      <TextInput id={`t-${idx}-endpoint`} value={tunnel.remoteEndpoint} onChange={(_e, v) => updateTunnel(idx, 'remoteEndpoint', v)} isRequired placeholder="e.g. 203.0.113.10" />
                      <FormHelperText>
                        <HelperText><HelperTextItem>Public IP address or hostname of the remote VPN peer</HelperTextItem></HelperText>
                      </FormHelperText>
                    </FormGroup>

                    <FormGroup label="Remote Networks" isRequired fieldId={`t-${idx}-networks`}>
                      <TextInput id={`t-${idx}-networks`} value={tunnel.remoteNetworks} onChange={(_e, v) => updateTunnel(idx, 'remoteNetworks', v)} isRequired placeholder="e.g. 10.0.0.0/24, 172.16.0.0/16" />
                      <FormHelperText>
                        <HelperText><HelperTextItem>Comma-separated CIDRs reachable via this tunnel</HelperTextItem></HelperText>
                      </FormHelperText>
                    </FormGroup>

                    {protocol === 'wireguard' && (
                      <>
                        <FormGroup label="Peer Public Key" isRequired fieldId={`t-${idx}-pubkey`}>
                          <TextInput id={`t-${idx}-pubkey`} value={tunnel.peerPublicKey} onChange={(_e, v) => updateTunnel(idx, 'peerPublicKey', v)} isRequired placeholder="Base64 WireGuard public key" />
                          <FormHelperText>
                            <HelperText><HelperTextItem>The remote peer&apos;s WireGuard public key. Generate with: wg genkey | tee privatekey | wg pubkey</HelperTextItem></HelperText>
                          </FormHelperText>
                        </FormGroup>
                        <FormGroup label="Tunnel Address Local" fieldId={`t-${idx}-addr-local`}>
                          <TextInput id={`t-${idx}-addr-local`} value={tunnel.tunnelAddressLocal} onChange={(_e, v) => updateTunnel(idx, 'tunnelAddressLocal', v)} placeholder="e.g. 10.99.0.1/30" />
                          <FormHelperText>
                            <HelperText><HelperTextItem>Inner tunnel IP in CIDR notation for this end (e.g. 10.99.0.1/30). Use a /30 point-to-point block.</HelperTextItem></HelperText>
                          </FormHelperText>
                        </FormGroup>
                        <FormGroup label="Tunnel Address Remote" fieldId={`t-${idx}-addr-remote`}>
                          <TextInput id={`t-${idx}-addr-remote`} value={tunnel.tunnelAddressRemote} onChange={(_e, v) => updateTunnel(idx, 'tunnelAddressRemote', v)} placeholder="e.g. 10.99.0.2/30" />
                          <FormHelperText>
                            <HelperText><HelperTextItem>Inner tunnel IP in CIDR notation for the remote end. Must be in the same subnet as local address.</HelperTextItem></HelperText>
                          </FormHelperText>
                        </FormGroup>
                      </>
                    )}

                    {protocol === 'ipsec' && (
                      <>
                        <FormGroup label="PSK Secret Name" isRequired fieldId={`t-${idx}-psk-secret`}>
                          <TextInput id={`t-${idx}-psk-secret`} value={tunnel.presharedKeySecret} onChange={(_e, v) => updateTunnel(idx, 'presharedKeySecret', v)} isRequired placeholder="e.g. ipsec-psk-tunnel1" />
                          <FormHelperText>
                            <HelperText><HelperTextItem>Kubernetes Secret containing the IPsec pre-shared key for this tunnel</HelperTextItem></HelperText>
                          </FormHelperText>
                        </FormGroup>
                        <FormGroup label="PSK Secret Key" isRequired fieldId={`t-${idx}-psk-key`}>
                          <TextInput id={`t-${idx}-psk-key`} value={tunnel.presharedKeySecretKey} onChange={(_e, v) => updateTunnel(idx, 'presharedKeySecretKey', v)} isRequired placeholder="e.g. psk" />
                          <FormHelperText>
                            <HelperText><HelperTextItem>The key within the Secret that holds the pre-shared key data (e.g. &quot;psk&quot;)</HelperTextItem></HelperText>
                          </FormHelperText>
                        </FormGroup>
                      </>
                    )}
                  </CardBody>
                </Card>
              ))}

              <Text component={TextVariants.small} style={{ marginBottom: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
                Add multiple tunnels to connect to different remote sites or for redundancy.
              </Text>
              <Button variant="secondary" onClick={addTunnel} style={{ marginBottom: '16px' }}>
                Add Tunnel
              </Button>

              {/* Local Networks (push routes to clients) */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Local Networks</Title>
              <Text component={TextVariants.small} style={{ marginBottom: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
                CIDRs of local networks to advertise to VPN peers. These are pushed as routes to connecting clients.
              </Text>

              {localNetworks.map((cidr, idx) => (
                <FormGroup key={idx} label={`Network ${idx + 1}`} fieldId={`ln-${idx}`}>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <TextInput
                      id={`ln-${idx}`}
                      value={cidr}
                      onChange={(_e, v) => {
                        setLocalNetworks((prev) => {
                          const updated = [...prev];
                          updated[idx] = v;
                          return updated;
                        });
                      }}
                      placeholder="e.g. 10.240.0.0/24"
                      style={{ flex: 1 }}
                    />
                    {localNetworks.length > 1 && (
                      <Button variant="plain" isDanger onClick={() => setLocalNetworks((prev) => prev.filter((_, i) => i !== idx))}>
                        Remove
                      </Button>
                    )}
                  </div>
                </FormGroup>
              ))}
              <Button variant="secondary" onClick={() => setLocalNetworks((prev) => [...prev, ''])} style={{ marginBottom: '16px' }}>
                Add Network
              </Button>

              {/* Remote Access */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Remote Access (Client-to-Site)</Title>

              <FormGroup label="Enable Remote Access" fieldId="vpn-remote-access">
                <Switch
                  id="vpn-remote-access"
                  label="Enabled"
                  labelOff="Disabled"
                  isChecked={remoteAccessEnabled}
                  onChange={(_e, checked) => setRemoteAccessEnabled(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Allow individual clients to connect to the VPN (client-to-site mode)</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {remoteAccessEnabled && (
                <>
                  <FormGroup label="Address Pool" fieldId="vpn-ra-pool">
                    <TextInput id="vpn-ra-pool" value={addressPool} onChange={(_e, v) => setAddressPool(v)} placeholder="e.g. 10.200.0.0/24" />
                    <FormHelperText><HelperText><HelperTextItem>IP pool for remote access clients. Leave empty to use the OpenVPN client subnet.</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="DNS Servers" fieldId="vpn-ra-dns">
                    <TextInput id="vpn-ra-dns" value={dnsServers} onChange={(_e, v) => setDnsServers(v)} placeholder="e.g. 8.8.8.8, 8.8.4.4" />
                    <FormHelperText><HelperText><HelperTextItem>Comma-separated DNS servers pushed to connecting clients</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="Max Clients" fieldId="vpn-ra-max">
                    <TextInput id="vpn-ra-max" type="number" value={maxClients} onChange={(_e, v) => setMaxClients(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Maximum concurrent remote access clients (default: 10)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* MTU Settings */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>MTU Settings</Title>

              <FormGroup label="Tunnel MTU" fieldId="vpn-mtu">
                <TextInput
                  id="vpn-mtu"
                  type="number"
                  value={tunnelMTU}
                  onChange={(_e, v) => setTunnelMTU(v)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Maximum transmission unit for tunnel interfaces (default 1420)</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="MSS Clamping" fieldId="vpn-mss-clamp">
                <Switch
                  id="vpn-mss-clamp"
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
                <Button variant="link" onClick={() => navigate('/vpc-networking/vpn-gateways')} isDisabled={submitting}>
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

VPNGatewayCreatePage.displayName = 'VPNGatewayCreatePage';
export default VPNGatewayCreatePage;
