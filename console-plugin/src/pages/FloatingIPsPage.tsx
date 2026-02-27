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
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { PlusCircleIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { useFloatingIPs } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import CreateFloatingIPModal from '../components/CreateFloatingIPModal';

const FloatingIPsPage: React.FC = () => {
  const { floatingIps, loading } = useFloatingIPs();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const handleCreated = useCallback(() => {
    setIsCreateOpen(false);
    window.location.reload();
  }, []);

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          Public floating IPs that can be attached to VNIs to give VMs internet-reachable addresses.
        </Text>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button
                  variant={ButtonVariant.primary}
                  icon={<PlusCircleIcon />}
                  onClick={() => setIsCreateOpen(true)}
                >
                  Reserve Floating IP
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !floatingIps?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No floating IPs found</div>
            ) : (
              <Table aria-label="Floating IPs table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>Address</Th>
                    <Th>Target</Th>
                    <Th>Zone</Th>
                    <Th>Status</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {floatingIps.map((fip) => (
                    <Tr key={fip.id || fip.name}>
                      <Td><Link to={`/vpc-networking/floating-ips/${fip.id}`}>{fip.name || '-'}</Link></Td>
                      <Td>{fip.address || '-'}</Td>
                      <Td>{fip.target?.name || '-'}</Td>
                      <Td>{fip.zone?.name || '-'}</Td>
                      <Td><StatusBadge status={fip.status} /></Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      </PageSection>
      <CreateFloatingIPModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreated={handleCreated}
      />
    </VPCNetworkingShell>
  );
};

export default FloatingIPsPage;
