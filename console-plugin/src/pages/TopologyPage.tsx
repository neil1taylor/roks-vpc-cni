import React from 'react';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
} from '@patternfly/react-core';
import { TopologyViewer } from '../topology/TopologyViewer';

/**
 * Network Topology Page
 * Visualizes VPC network topology with nodes and edges
 */
const TopologyPage: React.FC = () => {
  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">Network Topology</Title>
      </PageSection>

      <PageSection isFilled style={{ minHeight: '600px' }}>
        <TopologyViewer />
      </PageSection>
    </Page>
  );
};

export default TopologyPage;
