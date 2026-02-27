import React from 'react';
import {
  PageSection,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { TopologyViewer } from '../topology/TopologyViewer';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * Network Topology Page
 * Visualizes VPC network topology with nodes and edges
 */
const TopologyPage: React.FC = () => {
  return (
    <VPCNetworkingShell>
      <PageSection padding={{ default: 'padding' }}>
        <Text component={TextVariants.p} style={{ color: 'var(--pf-v5-global--Color--200)' }}>
          Visual map of VPC resources and their relationships — subnets, VNIs, security groups, and networks.
        </Text>
      </PageSection>
      <PageSection isFilled padding={{ default: 'noPadding' }}>
        <TopologyViewer />
      </PageSection>
    </VPCNetworkingShell>
  );
};

export default TopologyPage;
