import React from 'react';
import { Alert } from '@patternfly/react-core';
import { IPAssignmentMode } from '../api/types';

interface IPModeInfoAlertProps {
  mode: IPAssignmentMode;
  description?: string;
}

const defaultDescriptions: Record<IPAssignmentMode, { title: string; body: string }> = {
  static_reserved: {
    title: 'Static Reserved IP',
    body: 'VPC API reserves an IP from the subnet when the VNI is created. The IP and MAC address are injected into the VM via cloud-init. IPs are deterministic and VPC-managed.',
  },
  dhcp: {
    title: 'DHCP',
    body: "OVN's built-in DHCP server assigns IPs from the configured subnet range. IPs are dynamic within the range and managed by OVN.",
  },
  none: {
    title: 'No Automatic IP',
    body: 'Pure L2 connectivity. IP must be configured manually inside the VM or via an external DHCP server.',
  },
};

const IPModeInfoAlert: React.FC<IPModeInfoAlertProps> = ({ mode, description }) => {
  const config = defaultDescriptions[mode];
  if (!config) return null;

  return (
    <Alert variant="info" isInline isPlain title={`IP Assignment: ${config.title}`}>
      {description || config.body}
    </Alert>
  );
};

IPModeInfoAlert.displayName = 'IPModeInfoAlert';
export default IPModeInfoAlert;
