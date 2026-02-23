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
import { useNetworkACLs } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';

/**
 * Network ACLs List Page
 */
const NetworkACLsListPage: React.FC = () => {
  const { networkAcls, loading } = useNetworkACLs();

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">Network ACLs</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />}>
                  Create Network ACL
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !networkAcls?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No network ACLs found</div>
            ) : (
              <Table aria-label="Network ACLs table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>VPC</Th>
                    <Th>Rules</Th>
                    <Th>Status</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {networkAcls.map((acl) => (
                    <Tr key={acl.id || acl.name}>
                      <Td><a href={`/vpc-networking/network-acls/${acl.name}`}>{acl.name || '-'}</a></Td>
                      <Td>{acl.vpc?.name || '-'}</Td>
                      <Td>{acl.rules?.length || 0}</Td>
                      <Td><StatusBadge status={acl.status} /></Td>
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

export default NetworkACLsListPage;
