import React, { useState } from 'react';
import { Card, CardBody, CardTitle } from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td, ThProps } from '@patternfly/react-table';
import { NFTRuleMetrics } from '../../api/types';
import { formatBytes, formatNumber } from '../../utils/formatters';

interface NFTCountersTableProps {
  rules: NFTRuleMetrics[];
}

type SortKey = 'table' | 'chain' | 'comment' | 'packets' | 'bytes';

const NFTCountersTable: React.FC<NFTCountersTableProps> = ({ rules }) => {
  const [sortKey, setSortKey] = useState<SortKey>('packets');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');

  const sorted = [...rules].sort((a, b) => {
    const av = a[sortKey];
    const bv = b[sortKey];
    if (typeof av === 'number' && typeof bv === 'number') {
      return sortDir === 'asc' ? av - bv : bv - av;
    }
    const cmp = String(av).localeCompare(String(bv));
    return sortDir === 'asc' ? cmp : -cmp;
  });

  const getSortParams = (key: SortKey): ThProps['sort'] => ({
    sortBy: { index: ['table', 'chain', 'comment', 'packets', 'bytes'].indexOf(sortKey), direction: sortDir },
    onSort: (_event, _index, direction) => {
      setSortKey(key);
      setSortDir(direction);
    },
    columnIndex: ['table', 'chain', 'comment', 'packets', 'bytes'].indexOf(key),
  });

  if (rules.length === 0) {
    return (
      <Card isCompact>
        <CardTitle>NFT Rule Counters</CardTitle>
        <CardBody>No rule counters available</CardBody>
      </Card>
    );
  }

  return (
    <Card isCompact>
      <CardTitle>NFT Rule Counters</CardTitle>
      <CardBody>
        <Table aria-label="NFT rule counters" variant="compact">
          <Thead>
            <Tr>
              <Th sort={getSortParams('table')}>Table</Th>
              <Th sort={getSortParams('chain')}>Chain</Th>
              <Th sort={getSortParams('comment')}>Rule</Th>
              <Th sort={getSortParams('packets')}>Packets</Th>
              <Th sort={getSortParams('bytes')}>Bytes</Th>
            </Tr>
          </Thead>
          <Tbody>
            {sorted.map((rule, i) => (
              <Tr key={`${rule.table}-${rule.chain}-${rule.comment}-${i}`}>
                <Td>{rule.table}</Td>
                <Td>{rule.chain}</Td>
                <Td>{rule.comment || '—'}</Td>
                <Td>{formatNumber(rule.packets)}</Td>
                <Td>{formatBytes(rule.bytes)}</Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      </CardBody>
    </Card>
  );
};

export default NFTCountersTable;
