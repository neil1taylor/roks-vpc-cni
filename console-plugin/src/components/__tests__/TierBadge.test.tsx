import { render, screen } from '@testing-library/react';
import TierBadge from '../TierBadge';
import { NetworkTier } from '../../api/types';

describe('TierBadge', () => {
  it('renders "Recommended" for recommended tier', () => {
    render(<TierBadge tier="recommended" />);
    expect(screen.getByText('Recommended')).toBeInTheDocument();
  });

  it('renders "Advanced" for advanced tier', () => {
    render(<TierBadge tier="advanced" />);
    expect(screen.getByText('Advanced')).toBeInTheDocument();
  });

  it('renders "Expert" for expert tier', () => {
    render(<TierBadge tier="expert" />);
    expect(screen.getByText('Expert')).toBeInTheDocument();
  });

  it('renders the tier string for unknown tiers', () => {
    render(<TierBadge tier={'custom' as NetworkTier} />);
    expect(screen.getByText('custom')).toBeInTheDocument();
  });
});
