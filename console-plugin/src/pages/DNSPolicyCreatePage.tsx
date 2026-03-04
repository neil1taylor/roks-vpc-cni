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
  Title,
  Text,
  TextVariants,
  FormSelect,
  FormSelectOption,
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { apiClient } from '../api/client';
import { CreateDNSPolicyRequest } from '../api/types';
import { useRouters } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const DNSPolicyCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const { routers, loading: routersLoading } = useRouters();

  // Core fields
  const [name, setName] = useState('');
  const [router, setRouter] = useState('');

  // Upstream DNS
  const [upstreamServers, setUpstreamServers] = useState<string[]>(['']);

  // Filtering
  const [filteringEnabled, setFilteringEnabled] = useState(false);
  const [blocklists, setBlocklists] = useState<string[]>(['']);
  const [allowlist, setAllowlist] = useState('');
  const [denylist, setDenylist] = useState('');

  // Local DNS
  const [localDNSEnabled, setLocalDNSEnabled] = useState(false);
  const [localDNSDomain, setLocalDNSDomain] = useState('');

  // Image
  const [image, setImage] = useState('');

  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const isValid = name.trim() !== '' && router !== '';

  const handleSubmit = async () => {
    if (!isValid) return;
    setSubmitting(true);
    setSubmitError('');

    const req: CreateDNSPolicyRequest = {
      name: name.trim(),
      routerRef: router,
    };

    // Upstream servers
    const servers = upstreamServers.map((s) => s.trim()).filter(Boolean);
    if (servers.length > 0) {
      req.upstream = { servers };
    }

    // Filtering
    if (filteringEnabled) {
      const bl = blocklists.map((b) => b.trim()).filter(Boolean);
      const al = allowlist.split(',').map((a) => a.trim()).filter(Boolean);
      const dl = denylist.split(',').map((d) => d.trim()).filter(Boolean);
      req.filtering = {
        enabled: true,
        blocklists: bl.length > 0 ? bl : undefined,
        allowlist: al.length > 0 ? al : undefined,
        denylist: dl.length > 0 ? dl : undefined,
      };
    }

    // Local DNS
    if (localDNSEnabled) {
      req.localDNS = {
        enabled: true,
        domain: localDNSDomain.trim() || undefined,
      };
    }

    // Image override
    if (image.trim()) {
      req.image = image.trim();
    }

    const resp = await apiClient.createDNSPolicy(req);
    if (resp.error) {
      setSubmitError(resp.error.message);
      setSubmitting(false);
    } else {
      navigate('/vpc-networking/dns-policies');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/dns-policies">DNS Policies</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '16px' }}>Create VPCDNSPolicy</Title>
        <Text component={TextVariants.p} style={{ marginTop: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
          A VPCDNSPolicy configures DNS resolution for a VPCRouter using an AdGuard Home sidecar.
          It supports encrypted upstream DNS (DoH/DoT), ad/tracker filtering, and local DNS resolution.
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
              <FormGroup label="Name" isRequired fieldId="dns-name">
                <TextInput id="dns-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
                <FormHelperText>
                  <HelperText><HelperTextItem>Kubernetes resource name. Use lowercase letters, numbers, and hyphens.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Router */}
              <FormGroup label="Router" isRequired fieldId="dns-router">
                <FormSelect
                  id="dns-router"
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
                  <HelperText><HelperTextItem>VPCRouter to attach the DNS policy to. An AdGuard Home sidecar will be injected into this router pod.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {/* Upstream DNS Servers */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Upstream DNS Servers</Title>
              <Text component={TextVariants.small} style={{ marginBottom: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
                Configure upstream DNS servers. Use https:// for DNS-over-HTTPS, tls:// for DNS-over-TLS, or plain IP for standard DNS.
              </Text>

              {upstreamServers.map((server, idx) => (
                <FormGroup key={idx} label={`Server ${idx + 1}`} fieldId={`upstream-${idx}`}>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <TextInput
                      id={`upstream-${idx}`}
                      value={server}
                      onChange={(_e, v) => {
                        setUpstreamServers((prev) => {
                          const updated = [...prev];
                          updated[idx] = v;
                          return updated;
                        });
                      }}
                      placeholder="e.g. https://dns.cloudflare.com/dns-query"
                      style={{ flex: 1 }}
                    />
                    {upstreamServers.length > 1 && (
                      <Button variant="plain" isDanger onClick={() => setUpstreamServers((prev) => prev.filter((_, i) => i !== idx))}>
                        Remove
                      </Button>
                    )}
                  </div>
                </FormGroup>
              ))}
              <Button variant="secondary" onClick={() => setUpstreamServers((prev) => [...prev, ''])} style={{ marginBottom: '16px' }}>
                Add Server
              </Button>

              {/* Filtering */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>DNS Filtering</Title>

              <FormGroup label="Enable Filtering" fieldId="dns-filtering">
                <Switch
                  id="dns-filtering"
                  label="Enabled"
                  labelOff="Disabled"
                  isChecked={filteringEnabled}
                  onChange={(_e, checked) => setFilteringEnabled(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Block ads, trackers, and malware domains using curated blocklists</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {filteringEnabled && (
                <>
                  <Text component={TextVariants.small} style={{ marginBottom: '8px', color: 'var(--pf-v5-global--Color--200)' }}>
                    Add blocklist URLs (hosts format). Each URL provides a list of domains to block.
                  </Text>
                  {blocklists.map((url, idx) => (
                    <FormGroup key={idx} label={`Blocklist ${idx + 1}`} fieldId={`blocklist-${idx}`}>
                      <div style={{ display: 'flex', gap: '8px' }}>
                        <TextInput
                          id={`blocklist-${idx}`}
                          value={url}
                          onChange={(_e, v) => {
                            setBlocklists((prev) => {
                              const updated = [...prev];
                              updated[idx] = v;
                              return updated;
                            });
                          }}
                          placeholder="e.g. https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt"
                          style={{ flex: 1 }}
                        />
                        {blocklists.length > 1 && (
                          <Button variant="plain" isDanger onClick={() => setBlocklists((prev) => prev.filter((_, i) => i !== idx))}>
                            Remove
                          </Button>
                        )}
                      </div>
                    </FormGroup>
                  ))}
                  <Button variant="secondary" onClick={() => setBlocklists((prev) => [...prev, ''])} style={{ marginBottom: '16px' }}>
                    Add Blocklist
                  </Button>

                  <FormGroup label="Allowlist" fieldId="dns-allowlist">
                    <TextInput id="dns-allowlist" value={allowlist} onChange={(_e, v) => setAllowlist(v)} placeholder="e.g. example.com, *.cdn.example.com" />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Comma-separated domain patterns to always allow (overrides blocklists)</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>

                  <FormGroup label="Denylist" fieldId="dns-denylist">
                    <TextInput id="dns-denylist" value={denylist} onChange={(_e, v) => setDenylist(v)} placeholder="e.g. malware.example.com" />
                    <FormHelperText>
                      <HelperText><HelperTextItem>Comma-separated domain patterns to always block (in addition to blocklists)</HelperTextItem></HelperText>
                    </FormHelperText>
                  </FormGroup>
                </>
              )}

              {/* Local DNS */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Local DNS</Title>

              <FormGroup label="Enable Local DNS" fieldId="dns-local">
                <Switch
                  id="dns-local"
                  label="Enabled"
                  labelOff="Disabled"
                  isChecked={localDNSEnabled}
                  onChange={(_e, checked) => setLocalDNSEnabled(checked)}
                />
                <FormHelperText>
                  <HelperText><HelperTextItem>Resolve DHCP hostnames under a local domain suffix (e.g. myvm.vm.local)</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              {localDNSEnabled && (
                <FormGroup label="Domain" fieldId="dns-local-domain">
                  <TextInput id="dns-local-domain" value={localDNSDomain} onChange={(_e, v) => setLocalDNSDomain(v)} placeholder="e.g. vm.local" />
                  <FormHelperText>
                    <HelperText><HelperTextItem>Local domain suffix for DHCP hostname resolution</HelperTextItem></HelperText>
                  </FormHelperText>
                </FormGroup>
              )}

              {/* Advanced: Image Override */}
              <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>Advanced</Title>

              <FormGroup label="Container Image" fieldId="dns-image">
                <TextInput id="dns-image" value={image} onChange={(_e, v) => setImage(v)} placeholder="e.g. adguard/adguardhome:v0.107.46" />
                <FormHelperText>
                  <HelperText><HelperTextItem>Override the default AdGuard Home container image. Leave empty for the operator default.</HelperTextItem></HelperText>
                </FormHelperText>
              </FormGroup>

              <ActionGroup>
                <Button variant="primary" onClick={handleSubmit} isLoading={submitting} isDisabled={!isValid || submitting}>
                  Create
                </Button>
                <Button variant="link" onClick={() => navigate('/vpc-networking/dns-policies')} isDisabled={submitting}>
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

DNSPolicyCreatePage.displayName = 'DNSPolicyCreatePage';
export default DNSPolicyCreatePage;
