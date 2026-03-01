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
  Switch,
  FormSelect,
  FormSelectOption,
  Radio,
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateGatewayRequest } from '../api/types';
import { usePARs } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const GatewayCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [zone, setZone] = useState('');
  const [uplinkNetwork, setUplinkNetwork] = useState('');
  const [transitAddress, setTransitAddress] = useState('');
  const [transitCIDR, setTransitCIDR] = useState('');
  const [parEnabled, setPAREnabled] = useState(false);
  const [parPrefixLength, setPARPrefixLength] = useState('32');
  const [parMode, setPARMode] = useState<'create' | 'existing'>('create');
  const [parID, setPARID] = useState('');
  const { pars } = usePARs();
  const unattachedPARs = pars?.filter((p) => !p.gatewayName) || [];
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const isValid = name.trim() !== '' && zone.trim() !== '' && uplinkNetwork.trim() !== '' && transitAddress.trim() !== '';

  const handleSubmit = async () => {
    if (!isValid) return;
    setIsSubmitting(true);
    setSubmitError(null);

    const req: CreateGatewayRequest = {
      name: name.trim(),
      namespace: namespace.trim() || undefined,
      zone: zone.trim(),
      uplinkNetwork: uplinkNetwork.trim(),
      transitAddress: transitAddress.trim(),
      transitCIDR: transitCIDR.trim() || undefined,
      parEnabled: parEnabled || undefined,
      parPrefixLength: parEnabled && parMode === 'create' ? parseInt(parPrefixLength, 10) : undefined,
      parID: parEnabled && parMode === 'existing' && parID ? parID : undefined,
    };

    const resp = await apiClient.createGateway(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setIsSubmitting(false);
    } else {
      navigate('/vpc-networking/gateways');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/gateways">Gateways</Link></BreadcrumbItem>
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
              <FormGroup label="Name" isRequired fieldId="gw-name">
                <TextInput id="gw-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Unique name for the VPCGateway resource</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Namespace" fieldId="gw-namespace">
                <TextInput id="gw-namespace" value={namespace} onChange={(_e, v) => setNamespace(v)} />
              </FormGroup>

              <FormGroup label="Zone" isRequired fieldId="gw-zone">
                <TextInput id="gw-zone" value={zone} onChange={(_e, v) => setZone(v)} isRequired placeholder="e.g. eu-de-1" />
                <FormHelperText>
                  <HelperText><HelperTextItem>VPC availability zone</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Uplink Network" isRequired fieldId="gw-uplink">
                <TextInput id="gw-uplink" value={uplinkNetwork} onChange={(_e, v) => setUplinkNetwork(v)} isRequired placeholder="e.g. localnet-uplink" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Name of the LocalNet CUDN/UDN for the uplink</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Transit Address" isRequired fieldId="gw-transit-addr">
                <TextInput id="gw-transit-addr" value={transitAddress} onChange={(_e, v) => setTransitAddress(v)} isRequired placeholder="e.g. 192.168.255.1" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Gateway IP address on the transit network</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Transit CIDR" fieldId="gw-transit-cidr">
                <TextInput id="gw-transit-cidr" value={transitCIDR} onChange={(_e, v) => setTransitCIDR(v)} placeholder="e.g. 192.168.255.0/24" />
                <FormHelperText>
                  <HelperText><HelperTextItem>CIDR block for the transit network (optional)</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Public Address Range (PAR)" fieldId="gw-par-enabled">
                <Switch
                  id="gw-par-enabled"
                  label="Enabled"
                  labelOff="Disabled"
                  isChecked={parEnabled}
                  onChange={(_e, checked) => setPAREnabled(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Provision a block of public IPs routed through this gateway</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {parEnabled && (
                <>
                  <FormGroup label="PAR Source" fieldId="gw-par-mode">
                    <Radio
                      id="gw-par-mode-create"
                      name="par-mode"
                      label="Create new PAR"
                      isChecked={parMode === 'create'}
                      onChange={() => setPARMode('create')}
                    />
                    <Radio
                      id="gw-par-mode-existing"
                      name="par-mode"
                      label="Use existing PAR"
                      isChecked={parMode === 'existing'}
                      onChange={() => setPARMode('existing')}
                      style={{ marginTop: '8px' }}
                    />
                  </FormGroup>

                  {parMode === 'create' && (
                    <FormGroup label="PAR Prefix Length" fieldId="gw-par-prefix">
                      <FormSelect id="gw-par-prefix" value={parPrefixLength} onChange={(_e, v) => setPARPrefixLength(v)}>
                        <FormSelectOption value="28" label="/28 (16 IPs)" />
                        <FormSelectOption value="29" label="/29 (8 IPs)" />
                        <FormSelectOption value="30" label="/30 (4 IPs)" />
                        <FormSelectOption value="31" label="/31 (2 IPs)" />
                        <FormSelectOption value="32" label="/32 (1 IP)" />
                      </FormSelect>
                    </FormGroup>
                  )}

                  {parMode === 'existing' && (
                    <FormGroup label="Existing PAR" fieldId="gw-par-existing">
                      <FormSelect
                        id="gw-par-existing"
                        value={parID}
                        onChange={(_e, v) => setPARID(v)}
                      >
                        <FormSelectOption value="" label="Select an unattached PAR" isPlaceholder />
                        {unattachedPARs.map((p) => (
                          <FormSelectOption
                            key={p.id}
                            value={p.id}
                            label={`${p.name || p.id} — ${p.cidr} (${p.zone})`}
                          />
                        ))}
                      </FormSelect>
                      {unattachedPARs.length === 0 && (
                        <FormHelperText>
                          <HelperText><HelperTextItem variant="warning">No unattached PARs available. Create one first.</HelperTextItem></HelperText>
                        </FormHelperText>
                      )}
                    </FormGroup>
                  )}
                </>
              )}

              <ActionGroup>
                <Button variant="primary" onClick={handleSubmit} isLoading={isSubmitting} isDisabled={!isValid || isSubmitting}>
                  Create
                </Button>
                <Button variant="link" onClick={() => navigate('/vpc-networking/gateways')} isDisabled={isSubmitting}>
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

export default GatewayCreatePage;
