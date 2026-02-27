import { render, screen } from '@testing-library/react';
import IPModeInfoAlert from '../IPModeInfoAlert';
import { IPAssignmentMode } from '../../api/types';

describe('IPModeInfoAlert', () => {
  it('renders static_reserved info', () => {
    render(<IPModeInfoAlert mode="static_reserved" />);
    expect(screen.getByText(/IP Assignment: Static Reserved IP/)).toBeInTheDocument();
    expect(screen.getByText(/VPC API reserves an IP/)).toBeInTheDocument();
  });

  it('renders dhcp info', () => {
    render(<IPModeInfoAlert mode="dhcp" />);
    expect(screen.getByText(/IP Assignment: DHCP/)).toBeInTheDocument();
    expect(screen.getByText(/OVN.*built-in DHCP server/)).toBeInTheDocument();
  });

  it('renders none info', () => {
    render(<IPModeInfoAlert mode="none" />);
    expect(screen.getByText(/IP Assignment: No Automatic IP/)).toBeInTheDocument();
    expect(screen.getByText(/Pure L2 connectivity/)).toBeInTheDocument();
  });

  it('uses custom description when provided', () => {
    render(<IPModeInfoAlert mode="static_reserved" description="Custom text" />);
    expect(screen.getByText('Custom text')).toBeInTheDocument();
  });

  it('returns null for unknown mode', () => {
    const { container } = render(<IPModeInfoAlert mode={'unknown' as IPAssignmentMode} />);
    expect(container.firstChild).toBeNull();
  });
});
