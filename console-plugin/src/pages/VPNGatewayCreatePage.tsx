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

  // Tunnels
  const [tunnels, setTunnels] = useState<TunnelEntry[]>([emptyTunnel()]);

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
    (protocol !== 'wireguard' || (wgSecretName.trim() !== '' && wgSecretKey.trim() !== ''));

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
          A VPCVPNGateway establishes encrypted VPN tunnels to remote sites using WireGuard or IPsec/StrongSwan.
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
                </FormSelect>
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
                  </FormGroup>

                  <FormGroup label="Listen Port" fieldId="vpn-wg-port">
                    <TextInput
                      id="vpn-wg-port"
                      type="number"
                      value={listenPort}
                      onChange={(_e, v) => setListenPort(v)}
                    />
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
                    </FormGroup>

                    <FormGroup label="Remote Endpoint" isRequired fieldId={`t-${idx}-endpoint`}>
                      <TextInput id={`t-${idx}-endpoint`} value={tunnel.remoteEndpoint} onChange={(_e, v) => updateTunnel(idx, 'remoteEndpoint', v)} isRequired placeholder="e.g. 203.0.113.10" />
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
                        </FormGroup>
                        <FormGroup label="Tunnel Address Local" fieldId={`t-${idx}-addr-local`}>
                          <TextInput id={`t-${idx}-addr-local`} value={tunnel.tunnelAddressLocal} onChange={(_e, v) => updateTunnel(idx, 'tunnelAddressLocal', v)} placeholder="e.g. 10.99.0.1/30" />
                        </FormGroup>
                        <FormGroup label="Tunnel Address Remote" fieldId={`t-${idx}-addr-remote`}>
                          <TextInput id={`t-${idx}-addr-remote`} value={tunnel.tunnelAddressRemote} onChange={(_e, v) => updateTunnel(idx, 'tunnelAddressRemote', v)} placeholder="e.g. 10.99.0.2/30" />
                        </FormGroup>
                      </>
                    )}

                    {protocol === 'ipsec' && (
                      <>
                        <FormGroup label="PSK Secret Name" isRequired fieldId={`t-${idx}-psk-secret`}>
                          <TextInput id={`t-${idx}-psk-secret`} value={tunnel.presharedKeySecret} onChange={(_e, v) => updateTunnel(idx, 'presharedKeySecret', v)} isRequired placeholder="e.g. ipsec-psk-tunnel1" />
                        </FormGroup>
                        <FormGroup label="PSK Secret Key" isRequired fieldId={`t-${idx}-psk-key`}>
                          <TextInput id={`t-${idx}-psk-key`} value={tunnel.presharedKeySecretKey} onChange={(_e, v) => updateTunnel(idx, 'presharedKeySecretKey', v)} isRequired placeholder="e.g. psk" />
                        </FormGroup>
                      </>
                    )}
                  </CardBody>
                </Card>
              ))}

              <Button variant="secondary" onClick={addTunnel} style={{ marginBottom: '16px' }}>
                Add Tunnel
              </Button>

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
