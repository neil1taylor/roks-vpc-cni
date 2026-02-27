import React from 'react';
import { Label } from '@patternfly/react-core';
import { NetworkTier } from '../api/types';

interface TierBadgeProps {
  tier: NetworkTier;
}

const tierConfig: Record<NetworkTier, { color: 'green' | 'blue' | 'orange'; text: string }> = {
  recommended: { color: 'green', text: 'Recommended' },
  advanced: { color: 'blue', text: 'Advanced' },
  expert: { color: 'orange', text: 'Expert' },
};

const TierBadge: React.FC<TierBadgeProps> = ({ tier }) => {
  const config = tierConfig[tier] || { color: 'grey' as const, text: tier };
  return <Label color={config.color} isCompact>{config.text}</Label>;
};

TierBadge.displayName = 'TierBadge';
export default TierBadge;
