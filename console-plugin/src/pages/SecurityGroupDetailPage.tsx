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
import { useSecurityGroup } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { RuleEditor } from '../components/RuleEditor';
import { formatTimestamp } from '../utils/formatters';

/**
 * Security Group Detail Page
 * Displays detailed information about a specific security group and its rules
 */
const SecurityGroupDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { securityGroup, loading } = useSecurityGroup(name || '');

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem href="/vpc-networking/security-groups">Security Groups</BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Security Group: {name}</Title>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : securityGroup ? (
          <>
            <Card>
              <CardBody>
                <DescriptionList>
                  <DescriptionListGroup>
                    <DescriptionListTerm>ID</DescriptionListTerm>
                    <DescriptionListDescription>{securityGroup.id}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Name</DescriptionListTerm>
                    <DescriptionListDescription>{securityGroup.name || '-'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Description</DescriptionListTerm>
                    <DescriptionListDescription>
                      {securityGroup.description || '-'}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>VPC</DescriptionListTerm>
                    <DescriptionListDescription>
                      {securityGroup.vpc.name || securityGroup.vpc.id}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Status</DescriptionListTerm>
                    <DescriptionListDescription>
                      <StatusBadge status={securityGroup.status} />
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Created</DescriptionListTerm>
                    <DescriptionListDescription>
                      {formatTimestamp(securityGroup.createdAt)}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>

            <Card style={{ marginTop: '16px' }}>
              <CardTitle>Rules</CardTitle>
              <CardBody>
                <RuleEditor
                  resourceType="security-group"
                  resourceId={securityGroup.id}
                  rules={securityGroup.rules || []}
                  readOnly={false}
                />
              </CardBody>
            </Card>
          </>
        ) : (
          <Card>
            <CardBody>Security group not found</CardBody>
          </Card>
        )}
      </PageSection>
    </Page>
  );
};

export default SecurityGroupDetailPage;
