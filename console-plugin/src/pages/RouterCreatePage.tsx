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
} from '@patternfly/react-core';
import { PlusCircleIcon, MinusCircleIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateRouterRequest } from '../api/types';
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

  const hasValidNetwork = networks.some((n) => n.name.trim() !== '' && n.address.trim() !== '');
  const isValid = name.trim() !== '' && gateway.trim() !== '' && hasValidNetwork;

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
                  <HelperText><HelperTextItem>Unique name for the VPCRouter resource</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Namespace" fieldId="rt-namespace">
                <TextInput id="rt-namespace" value={namespace} onChange={(_e, v) => setNamespace(v)} />
              </FormGroup>

              <FormGroup label="Gateway" isRequired fieldId="rt-gateway">
                <TextInput id="rt-gateway" value={gateway} onChange={(_e, v) => setGateway(v)} isRequired placeholder="e.g. my-gateway" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Name of the VPCGateway this router connects to</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Networks" isRequired fieldId="rt-networks">
                <FormHelperText>
                  <HelperText><HelperTextItem>At least one network is required</HelperTextItem></HelperText>
                </FormHelperText>
                {networks.map((net, i) => (
                  <div key={i} style={{ display: 'flex', gap: '8px', alignItems: 'flex-start', marginBottom: '8px' }}>
                    <TextInput
                      aria-label={`Network ${i + 1} name`}
                      value={net.name}
                      onChange={(_e, v) => updateNetwork(i, 'name', v)}
                      placeholder="Network name"
                      style={{ flex: 1 }}
                    />
                    <TextInput
                      aria-label={`Network ${i + 1} address`}
                      value={net.address}
                      onChange={(_e, v) => updateNetwork(i, 'address', v)}
                      placeholder="Router address (e.g. 10.0.1.1)"
                      style={{ flex: 1 }}
                    />
                    <Button
                      variant="plain"
                      aria-label="Remove network"
                      onClick={() => removeNetwork(i)}
                      isDisabled={networks.length <= 1}
                    >
                      <MinusCircleIcon />
                    </Button>
                  </div>
                ))}
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
