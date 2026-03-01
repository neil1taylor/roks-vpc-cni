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
} from '@patternfly/react-core';
import { PlusCircleIcon, MinusCircleIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateRouterRequest } from '../api/types';
import { useGateways, useNetworkDefinitions } from '../api/hooks';
import { isValidIPv4 } from '../utils/validators';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

interface NetworkEntry {
  name: string;
  address: string;
}

const RouterCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [gateway, setGateway] = useState('');
  const [networks, setNetworks] = useState<NetworkEntry[]>([{ name: '', address: '' }]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

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
    setNetworks((prev) => [...prev, { name: '', address: '' }]);
  };

  const removeNetwork = (index: number) => {
    if (networks.length <= 1) return;
    setNetworks((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSubmit = async () => {
    if (!isValid) return;
    setIsSubmitting(true);
    setSubmitError(null);

    const validNetworks = networks
      .filter((n) => n.name.trim() !== '' && n.address.trim() !== '')
      .map((n) => ({ name: n.name.trim(), address: n.address.trim() }));

    const req: CreateRouterRequest = {
      name: name.trim(),
      namespace: namespace.trim() || undefined,
      gateway: gateway.trim(),
      networks: validNetworks,
    };

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
                    <div key={i} style={{ display: 'flex', gap: '8px', alignItems: 'flex-start', marginBottom: '8px' }}>
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
                  );
                })}
                {!networksLoading && localNetNetworks.length === 0 && (
                  <Alert variant="warning" isInline title="No LocalNet networks available. Create a LocalNet CUDN or UDN first." style={{ marginBottom: '8px' }} />
                )}
                <Button variant="link" icon={<PlusCircleIcon />} onClick={addNetwork}>
                  Add network
                </Button>
              </FormGroup>

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
