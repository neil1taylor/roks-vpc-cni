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
  ActionGroup,
  Button,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
  FormSelect,
  FormSelectOption,
  Title,
  Text,
  TextVariants,
  Switch,
  ExpandableSection,
} from '@patternfly/react-core';
import { PlusCircleIcon, MinusCircleIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateRouterRequest, CreateRouterNetworkDHCP } from '../api/types';
import { useGateways, useNetworkDefinitions } from '../api/hooks';
import { isValidIPv4, isValidMAC } from '../utils/validators';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

interface ReservationEntry {
  mac: string;
  ip: string;
  hostname: string;
}

interface NetworkEntry {
  name: string;
  address: string;
  dhcpOverride: 'inherit' | 'enabled' | 'disabled';
  dhcpRangeStart: string;
  dhcpRangeEnd: string;
  dhcpLeaseTime: string;
  reservations: ReservationEntry[];
}

const emptyNetwork = (): NetworkEntry => ({
  name: '',
  address: '',
  dhcpOverride: 'inherit',
  dhcpRangeStart: '',
  dhcpRangeEnd: '',
  dhcpLeaseTime: '',
  reservations: [],
});

const RouterCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [gateway, setGateway] = useState('');
  const [networks, setNetworks] = useState<NetworkEntry[]>([emptyNetwork()]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // DHCP global state
  const [dhcpEnabled, setDhcpEnabled] = useState(false);
  const [dhcpLeaseTime, setDhcpLeaseTime] = useState('12h');
  const [dhcpNameservers, setDhcpNameservers] = useState('');
  const [dhcpSearchDomains, setDhcpSearchDomains] = useState('');
  const [dhcpLocalDomain, setDhcpLocalDomain] = useState('');

  const { gateways, loading: gatewaysLoading } = useGateways();
  const { networks: networkDefs, loading: networksLoading } = useNetworkDefinitions();
  const localNetNetworks = networkDefs?.filter((n) => n.topology === 'LocalNet') || [];

  // Track whether namespace was auto-set from gateway
  const [namespaceFromGateway, setNamespaceFromGateway] = useState(false);

  const selectedGateway = gateways?.find((g) => g.name === gateway);

  const handleGatewayChange = (value: string) => {
    setGateway(value);
    const gw = gateways?.find((g) => g.name === value);
    if (gw) {
      setNamespace(gw.namespace);
      setNamespaceFromGateway(true);
    } else {
      setNamespaceFromGateway(false);
    }
  };

  // Filter out already-selected networks from dropdown options
  const selectedNetworkNames = new Set(networks.map((n) => n.name).filter(Boolean));

  const hasValidNetwork = networks.some(
    (n) => n.name.trim() !== '' && n.address.trim() !== '' && isValidIPv4(n.address.trim()),
  );
  const isValid = name.trim() !== '' && gateway !== '' && hasValidNetwork;

  const updateNetwork = (index: number, field: keyof NetworkEntry, value: string) => {
    setNetworks((prev) => prev.map((n, i) => (i === index ? { ...n, [field]: value } : n)));
  };

  const addNetwork = () => {
    setNetworks((prev) => [...prev, emptyNetwork()]);
  };

  const removeNetwork = (index: number) => {
    if (networks.length <= 1) return;
    setNetworks((prev) => prev.filter((_, i) => i !== index));
  };

  const addReservation = (netIndex: number) => {
    setNetworks((prev) =>
      prev.map((n, i) =>
        i === netIndex
          ? { ...n, reservations: [...n.reservations, { mac: '', ip: '', hostname: '' }] }
          : n,
      ),
    );
  };

  const removeReservation = (netIndex: number, resIndex: number) => {
    setNetworks((prev) =>
      prev.map((n, i) =>
        i === netIndex
          ? { ...n, reservations: n.reservations.filter((_, ri) => ri !== resIndex) }
          : n,
      ),
    );
  };

  const updateReservation = (
    netIndex: number,
    resIndex: number,
    field: keyof ReservationEntry,
    value: string,
  ) => {
    setNetworks((prev) =>
      prev.map((n, i) =>
        i === netIndex
          ? {
              ...n,
              reservations: n.reservations.map((r, ri) =>
                ri === resIndex ? { ...r, [field]: value } : r,
              ),
            }
          : n,
      ),
    );
  };

  const handleSubmit = async () => {
    if (!isValid) return;
    setIsSubmitting(true);
    setSubmitError(null);

    const validNetworks = networks
      .filter((n) => n.name.trim() !== '' && n.address.trim() !== '')
      .map((n) => {
        const net: { name: string; address: string; dhcp?: CreateRouterNetworkDHCP } = {
          name: n.name.trim(),
          address: n.address.trim(),
        };
        if (dhcpEnabled && n.dhcpOverride !== 'inherit') {
          const dhcp: CreateRouterNetworkDHCP = { override: n.dhcpOverride };
          if (n.dhcpRangeStart && n.dhcpRangeEnd) {
            dhcp.rangeStart = n.dhcpRangeStart;
            dhcp.rangeEnd = n.dhcpRangeEnd;
          }
          if (n.dhcpLeaseTime) {
            dhcp.leaseTime = n.dhcpLeaseTime;
          }
          const validRes = n.reservations.filter((r) => r.mac.trim() && r.ip.trim());
          if (validRes.length > 0) {
            dhcp.reservations = validRes.map((r) => ({
              mac: r.mac.trim(),
              ip: r.ip.trim(),
              hostname: r.hostname.trim() || undefined,
            }));
          }
          net.dhcp = dhcp;
        }
        return net;
      });

    const req: CreateRouterRequest = {
      name: name.trim(),
      namespace: namespace.trim() || undefined,
      gateway: gateway.trim(),
      networks: validNetworks,
    };

    if (dhcpEnabled) {
      const nsArr = dhcpNameservers.split(',').map((s) => s.trim()).filter(Boolean);
      const sdArr = dhcpSearchDomains.split(',').map((s) => s.trim()).filter(Boolean);
      req.dhcp = {
        enabled: true,
        leaseTime: dhcpLeaseTime || undefined,
        dns:
          nsArr.length > 0 || sdArr.length > 0 || dhcpLocalDomain
            ? {
                nameservers: nsArr.length > 0 ? nsArr : undefined,
                searchDomains: sdArr.length > 0 ? sdArr : undefined,
                localDomain: dhcpLocalDomain || undefined,
              }
            : undefined,
      };
    }

    const resp = await apiClient.createRouter(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setIsSubmitting(false);
    } else {
      navigate('/vpc-networking/routers');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/routers">Routers</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '16px' }}>Create VPCRouter</Title>
        <Text component={TextVariants.p} style={{ marginTop: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
          A VPCRouter deploys a router pod that connects workload networks to a VPCGateway,
          providing IP forwarding, NAT, and DHCP for VMs on those networks.
        </Text>
      </PageSection>

      <PageSection>
        <Card>
          <CardBody>
            {submitError && (
              <Alert variant="danger" title={submitError} isInline style={{ marginBottom: '1rem' }} />
            )}
            <Form>
              <FormGroup label="Name" isRequired fieldId="rt-name">
                <TextInput id="rt-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Kubernetes resource name. Use lowercase letters, numbers, and hyphens.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Gateway" isRequired fieldId="rt-gateway">
                <FormSelect
                  id="rt-gateway"
                  value={gateway}
                  onChange={(_e, v) => handleGatewayChange(v)}
                  isDisabled={gatewaysLoading}
                >
                  <FormSelectOption value="" label="Select a gateway" isPlaceholder />
                  {gateways?.map((gw) => (
                    <FormSelectOption
                      key={gw.name}
                      value={gw.name}
                      label={`${gw.name} (${gw.zone}, ${gw.namespace})`}
                    />
                  ))}
                </FormSelect>
                {!gatewaysLoading && (!gateways || gateways.length === 0) && (
                  <FormHelperText>
                    <HelperText><HelperTextItem variant="warning">No gateways available. Create a VPCGateway first.</HelperTextItem></HelperText>
                  </FormHelperText>
                )}
                {(gatewaysLoading || (gateways && gateways.length > 0)) && (
                  <FormHelperText>
                    <HelperText><HelperTextItem>The VPCGateway this router connects to for external traffic</HelperTextItem></HelperText>
                  </FormHelperText>
                )}
              </FormGroup>

              <FormGroup label="Namespace" fieldId="rt-namespace">
                <TextInput
                  id="rt-namespace"
                  value={namespace}
                  onChange={(_e, v) => setNamespace(v)}
                  isDisabled={namespaceFromGateway}
                />
                <FormHelperText>
                  <HelperText>
                    <HelperTextItem>
                      {namespaceFromGateway
                        ? `Auto-set to match the selected gateway's namespace (${selectedGateway?.namespace})`
                        : 'Namespace for the VPCRouter resource'}
                    </HelperTextItem>
                  </HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Networks" isRequired fieldId="rt-networks">
                <FormHelperText>
                  <HelperText><HelperTextItem>At least one network with a valid IP address is required. Pick an unused IP within each network's CIDR range.</HelperTextItem></HelperText>
                </FormHelperText>
                {networks.map((net, i) => {
                  const addressValid = net.address === '' || isValidIPv4(net.address);
                  // Available networks: those not already selected by other rows
                  const availableNetworks = localNetNetworks.filter(
                    (n) => n.name === net.name || !selectedNetworkNames.has(n.name),
                  );
                  return (
                    <div key={i} style={{ marginBottom: '12px' }}>
                      <div style={{ display: 'flex', gap: '8px', alignItems: 'flex-start' }}>
                        <FormSelect
                          aria-label={`Network ${i + 1} name`}
                          value={net.name}
                          onChange={(_e, v) => updateNetwork(i, 'name', v)}
                          isDisabled={networksLoading}
                          style={{ flex: 1 }}
                        >
                          <FormSelectOption value="" label="Select a network" isPlaceholder />
                          {availableNetworks.map((n) => (
                            <FormSelectOption
                              key={`${n.kind}-${n.name}`}
                              value={n.name}
                              label={`${n.name} (${n.kind === 'ClusterUserDefinedNetwork' ? 'CUDN' : 'UDN'})`}
                            />
                          ))}
                        </FormSelect>
                        <div style={{ flex: 1 }}>
                          <TextInput
                            aria-label={`Network ${i + 1} address`}
                            value={net.address}
                            onChange={(_e, v) => updateNetwork(i, 'address', v)}
                            placeholder="Router address (e.g. 10.0.1.1)"
                            validated={addressValid ? 'default' : 'error'}
                          />
                          {!addressValid && (
                            <HelperText>
                              <HelperTextItem variant="error">Enter a valid IPv4 address</HelperTextItem>
                            </HelperText>
                          )}
                        </div>
                        <Button
                          variant="plain"
                          aria-label="Remove network"
                          onClick={() => removeNetwork(i)}
                          isDisabled={networks.length <= 1}
                        >
                          <MinusCircleIcon />
                        </Button>
                      </div>
                      {dhcpEnabled && net.name && (
                        <ExpandableSection
                          toggleText="DHCP Settings"
                          isIndented
                          style={{ marginTop: '4px' }}
                        >
                          <div style={{ display: 'flex', gap: '16px', flexWrap: 'wrap', marginBottom: '8px' }}>
                            <FormGroup label="Override" fieldId={`net-${i}-dhcp-override`} style={{ minWidth: '160px' }}>
                              <FormSelect
                                id={`net-${i}-dhcp-override`}
                                value={net.dhcpOverride}
                                onChange={(_e, v) => updateNetwork(i, 'dhcpOverride', v)}
                              >
                                <FormSelectOption value="inherit" label="Inherit global" />
                                <FormSelectOption value="enabled" label="Enabled" />
                                <FormSelectOption value="disabled" label="Disabled" />
                              </FormSelect>
                            </FormGroup>
                            <FormGroup label="Range Start" fieldId={`net-${i}-range-start`} style={{ flex: 1 }}>
                              <TextInput
                                id={`net-${i}-range-start`}
                                value={net.dhcpRangeStart}
                                onChange={(_e, v) => updateNetwork(i, 'dhcpRangeStart', v)}
                                placeholder="Auto if empty"
                              />
                            </FormGroup>
                            <FormGroup label="Range End" fieldId={`net-${i}-range-end`} style={{ flex: 1 }}>
                              <TextInput
                                id={`net-${i}-range-end`}
                                value={net.dhcpRangeEnd}
                                onChange={(_e, v) => updateNetwork(i, 'dhcpRangeEnd', v)}
                                placeholder="Auto if empty"
                              />
                            </FormGroup>
                            <FormGroup label="Lease Time" fieldId={`net-${i}-lease`} style={{ minWidth: '120px' }}>
                              <TextInput
                                id={`net-${i}-lease`}
                                value={net.dhcpLeaseTime}
                                onChange={(_e, v) => updateNetwork(i, 'dhcpLeaseTime', v)}
                                placeholder={dhcpLeaseTime || '12h'}
                              />
                            </FormGroup>
                          </div>
                          <Title headingLevel="h5" style={{ marginBottom: '4px' }}>Static Reservations</Title>
                          {net.reservations.map((res, ri) => {
                            const macValid = res.mac === '' || isValidMAC(res.mac);
                            const ipValid = res.ip === '' || isValidIPv4(res.ip);
                            return (
                              <div key={ri} style={{ display: 'flex', gap: '8px', alignItems: 'flex-start', marginBottom: '4px' }}>
                                <div style={{ flex: 2 }}>
                                  <TextInput
                                    aria-label="MAC address"
                                    value={res.mac}
                                    onChange={(_e, v) => updateReservation(i, ri, 'mac', v)}
                                    placeholder="fa:16:3e:aa:bb:cc"
                                    validated={macValid ? 'default' : 'error'}
                                  />
                                </div>
                                <div style={{ flex: 2 }}>
                                  <TextInput
                                    aria-label="IP address"
                                    value={res.ip}
                                    onChange={(_e, v) => updateReservation(i, ri, 'ip', v)}
                                    placeholder="10.0.1.50"
                                    validated={ipValid ? 'default' : 'error'}
                                  />
                                </div>
                                <div style={{ flex: 2 }}>
                                  <TextInput
                                    aria-label="Hostname"
                                    value={res.hostname}
                                    onChange={(_e, v) => updateReservation(i, ri, 'hostname', v)}
                                    placeholder="Optional hostname"
                                  />
                                </div>
                                <Button variant="plain" aria-label="Remove reservation" onClick={() => removeReservation(i, ri)}>
                                  <MinusCircleIcon />
                                </Button>
                              </div>
                            );
                          })}
                          <Button variant="link" icon={<PlusCircleIcon />} onClick={() => addReservation(i)} size="sm">
                            Add reservation
                          </Button>
                        </ExpandableSection>
                      )}
                    </div>
                  );
                })}
                {!networksLoading && localNetNetworks.length === 0 && (
                  <Alert variant="warning" isInline title="No LocalNet networks available. Create a LocalNet CUDN or UDN first." style={{ marginBottom: '8px' }} />
                )}
                <Button variant="link" icon={<PlusCircleIcon />} onClick={addNetwork}>
                  Add network
                </Button>
              </FormGroup>

              <FormGroup fieldId="rt-dhcp-toggle">
                <Switch
                  id="rt-dhcp-toggle"
                  label="DHCP Server Enabled"
                  labelOff="DHCP Server Disabled"
                  isChecked={dhcpEnabled}
                  onChange={(_e, checked) => setDhcpEnabled(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Enable the built-in dnsmasq DHCP server for workload networks</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {dhcpEnabled && (
                <ExpandableSection toggleText="DHCP Global Settings" isIndented>
                  <div style={{ display: 'flex', gap: '16px', flexWrap: 'wrap' }}>
                    <FormGroup label="Lease Time" fieldId="dhcp-lease-time" style={{ minWidth: '180px' }}>
                      <FormSelect
                        id="dhcp-lease-time"
                        value={dhcpLeaseTime}
                        onChange={(_e, v) => setDhcpLeaseTime(v)}
                      >
                        <FormSelectOption value="30m" label="30 minutes" />
                        <FormSelectOption value="1h" label="1 hour" />
                        <FormSelectOption value="6h" label="6 hours" />
                        <FormSelectOption value="12h" label="12 hours (default)" />
                        <FormSelectOption value="24h" label="24 hours" />
                      </FormSelect>
                    </FormGroup>
                    <FormGroup label="DNS Nameservers" fieldId="dhcp-nameservers" style={{ flex: 1, minWidth: '200px' }}>
                      <TextInput
                        id="dhcp-nameservers"
                        value={dhcpNameservers}
                        onChange={(_e, v) => setDhcpNameservers(v)}
                        placeholder="8.8.8.8, 8.8.4.4"
                      />
                      <FormHelperText>
                        <HelperText><HelperTextItem>Comma-separated DNS server IPs</HelperTextItem></HelperText>
                      </FormHelperText>
                    </FormGroup>
                    <FormGroup label="Search Domains" fieldId="dhcp-search-domains" style={{ flex: 1, minWidth: '200px' }}>
                      <TextInput
                        id="dhcp-search-domains"
                        value={dhcpSearchDomains}
                        onChange={(_e, v) => setDhcpSearchDomains(v)}
                        placeholder="example.com, internal.local"
                      />
                      <FormHelperText>
                        <HelperText><HelperTextItem>Comma-separated DNS search domains</HelperTextItem></HelperText>
                      </FormHelperText>
                    </FormGroup>
                    <FormGroup label="Local Domain" fieldId="dhcp-local-domain" style={{ minWidth: '180px' }}>
                      <TextInput
                        id="dhcp-local-domain"
                        value={dhcpLocalDomain}
                        onChange={(_e, v) => setDhcpLocalDomain(v)}
                        placeholder="local.lan"
                      />
                    </FormGroup>
                  </div>
                </ExpandableSection>
              )}

              <ActionGroup>
                <Button variant="primary" onClick={handleSubmit} isLoading={isSubmitting} isDisabled={!isValid || isSubmitting}>
                  Create
                </Button>
                <Button variant="link" onClick={() => navigate('/vpc-networking/routers')} isDisabled={isSubmitting}>
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

export default RouterCreatePage;
