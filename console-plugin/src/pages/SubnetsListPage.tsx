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
import { useSubnets } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import { formatRelativeTime } from '../utils/formatters';

/**
 * Subnets List Page
 * Displays list of VPC subnets with CRUD operations
 */
const SubnetsListPage: React.FC = () => {
  const { subnets, loading } = useSubnets();

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">VPC Subnets</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button
                  variant={ButtonVariant.primary}
                  icon={<PlusCircleIcon />}
                >
                  Create Subnet
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>

          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !subnets?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No subnets found</div>
            ) : (
              <Table aria-label="Subnets table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>VPC</Th>
                    <Th>Zone</Th>
                    <Th>CIDR</Th>
                    <Th>Status</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {subnets.map((subnet) => (
                    <Tr key={subnet.id || subnet.name}>
                      <Td><a href={`/vpc-networking/subnets/${subnet.name}`}>{subnet.name || '-'}</a></Td>
                      <Td>{subnet.vpc?.name || '-'}</Td>
                      <Td>{subnet.zone?.name || '-'}</Td>
                      <Td>{subnet.ipv4CidrBlock || '-'}</Td>
                      <Td><StatusBadge status={subnet.status} /></Td>
                      <Td>{formatRelativeTime(subnet.createdAt)}</Td>
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

SubnetsListPage.displayName = 'SubnetsListPage';

export default SubnetsListPage;
