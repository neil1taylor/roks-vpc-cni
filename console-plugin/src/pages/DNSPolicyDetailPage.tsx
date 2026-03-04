import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Breadcrumb,
  BreadcrumbItem,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Spinner,
  Button,
  Alert,
  Split,
  SplitItem,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  Label,
  List,
  ListItem,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useDNSPolicy } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const DNSPolicyDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { dnsPolicy, loading } = useDNSPolicy(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteDNSPolicy(name, ns);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/dns-policies');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/dns-policies">DNS Policies</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{dnsPolicy?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : dnsPolicy ? (
          <>
            {actionError && (
              <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
            )}

            <Grid hasGutter lg={6}>
              {/* Card 1: Overview */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>
                    <Split hasGutter>
                      <SplitItem isFilled>Overview</SplitItem>
                      <SplitItem>
                        <Button
                          variant="danger"
                          onClick={() => { setActionError(null); setIsDeleteModalOpen(true); }}
                          isDisabled={actionLoading}
                        >
                          Delete
                        </Button>
                      </SplitItem>
                    </Split>
                  </CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Name</DescriptionListTerm>
                        <DescriptionListDescription>{dnsPolicy.name || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Namespace</DescriptionListTerm>
                        <DescriptionListDescription>{dnsPolicy.namespace || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Router</DescriptionListTerm>
                        <DescriptionListDescription>
                          {dnsPolicy.routerRef ? (
                            <Link to={`/vpc-networking/routers/${dnsPolicy.routerRef}${ns ? `?ns=${encodeURIComponent(ns)}` : ''}`}>
                              {dnsPolicy.routerRef}
                            </Link>
                          ) : '-'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Phase</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={dnsPolicy.phase} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Sync Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={dnsPolicy.syncStatus} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>ConfigMap</DescriptionListTerm>
                        <DescriptionListDescription>{dnsPolicy.configMapName || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Created</DescriptionListTerm>
                        <DescriptionListDescription>
                          {formatTimestamp(dnsPolicy.createdAt)}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      {dnsPolicy.message && (
                        <DescriptionListGroup>
                          <DescriptionListTerm>Message</DescriptionListTerm>
                          <DescriptionListDescription>{dnsPolicy.message}</DescriptionListDescription>
                        </DescriptionListGroup>
                      )}
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 2: Upstream Servers */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Upstream DNS Servers</CardTitle>
                  <CardBody>
                    {dnsPolicy.upstreamServers && dnsPolicy.upstreamServers.length > 0 ? (
                      <List>
                        {dnsPolicy.upstreamServers.map((server, idx) => (
                          <ListItem key={idx}>
                            <code>{server}</code>
                          </ListItem>
                        ))}
                      </List>
                    ) : (
                      <span style={{ color: 'var(--pf-v5-global--Color--200)' }}>No upstream servers configured (using defaults)</span>
                    )}
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 3: Filtering Config */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>DNS Filtering</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={dnsPolicy.filteringEnabled ? 'green' : 'grey'}>
                            {dnsPolicy.filteringEnabled ? 'Enabled' : 'Disabled'}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Rules Loaded</DescriptionListTerm>
                        <DescriptionListDescription>
                          {dnsPolicy.filterRulesLoaded > 0 ? (
                            <Label color="blue">{dnsPolicy.filterRulesLoaded.toLocaleString()}</Label>
                          ) : (
                            '0'
                          )}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 4: Local DNS */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Local DNS</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={dnsPolicy.localDNSEnabled ? 'green' : 'grey'}>
                            {dnsPolicy.localDNSEnabled ? 'Enabled' : 'Disabled'}
                          </Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      {dnsPolicy.localDNSDomain && (
                        <DescriptionListGroup>
                          <DescriptionListTerm>Domain</DescriptionListTerm>
                          <DescriptionListDescription>
                            <code>{dnsPolicy.localDNSDomain}</code>
                          </DescriptionListDescription>
                        </DescriptionListGroup>
                      )}
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>
            </Grid>
          </>
        ) : (
          <EmptyState>
            <EmptyStateHeader
              titleText="DNS Policy not found"
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              The DNS policy &quot;{name}&quot; could not be found. It may have been deleted or the name is incorrect.
            </EmptyStateBody>
          </EmptyState>
        )}
      </PageSection>

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete DNS Policy"
        message={`Deleting DNS policy "${dnsPolicy?.name}" will remove the AdGuard Home sidecar from the associated router pod. DNS resolution will revert to default behavior. This action cannot be undone.`}
        resourceName={dnsPolicy?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

DNSPolicyDetailPage.displayName = 'DNSPolicyDetailPage';
export default DNSPolicyDetailPage;
