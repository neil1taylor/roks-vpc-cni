import React, { useState, useMemo } from 'react';
import {
  Card,
  CardBody,
  CardTitle,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  Label,
  LabelGroup,
  Spinner,
  Split,
  SplitItem,
  Stack,
  StackItem,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  InfoCircleIcon,
} from '@patternfly/react-icons';
import { Link } from 'react-router-dom-v5-compat';
import { useAlertTimeline } from '../api/hooks';
import { AlertTimelineEntry } from '../api/types';

/**
 * Maps a VPC CRD kind to a console plugin detail page path.
 */
function resourceLink(ref: { kind: string; name: string; namespace: string }): string | null {
  switch (ref.kind) {
    case 'VPCGateway':
      return `/vpc-networking/gateways/${ref.name}?ns=${encodeURIComponent(ref.namespace)}`;
    case 'VPCRouter':
      return `/vpc-networking/routers/${ref.name}?ns=${encodeURIComponent(ref.namespace)}`;
    case 'VPCL2Bridge':
      return `/vpc-networking/l2-bridges/${ref.name}?ns=${encodeURIComponent(ref.namespace)}`;
    case 'VPCVPNGateway':
      return `/vpc-networking/vpn-gateways/${ref.name}?namespace=${encodeURIComponent(ref.namespace)}`;
    case 'VPCDNSPolicy':
      return `/vpc-networking/dns-policies/${ref.name}?namespace=${encodeURIComponent(ref.namespace)}`;
    case 'VPCSubnet':
      return `/vpc-networking/subnets`;
    case 'VirtualNetworkInterface':
      return `/vpc-networking/vnis`;
    case 'VLANAttachment':
      return `/vpc-networking/vlan-attachments`;
    case 'FloatingIP':
      return `/vpc-networking/floating-ips`;
    default:
      return null;
  }
}

const severityIcon = (severity: string) => {
  switch (severity) {
    case 'critical':
      return <ExclamationCircleIcon color="var(--pf-v5-global--danger-color--100)" />;
    case 'warning':
      return <ExclamationTriangleIcon color="var(--pf-v5-global--warning-color--100)" />;
    case 'info':
      return <InfoCircleIcon color="var(--pf-v5-global--info-color--100)" />;
    default:
      return <InfoCircleIcon color="var(--pf-v5-global--info-color--100)" />;
  }
};

const severityColor = (severity: string): 'red' | 'orange' | 'blue' | 'grey' => {
  switch (severity) {
    case 'critical':
      return 'red';
    case 'warning':
      return 'orange';
    case 'info':
      return 'blue';
    default:
      return 'grey';
  }
};

function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHrs = Math.floor(diffMin / 60);
  if (diffHrs < 24) return `${diffHrs}h ago`;
  const diffDays = Math.floor(diffHrs / 24);
  return `${diffDays}d ago`;
}

/**
 * AlertTimelineCard displays a vertical timeline of recent VPC resource alerts.
 * Auto-refreshes every 30 seconds.
 */
const AlertTimelineCard: React.FC = () => {
  const [selectedKinds, setSelectedKinds] = useState<Set<string>>(new Set());
  const { data: alerts, loading, error } = useAlertTimeline('24h');

  // Gather all unique resource kinds for filter chips
  const allKinds = useMemo(() => {
    if (!alerts) return [];
    const kinds = new Set<string>();
    for (const a of alerts) {
      if (a.resourceRef?.kind) kinds.add(a.resourceRef.kind);
    }
    return Array.from(kinds).sort();
  }, [alerts]);

  // Filter alerts by selected kinds (if any)
  const filteredAlerts = useMemo(() => {
    if (!alerts) return [];
    if (selectedKinds.size === 0) return alerts;
    return alerts.filter((a) => a.resourceRef && selectedKinds.has(a.resourceRef.kind));
  }, [alerts, selectedKinds]);

  const toggleKind = (kind: string) => {
    setSelectedKinds((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) {
        next.delete(kind);
      } else {
        next.add(kind);
      }
      return next;
    });
  };

  return (
    <Card>
      <CardTitle>Alert Timeline</CardTitle>
      <CardBody>
        {loading && !alerts && <Spinner size="lg" />}
        {error && (
          <Text component={TextVariants.p} style={{ color: 'var(--pf-v5-global--danger-color--100)' }}>
            Failed to load alerts: {error.message}
          </Text>
        )}
        {!loading && alerts && alerts.length === 0 && (
          <EmptyState>
            <EmptyStateHeader
              titleText="No alerts"
              headingLevel="h4"
              icon={<EmptyStateIcon icon={CheckCircleIcon} color="var(--pf-v5-global--success-color--100)" />}
            />
            <EmptyStateBody>
              No VPC resource alerts in the last 24 hours.
            </EmptyStateBody>
          </EmptyState>
        )}
        {alerts && alerts.length > 0 && (
          <Stack hasGutter>
            {/* Filter chips */}
            {allKinds.length > 1 && (
              <StackItem>
                <LabelGroup categoryName="Filter by resource">
                  {allKinds.map((kind) => (
                    <Label
                      key={kind}
                      color={selectedKinds.has(kind) ? 'blue' : 'grey'}
                      onClick={() => toggleKind(kind)}
                      style={{ cursor: 'pointer' }}
                    >
                      {kind}
                    </Label>
                  ))}
                </LabelGroup>
              </StackItem>
            )}
            {/* Timeline entries */}
            <StackItem>
              <Stack hasGutter>
                {filteredAlerts.map((alert: AlertTimelineEntry, idx: number) => {
                  const link = alert.resourceRef ? resourceLink(alert.resourceRef) : null;
                  return (
                    <StackItem key={idx}>
                      <Split hasGutter style={{ alignItems: 'flex-start' }}>
                        <SplitItem style={{ paddingTop: '2px' }}>
                          {severityIcon(alert.severity)}
                        </SplitItem>
                        <SplitItem isFilled>
                          <div style={{ marginBottom: '4px' }}>
                            <Text component={TextVariants.small} style={{ color: 'var(--pf-v5-global--Color--200)' }}>
                              {formatTimestamp(alert.timestamp)}
                            </Text>
                            {' '}
                            <Label isCompact color={severityColor(alert.severity)}>
                              {alert.severity}
                            </Label>
                            {alert.resourceRef && (
                              <>
                                {' '}
                                <Label isCompact color="blue">{alert.resourceRef.kind}</Label>
                              </>
                            )}
                          </div>
                          <Text component={TextVariants.p} style={{ margin: 0 }}>
                            {alert.message}
                          </Text>
                          {alert.resourceRef && link && (
                            <Link
                              to={link}
                              style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)' }}
                            >
                              {alert.resourceRef.namespace}/{alert.resourceRef.name}
                            </Link>
                          )}
                          {alert.resourceRef && !link && (
                            <Text
                              component={TextVariants.small}
                              style={{ color: 'var(--pf-v5-global--Color--200)' }}
                            >
                              {alert.resourceRef.namespace}/{alert.resourceRef.name}
                            </Text>
                          )}
                        </SplitItem>
                      </Split>
                    </StackItem>
                  );
                })}
              </Stack>
            </StackItem>
          </Stack>
        )}
      </CardBody>
    </Card>
  );
};

AlertTimelineCard.displayName = 'AlertTimelineCard';

export default AlertTimelineCard;
