import React, { useState, useCallback } from 'react';
import {
  PageSection,
  Card,
  CardBody,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
  FormSelect,
  FormSelectOption,
  FormGroup,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { PlusCircleIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td, ActionsColumn, IAction } from '@patternfly/react-table';
import { useRoutingTables, useRoutes } from '../api/hooks';
import { Route } from '../api/types';
import { apiClient } from '../api/client';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import CreateRouteModal from '../components/CreateRouteModal';

const RoutesPage: React.FC = () => {
  const { routingTables, loading: tablesLoading } = useRoutingTables();
  const [selectedRtId, setSelectedRtId] = useState('');

  // Auto-select the default routing table once loaded
  React.useEffect(() => {
    if (routingTables && routingTables.length > 0 && !selectedRtId) {
      const defaultTable = routingTables.find((rt) => rt.isDefault);
      setSelectedRtId(defaultTable ? defaultTable.id : routingTables[0].id);
    }
  }, [routingTables, selectedRtId]);

  const { routes, loading: routesLoading } = useRoutes(selectedRtId);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const handleCreated = useCallback(() => {
    setIsCreateOpen(false);
    window.location.reload();
  }, []);

  const handleDelete = useCallback(async (route: Route) => {
    if (!selectedRtId) return;
    const resp = await apiClient.deleteRoute(selectedRtId, route.id);
    if (!resp.error) {
      window.location.reload();
    }
  }, [selectedRtId]);

  const getRowActions = (route: Route): IAction[] => {
    if (route.origin !== 'user') return [];
    return [
      {
        title: 'Delete',
        onClick: () => handleDelete(route),
      },
    ];
  };

  const loading = tablesLoading || (selectedRtId && routesLoading);

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          VPC routing table entries that control packet forwarding within the VPC.
        </Text>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <FormGroup label="Routing Table" fieldId="rt-select">
                  <FormSelect
                    id="rt-select"
                    value={selectedRtId}
                    onChange={(_e, v) => setSelectedRtId(v)}
                    isDisabled={tablesLoading || !routingTables?.length}
                  >
                    {(routingTables || []).map((rt) => (
                      <FormSelectOption
                        key={rt.id}
                        value={rt.id}
                        label={`${rt.name}${rt.isDefault ? ' (default)' : ''}`}
                      />
                    ))}
                  </FormSelect>
                </FormGroup>
              </ToolbarItem>
              <ToolbarItem>
                <Button
                  variant={ButtonVariant.primary}
                  icon={<PlusCircleIcon />}
                  onClick={() => setIsCreateOpen(true)}
                  isDisabled={!selectedRtId}
                >
                  Create Route
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !routes?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No routes found</div>
            ) : (
              <Table aria-label="Routes table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>Destination</Th>
                    <Th>Action</Th>
                    <Th>Next Hop</Th>
                    <Th>Zone</Th>
                    <Th>Priority</Th>
                    <Th>Origin</Th>
                    <Th>Status</Th>
                    <Th />
                  </Tr>
                </Thead>
                <Tbody>
                  {routes.map((route) => (
                    <Tr key={route.id}>
                      <Td>{route.name || '-'}</Td>
                      <Td>{route.destination}</Td>
                      <Td>{route.action}</Td>
                      <Td>{route.nextHop || '-'}</Td>
                      <Td>{route.zone}</Td>
                      <Td>{route.priority}</Td>
                      <Td>{route.origin}</Td>
                      <Td>{route.lifecycleState}</Td>
                      <Td isActionCell>
                        {route.origin === 'user' && (
                          <ActionsColumn items={getRowActions(route)} />
                        )}
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      </PageSection>
      {selectedRtId && (
        <CreateRouteModal
          isOpen={isCreateOpen}
          onClose={() => setIsCreateOpen(false)}
          onCreated={handleCreated}
          routingTableId={selectedRtId}
          routingTableName={routingTables?.find((rt) => rt.id === selectedRtId)?.name || ''}
        />
      )}
    </VPCNetworkingShell>
  );
};

export default RoutesPage;
