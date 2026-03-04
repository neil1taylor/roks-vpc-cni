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
  Title,
  Text,
  TextVariants,
  FormSelect,
  FormSelectOption,
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateTraceflowRequest } from '../api/types';
import { useRouters } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const TraceflowCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const { routers, loading: routersLoading } = useRouters();

  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [sourceIP, setSourceIP] = useState('');
  const [destinationIP, setDestinationIP] = useState('');
  const [destinationPort, setDestinationPort] = useState('');
  const [protocol, setProtocol] = useState('TCP');
  const [router, setRouter] = useState('');

  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const isValid = name.trim() !== '' && destinationIP.trim() !== '' && router !== '';

  const handleSubmit = async () => {
    if (!isValid) return;
    setSubmitting(true);
    setSubmitError('');

    const req: CreateTraceflowRequest = {
      name: name.trim(),
      namespace: namespace.trim() || 'default',
      destinationIP: destinationIP.trim(),
      protocol,
      router,
    };

    if (sourceIP.trim()) {
      req.sourceIP = sourceIP.trim();
    }

    const portNum = parseInt(destinationPort, 10);
    if (!isNaN(portNum) && portNum > 0) {
      req.destinationPort = portNum;
    }

    const resp = await apiClient.createTraceflow(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setSubmitting(false);
    } else {
      navigate(`/vpc-networking/traceflows/${req.name}?ns=${encodeURIComponent(req.namespace || 'default')}`);
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/traceflows">Traceflows</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '16px' }}>Create VPCTraceflow</Title>
        <Text component={TextVariants.p} style={{ marginTop: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
          A VPCTraceflow injects a probe packet into a VPCRouter to diagnose network path reachability,
          recording hop-by-hop latency and NFTables rule hits along the way.
        </Text>
      </PageSection>

      <PageSection>
        <Card>
          <CardBody>
            {submitError && (
              <Alert variant="danger" title={submitError} isInline style={{ marginBottom: '1rem' }} />
            )}
            <Form>
              <FormGroup label="Name" isRequired fieldId="tf-name">
                <TextInput id="tf-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Kubernetes resource name. Use lowercase letters, numbers, and hyphens.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Namespace" fieldId="tf-namespace">
                <TextInput id="tf-namespace" value={namespace} onChange={(_e, v) => setNamespace(v)} />
                <FormHelperText>
                  <HelperText><HelperTextItem>Namespace for the traceflow resource. Defaults to &quot;default&quot;.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Router" isRequired fieldId="tf-router">
                <FormSelect
                  id="tf-router"
                  value={router}
                  onChange={(_e, v) => setRouter(v)}
                  isDisabled={routersLoading}
                >
                  <FormSelectOption value="" label="Select a router" isPlaceholder />
                  {routers?.map((rt) => (
                    <FormSelectOption key={rt.name} value={rt.name} label={`${rt.name} (${rt.namespace})`} />
                  ))}
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>VPCRouter that will execute the traceflow probe.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Source IP" fieldId="tf-source-ip">
                <TextInput id="tf-source-ip" value={sourceIP} onChange={(_e, v) => setSourceIP(v)} placeholder="e.g. 10.0.1.5" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Source IP address for the probe packet. Leave empty to use the router&apos;s transit IP.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Destination IP" isRequired fieldId="tf-dest-ip">
                <TextInput id="tf-dest-ip" value={destinationIP} onChange={(_e, v) => setDestinationIP(v)} isRequired placeholder="e.g. 10.0.2.10" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Destination IP address to probe.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Destination Port" fieldId="tf-dest-port">
                <TextInput id="tf-dest-port" type="number" value={destinationPort} onChange={(_e, v) => setDestinationPort(v)} placeholder="e.g. 80" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Optional destination port. Required for TCP and UDP protocols.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Protocol" fieldId="tf-protocol">
                <FormSelect
                  id="tf-protocol"
                  value={protocol}
                  onChange={(_e, v) => setProtocol(v)}
                >
                  <FormSelectOption value="TCP" label="TCP" />
                  <FormSelectOption value="UDP" label="UDP" />
                  <FormSelectOption value="ICMP" label="ICMP" />
                </FormSelect>
                <FormHelperText>
                  <HelperText><HelperTextItem>IP protocol for the probe packet.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <ActionGroup>
                <Button variant="primary" onClick={handleSubmit} isLoading={submitting} isDisabled={!isValid || submitting}>
                  Create
                </Button>
                <Button variant="link" onClick={() => navigate('/vpc-networking/traceflows')} isDisabled={submitting}>
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

TraceflowCreatePage.displayName = 'TraceflowCreatePage';
export default TraceflowCreatePage;
