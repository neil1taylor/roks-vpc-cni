import React, { useMemo, useState } from 'react';
import {
  ExpandableSection,
  Label,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { CheckCircleIcon, InProgressIcon, BanIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';

type SupportStatus = 'supported' | 'coming-soon' | 'na';

interface NetworkTypeRow {
  crd: string;
  topology: string;
  allowedRoles: string;
  ipam: string;
  status: SupportStatus;
  notes: string;
}

const rows: NetworkTypeRow[] = [
  { crd: 'CUDN', topology: 'Localnet', allowedRoles: 'Secondary', ipam: 'Disabled (VPC manages IPs)', status: 'supported', notes: 'Requires VPC config' },
  { crd: 'CUDN', topology: 'Layer2', allowedRoles: 'Secondary', ipam: 'DHCP / Persistent / Disabled', status: 'supported', notes: 'No VPC resources needed' },
  { crd: 'CUDN', topology: 'Layer2', allowedRoles: 'Primary', ipam: 'Persistent (subnets required)', status: 'supported', notes: 'Replaces default pod network' },
  { crd: 'CUDN', topology: 'Layer3', allowedRoles: 'Primary, Secondary', ipam: '\u2014', status: 'coming-soon', notes: '' },
  { crd: 'UDN', topology: 'Layer2', allowedRoles: 'Secondary', ipam: 'DHCP / Persistent / Disabled', status: 'supported', notes: 'Namespace-scoped' },
  { crd: 'UDN', topology: 'Layer3', allowedRoles: 'Primary, Secondary', ipam: '\u2014', status: 'coming-soon', notes: '' },
  { crd: 'UDN', topology: 'Localnet', allowedRoles: '\u2014', ipam: '\u2014', status: 'na', notes: 'UDN has no localnet field' },
];

const statusLabel = (status: SupportStatus) => {
  switch (status) {
    case 'supported':
      return <Label color="green" icon={<CheckCircleIcon />}>Supported</Label>;
    case 'coming-soon':
      return <Label color="orange" icon={<InProgressIcon />}>Coming soon</Label>;
    case 'na':
      return <Label color="grey" icon={<BanIcon />}>N/A</Label>;
  }
};

interface Props {
  isROKS?: boolean;
}

const NetworkTypesInfoPanel: React.FC<Props> = ({ isROKS }) => {
  const [isExpanded, setIsExpanded] = useState(false);

  const effectiveRows = useMemo(() =>
    rows.map((row) => {
      if (isROKS && row.topology === 'Localnet' && row.status === 'supported') {
        return { ...row, status: 'coming-soon' as SupportStatus, notes: 'Requires ROKS platform API' };
      }
      return row;
    }),
    [isROKS],
  );

  return (
    <ExpandableSection
      toggleText={isExpanded ? 'OVN Network Types Reference' : 'OVN Network Types Reference'}
      onToggle={(_e, expanded) => setIsExpanded(expanded)}
      isExpanded={isExpanded}
      style={{ marginBottom: '16px' }}
    >
      <Text component={TextVariants.p} style={{ marginBottom: '12px', color: 'var(--pf-v5-global--Color--200)' }}>
        All supported topology, scope, and role combinations for OVN user-defined network CRDs.
      </Text>
      <Table aria-label="OVN Network Types reference" variant="compact">
        <Thead>
          <Tr>
            <Th>CRD</Th>
            <Th>Topology</Th>
            <Th>Allowed Roles</Th>
            <Th>IPAM</Th>
            <Th>Status</Th>
            <Th>Notes</Th>
          </Tr>
        </Thead>
        <Tbody>
          {effectiveRows.map((row, i) => (
            <Tr key={i}>
              <Td>{row.crd}</Td>
              <Td>{row.topology}</Td>
              <Td>{row.allowedRoles}</Td>
              <Td>{row.ipam}</Td>
              <Td>{statusLabel(row.status)}</Td>
              <Td>{row.notes}</Td>
            </Tr>
          ))}
        </Tbody>
      </Table>
    </ExpandableSection>
  );
};

export default NetworkTypesInfoPanel;
