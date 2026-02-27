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
import { useNetworkACLs } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import CreateNetworkACLModal from '../components/CreateNetworkACLModal';

/**
 * Network ACLs List Page
 */
const NetworkACLsListPage: React.FC = () => {
  const { networkAcls, loading } = useNetworkACLs();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const handleCreated = useCallback(() => {
    setIsCreateOpen(false);
    window.location.reload();
  }, []);

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          VPC network access control lists that provide subnet-level traffic filtering rules.
        </Text>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />} onClick={() => setIsCreateOpen(true)}>
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
                      <Td><Link to={`/vpc-networking/network-acls/${acl.id}`}>{acl.name || '-'}</Link></Td>
                      <Td>{acl.vpc?.name || acl.vpc?.id || '-'}</Td>
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
      <CreateNetworkACLModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreated={handleCreated}
      />
    </VPCNetworkingShell>
  );
};

export default NetworkACLsListPage;
