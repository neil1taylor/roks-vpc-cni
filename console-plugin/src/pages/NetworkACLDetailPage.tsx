import React from 'react';
import { useParams } from 'react-router-dom';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
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
import { useNetworkACL } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { RuleEditor } from '../components/RuleEditor';
import { formatTimestamp } from '../utils/formatters';

/**
 * Network ACL Detail Page
 * Displays detailed information about a specific network ACL and its rules
 */
const NetworkACLDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { networkAcl, loading } = useNetworkACL(name || '');

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem href="/vpc-networking/network-acls">Network ACLs</BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Network ACL: {name}</Title>
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
    </Page>
  );
};

export default NetworkACLDetailPage;
