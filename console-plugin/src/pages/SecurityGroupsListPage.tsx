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
import { useSecurityGroups } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import CreateSecurityGroupModal from '../components/CreateSecurityGroupModal';

/**
 * Security Groups List Page
 */
const SecurityGroupsListPage: React.FC = () => {
  const { securityGroups, loading } = useSecurityGroups();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const handleCreated = useCallback(() => {
    setIsCreateOpen(false);
    window.location.reload();
  }, []);

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          VPC security groups that control inbound and outbound traffic to VNIs on LocalNet networks.
        </Text>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />} onClick={() => setIsCreateOpen(true)}>
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
                      <Td><Link to={`/vpc-networking/security-groups/${sg.id}`}>{sg.name || '-'}</Link></Td>
                      <Td>{sg.vpc?.name || sg.vpc?.id || '-'}</Td>
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
      <CreateSecurityGroupModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreated={handleCreated}
      />
    </VPCNetworkingShell>
  );
};

export default SecurityGroupsListPage;
