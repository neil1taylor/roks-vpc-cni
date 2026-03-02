import React from 'react';
import {
  Card,
  CardBody,
  CardTitle,
  DescriptionList,
  DescriptionListDescription,
  DescriptionListGroup,
  DescriptionListTerm,
  Flex,
  FlexItem,
  Label,
  Skeleton,
  Split,
  SplitItem,
} from '@patternfly/react-core';
import { CheckCircleIcon, ExclamationCircleIcon, ExclamationTriangleIcon } from '@patternfly/react-icons';
import { RouterHealthSummary } from '../../api/types';
import { formatBytesPerSec, formatUptime, formatPercentage } from '../../utils/formatters';

interface RouterHealthCardProps {
  health: RouterHealthSummary | null;
  loading: boolean;
}

const statusIcon = (status: string) => {
  switch (status) {
    case 'healthy':
      return <CheckCircleIcon color="var(--pf-v5-global--success-color--100)" />;
    case 'degraded':
      return <ExclamationTriangleIcon color="var(--pf-v5-global--warning-color--100)" />;
    default:
      return <ExclamationCircleIcon color="var(--pf-v5-global--danger-color--100)" />;
  }
};

const RouterHealthCard: React.FC<RouterHealthCardProps> = ({ health, loading }) => {
  if (loading) {
    return (
      <Card isCompact>
        <CardTitle>Health</CardTitle>
        <CardBody>
          <Skeleton width="100%" height="80px" />
        </CardBody>
      </Card>
    );
  }

  if (!health) {
    return (
      <Card isCompact>
        <CardTitle>Health</CardTitle>
        <CardBody>No health data available</CardBody>
      </Card>
    );
  }

  const totalRx = health.interfaces.reduce((s, i) => s + i.rxBps, 0);
  const totalTx = health.interfaces.reduce((s, i) => s + i.txBps, 0);

  return (
    <Card isCompact>
      <CardTitle>
        <Split hasGutter>
          <SplitItem>{statusIcon(health.status)}</SplitItem>
          <SplitItem>Router Health</SplitItem>
          <SplitItem isFilled />
          <SplitItem>
            <Label color={health.status === 'healthy' ? 'green' : health.status === 'degraded' ? 'gold' : 'red'}>
              {health.status}
            </Label>
          </SplitItem>
        </Split>
      </CardTitle>
      <CardBody>
        <DescriptionList isHorizontal isCompact>
          <DescriptionListGroup>
            <DescriptionListTerm>Uptime</DescriptionListTerm>
            <DescriptionListDescription>{formatUptime(health.uptimeSeconds)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Throughput</DescriptionListTerm>
            <DescriptionListDescription>
              RX {formatBytesPerSec(totalRx)} / TX {formatBytesPerSec(totalTx)}
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Conntrack</DescriptionListTerm>
            <DescriptionListDescription>{formatPercentage(health.conntrackPercentage, 1)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Processes</DescriptionListTerm>
            <DescriptionListDescription>
              <Flex>
                {Object.entries(health.processes).map(([name, running]) => (
                  <FlexItem key={name}>
                    <Label color={running ? 'green' : 'red'} isCompact>
                      {name}
                    </Label>
                  </FlexItem>
                ))}
              </Flex>
            </DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </CardBody>
    </Card>
  );
};

export default RouterHealthCard;
