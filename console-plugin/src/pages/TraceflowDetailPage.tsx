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
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { CubesIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useTraceflow } from '../api/hooks';
import { TraceflowHop } from '../api/types';
import { apiClient } from '../api/client';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

type PhaseBadgeColor = 'grey' | 'blue' | 'green' | 'red';
type ResultBadgeColor = 'green' | 'red' | 'orange' | 'grey';

const phaseColors: Record<string, PhaseBadgeColor> = {
  pending: 'grey',
  running: 'blue',
  completed: 'green',
  failed: 'red',
};

const resultColors: Record<string, ResultBadgeColor> = {
  reachable: 'green',
  unreachable: 'red',
  filtered: 'orange',
  timeout: 'grey',
};

const TraceflowDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { data: traceflow, loading } = useTraceflow(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteTraceflow(name, ns);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/traceflows');
    }
  };

  const isRunning = traceflow?.phase?.toLowerCase() === 'running';

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/traceflows">Traceflows</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{traceflow?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading && !traceflow ? (
          <Spinner size="lg" />
        ) : traceflow ? (
          <>
            {actionError && (
              <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
            )}

            {/* Summary Card */}
            <Grid hasGutter lg={6}>
              <GridItem span={12}>
                <Card>
                  <CardTitle>
                    <Split hasGutter>
                      <SplitItem isFilled>Summary</SplitItem>
                      <SplitItem>
                        {isRunning && <Spinner size="md" style={{ marginRight: '8px' }} />}
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
                    <Grid hasGutter md={6} lg={4}>
                      <GridItem>
                        <DescriptionList>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Name</DescriptionListTerm>
                            <DescriptionListDescription>{traceflow.name}</DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Namespace</DescriptionListTerm>
                            <DescriptionListDescription>{traceflow.namespace}</DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Phase</DescriptionListTerm>
                            <DescriptionListDescription>
                              <Label color={phaseColors[traceflow.phase?.toLowerCase()] || 'grey'} variant="outline">
                                {traceflow.phase || 'Unknown'}
                              </Label>
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Result</DescriptionListTerm>
                            <DescriptionListDescription>
                              {traceflow.result ? (
                                <Label color={resultColors[traceflow.result?.toLowerCase()] || 'grey'} variant="outline">
                                  {traceflow.result}
                                </Label>
                              ) : '-'}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                        </DescriptionList>
                      </GridItem>
                      <GridItem>
                        <DescriptionList>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Source IP</DescriptionListTerm>
                            <DescriptionListDescription>{traceflow.sourceIP || '-'}</DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Destination IP</DescriptionListTerm>
                            <DescriptionListDescription>
                              {traceflow.destinationIP}
                              {traceflow.destinationPort ? `:${traceflow.destinationPort}` : ''}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Protocol</DescriptionListTerm>
                            <DescriptionListDescription>{traceflow.protocol || '-'}</DescriptionListDescription>
                          </DescriptionListGroup>
                        </DescriptionList>
                      </GridItem>
                      <GridItem>
                        <DescriptionList>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Router</DescriptionListTerm>
                            <DescriptionListDescription>
                              {traceflow.router ? (
                                <Link to={`/vpc-networking/routers/${traceflow.router}${ns ? `?ns=${encodeURIComponent(ns)}` : ''}`}>
                                  {traceflow.router}
                                </Link>
                              ) : '-'}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Total Latency</DescriptionListTerm>
                            <DescriptionListDescription>
                              {traceflow.totalLatencyMs !== undefined && traceflow.totalLatencyMs !== null
                                ? `${traceflow.totalLatencyMs} ms`
                                : '-'}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                          <DescriptionListGroup>
                            <DescriptionListTerm>Created</DescriptionListTerm>
                            <DescriptionListDescription>{formatTimestamp(traceflow.createdAt)}</DescriptionListDescription>
                          </DescriptionListGroup>
                        </DescriptionList>
                      </GridItem>
                    </Grid>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Hops Section */}
              <GridItem span={12}>
                <Card>
                  <CardTitle>
                    Hops
                    {isRunning && <Spinner size="md" style={{ marginLeft: '8px' }} />}
                  </CardTitle>
                  <CardBody>
                    {isRunning && (!traceflow.hops || traceflow.hops.length === 0) ? (
                      <div style={{ textAlign: 'center', padding: '2rem' }}>
                        <Spinner size="lg" />
                        <div style={{ marginTop: '1rem', color: 'var(--pf-v5-global--Color--200)' }}>
                          Traceflow is running. Waiting for hop results...
                        </div>
                      </div>
                    ) : traceflow.hops && traceflow.hops.length > 0 ? (
                      <div>
                        {traceflow.hops
                          .sort((a, b) => a.hop - b.hop)
                          .map((hop: TraceflowHop) => (
                            <Card key={hop.hop} isFlat isCompact style={{ marginBottom: '12px' }}>
                              <CardBody>
                                <Grid hasGutter md={6} lg={3}>
                                  <GridItem>
                                    <DescriptionList isCompact>
                                      <DescriptionListGroup>
                                        <DescriptionListTerm>Hop</DescriptionListTerm>
                                        <DescriptionListDescription>
                                          <Label color="blue" variant="outline">#{hop.hop}</Label>
                                        </DescriptionListDescription>
                                      </DescriptionListGroup>
                                    </DescriptionList>
                                  </GridItem>
                                  <GridItem>
                                    <DescriptionList isCompact>
                                      <DescriptionListGroup>
                                        <DescriptionListTerm>Node</DescriptionListTerm>
                                        <DescriptionListDescription>{hop.node || '-'}</DescriptionListDescription>
                                      </DescriptionListGroup>
                                    </DescriptionList>
                                  </GridItem>
                                  <GridItem>
                                    <DescriptionList isCompact>
                                      <DescriptionListGroup>
                                        <DescriptionListTerm>Component</DescriptionListTerm>
                                        <DescriptionListDescription>{hop.component || '-'}</DescriptionListDescription>
                                      </DescriptionListGroup>
                                    </DescriptionList>
                                  </GridItem>
                                  <GridItem>
                                    <DescriptionList isCompact>
                                      <DescriptionListGroup>
                                        <DescriptionListTerm>Action</DescriptionListTerm>
                                        <DescriptionListDescription>
                                          <Label
                                            color={
                                              hop.action?.toLowerCase() === 'forward' ? 'green'
                                                : hop.action?.toLowerCase() === 'drop' ? 'red'
                                                  : hop.action?.toLowerCase() === 'nat' ? 'blue'
                                                    : 'grey'
                                            }
                                            variant="outline"
                                          >
                                            {hop.action || '-'}
                                          </Label>
                                        </DescriptionListDescription>
                                      </DescriptionListGroup>
                                    </DescriptionList>
                                  </GridItem>
                                  <GridItem>
                                    <DescriptionList isCompact>
                                      <DescriptionListGroup>
                                        <DescriptionListTerm>Latency</DescriptionListTerm>
                                        <DescriptionListDescription>
                                          {hop.latencyMs !== undefined && hop.latencyMs !== null
                                            ? `${hop.latencyMs} ms`
                                            : '-'}
                                        </DescriptionListDescription>
                                      </DescriptionListGroup>
                                    </DescriptionList>
                                  </GridItem>
                                </Grid>

                                {/* NFT Hits table */}
                                {hop.nftHits && hop.nftHits.length > 0 && (
                                  <div style={{ marginTop: '12px' }}>
                                    <strong style={{ fontSize: '0.85rem', color: 'var(--pf-v5-global--Color--200)' }}>NFTables Hits</strong>
                                    <Table aria-label={`NFT hits for hop ${hop.hop}`} variant="compact" style={{ marginTop: '4px' }}>
                                      <Thead>
                                        <Tr>
                                          <Th>Rule</Th>
                                          <Th>Chain</Th>
                                          <Th>Packets</Th>
                                        </Tr>
                                      </Thead>
                                      <Tbody>
                                        {hop.nftHits.map((hit, idx) => (
                                          <Tr key={idx}>
                                            <Td><code>{hit.rule}</code></Td>
                                            <Td>{hit.chain}</Td>
                                            <Td>{hit.packets}</Td>
                                          </Tr>
                                        ))}
                                      </Tbody>
                                    </Table>
                                  </div>
                                )}
                              </CardBody>
                            </Card>
                          ))}
                      </div>
                    ) : (
                      <div style={{ color: 'var(--pf-v5-global--Color--200)', textAlign: 'center', padding: '1rem' }}>
                        No hop data available.
                      </div>
                    )}
                  </CardBody>
                </Card>
              </GridItem>
            </Grid>
          </>
        ) : (
          <EmptyState>
            <EmptyStateHeader
              titleText="Traceflow not found"
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              The traceflow &quot;{name}&quot; could not be found. It may have been deleted or the name is incorrect.
            </EmptyStateBody>
          </EmptyState>
        )}
      </PageSection>

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete Traceflow"
        message={`Deleting traceflow "${traceflow?.name}" will remove the diagnostic result. This action cannot be undone.`}
        resourceName={traceflow?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

TraceflowDetailPage.displayName = 'TraceflowDetailPage';
export default TraceflowDetailPage;
