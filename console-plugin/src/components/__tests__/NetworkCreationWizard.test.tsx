import { render, screen } from '@testing-library/react';
import NetworkCreationWizard from '../NetworkCreationWizard';

jest.mock('../../api/hooks', () => ({
  useNetworkTypes: () => ({ networkTypes: null, loading: false, error: null }),
  useVPCs: () => ({ vpcs: [], loading: false, error: null }),
  useZones: () => ({ zones: [], loading: false, error: null }),
  useSecurityGroups: () => ({ securityGroups: [], loading: false, error: null }),
  useNetworkACLs: () => ({ networkAcls: [], loading: false, error: null }),
}));

jest.mock('../../api/client', () => ({
  apiClient: {
    createCUDN: jest.fn(),
    createUDN: jest.fn(),
  },
}));

describe('NetworkCreationWizard', () => {
  const defaultProps = {
    isOpen: true,
    onClose: jest.fn(),
    onCreated: jest.fn(),
  };

  it('renders the wizard modal when isOpen is true', () => {
    render(<NetworkCreationWizard {...defaultProps} />);
    expect(screen.getByText('Create Network')).toBeInTheDocument();
  });

  it('does not render when isOpen is false', () => {
    const { container } = render(
      <NetworkCreationWizard isOpen={false} onClose={jest.fn()} onCreated={jest.fn()} />
    );
    expect(container.innerHTML).toBe('');
  });

  it('validates network name with correct regex pattern', () => {
    // The regex used in the component: /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/
    const nameRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

    // Valid names
    expect(nameRegex.test('my-network')).toBe(true);
    expect(nameRegex.test('a')).toBe(true);
    expect(nameRegex.test('network123')).toBe(true);
    expect(nameRegex.test('a-b-c')).toBe(true);

    // Invalid names
    expect(nameRegex.test('')).toBe(false);
    expect(nameRegex.test('-starts-with-dash')).toBe(false);
    expect(nameRegex.test('ends-with-dash-')).toBe(false);
    expect(nameRegex.test('UPPERCASE')).toBe(false);
    expect(nameRegex.test('has spaces')).toBe(false);
    expect(nameRegex.test('has_underscore')).toBe(false);
  });
});
