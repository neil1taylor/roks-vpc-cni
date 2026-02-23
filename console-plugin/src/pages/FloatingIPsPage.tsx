import React from 'react';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
  Card,
  CardBody,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
} from '@patternfly/react-core';
import { PlusCircleIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { useFloatingIPs } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';

/**
 * Floating IPs Page
 */
const FloatingIPsPage: React.FC = () => {
  const { floatingIps, loading } = useFloatingIPs();

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">Floating IPs</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />}>
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
                      <Td>{fip.name || '-'}</Td>
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
    </Page>
  );
};

export default FloatingIPsPage;
