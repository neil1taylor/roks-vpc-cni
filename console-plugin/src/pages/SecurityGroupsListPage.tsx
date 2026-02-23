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
import { useSecurityGroups } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';

/**
 * Security Groups List Page
 */
const SecurityGroupsListPage: React.FC = () => {
  const { securityGroups, loading } = useSecurityGroups();

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">Security Groups</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />}>
                  Create Security Group
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !securityGroups?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No security groups found</div>
            ) : (
              <Table aria-label="Security Groups table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>VPC</Th>
                    <Th>Rules</Th>
                    <Th>Status</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {securityGroups.map((sg) => (
                    <Tr key={sg.id || sg.name}>
                      <Td><a href={`/vpc-networking/security-groups/${sg.name}`}>{sg.name || '-'}</a></Td>
                      <Td>{sg.vpc?.name || '-'}</Td>
                      <Td>{sg.rules?.length || 0}</Td>
                      <Td><StatusBadge status={sg.status} /></Td>
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

export default SecurityGroupsListPage;
