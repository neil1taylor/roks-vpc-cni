import React from 'react';
import { Label, LabelProps } from '@patternfly/react-core';

export interface StatusBadgeProps extends Omit<LabelProps, 'children'> {
  status?: string;
  showText?: boolean;
}

/**
 * StatusBadge component for displaying resource status
 * Shows visual indication with color coding
 */
export const StatusBadge: React.FC<StatusBadgeProps> = ({
  status = 'unknown',
  showText = true,
  ...props
}) => {
  const statusLower = status?.toLowerCase() || 'unknown';

  // Determine color and display text based on status
  const getStatusColor = (): LabelProps['color'] => {
    switch (statusLower) {
      case 'available':
      case 'active':
      case 'running':
      case 'synced':
      case 'connected':
      case 'ready':
        return 'green';
      case 'pending':
      case 'provisioning':
      case 'updating':
      case 'syncing':
        return 'blue';
      case 'failed':
      case 'error':
      case 'disconnected':
      case 'degraded':
        return 'red';
      case 'deleting':
      case 'terminating':
        return 'orange';
      case 'suspended':
      case 'paused':
        return 'cyan';
      default:
        return 'grey';
    }
  };

  const getStatusDisplay = (): string => {
    const displayMap: Record<string, string> = {
      available: 'Available',
      active: 'Active',
      running: 'Running',
      synced: 'Synced',
      connected: 'Connected',
      ready: 'Ready',
      pending: 'Pending',
      provisioning: 'Provisioning',
      updating: 'Updating',
      syncing: 'Syncing',
      failed: 'Failed',
      error: 'Error',
      disconnected: 'Disconnected',
      degraded: 'Degraded',
      deleting: 'Deleting',
      terminating: 'Terminating',
      suspended: 'Suspended',
      paused: 'Paused',
      unknown: 'Unknown',
    };

    return displayMap[statusLower] || status || 'Unknown';
  };

  return (
    <Label
      color={getStatusColor()}
      variant="outline"
      {...props}
    >
      {showText ? getStatusDisplay() : null}
    </Label>
  );
};

StatusBadge.displayName = 'StatusBadge';

export default StatusBadge;
