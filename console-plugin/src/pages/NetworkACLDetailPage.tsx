import React from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
} from '@patternfly/react-core';
import { Link } from 'react-router-dom-v5-compat';
import { useNetworkACL } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { RuleEditor } from '../components/RuleEditor';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * Network ACL Detail Page
 * Displays detailed information about a specific network ACL and its rules
 */
const NetworkACLDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { networkAcl, loading } = useNetworkACL(id || '');

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/network-acls">Network ACLs</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{networkAcl?.name || id}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : networkAcl ? (
          <>
            <Card>
              <CardBody>
                <DescriptionList>
                  <DescriptionListGroup>
                    <DescriptionListTerm>ID</DescriptionListTerm>
                    <DescriptionListDescription>{networkAcl.id}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Name</DescriptionListTerm>
                    <DescriptionListDescription>{networkAcl.name || '-'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>VPC</DescriptionListTerm>
                    <DescriptionListDescription>
                      {networkAcl.vpc.name || networkAcl.vpc.id}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Subnets</DescriptionListTerm>
                    <DescriptionListDescription>
                      {networkAcl.subnets?.length || 0}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Status</DescriptionListTerm>
                    <DescriptionListDescription>
                      <StatusBadge status={networkAcl.status} />
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Created</DescriptionListTerm>
                    <DescriptionListDescription>
                      {formatTimestamp(networkAcl.createdAt)}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>

            <Card style={{ marginTop: '16px' }}>
              <CardTitle>Rules</CardTitle>
              <CardBody>
                <RuleEditor
                  resourceType="network-acl"
                  resourceId={networkAcl.id}
                  rules={networkAcl.rules || []}
                  readOnly={false}
                />
              </CardBody>
            </Card>
          </>
        ) : (
          <Card>
            <CardBody>Network ACL not found</CardBody>
          </Card>
        )}
      </PageSection>
    </VPCNetworkingShell>
  );
};

export default NetworkACLDetailPage;
