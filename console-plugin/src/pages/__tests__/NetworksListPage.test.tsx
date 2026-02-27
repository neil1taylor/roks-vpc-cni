import { render, screen } from '@testing-library/react';
import NetworksListPage from '../NetworksListPage';

jest.mock('../../api/hooks', () => ({
  useNetworkDefinitions: () => ({
    networks: [],
    loading: false,
    error: null,
    refetch: jest.fn(),
  }),
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

jest.mock('react-router-dom-v5-compat', () => ({
  ...jest.requireActual('react-router-dom-v5-compat'),
  useNavigate: () => jest.fn(),
}));

describe('NetworksListPage', () => {
  it('renders "Networks" heading', () => {
    render(<NetworksListPage />);
    expect(screen.getByText('Networks')).toBeInTheDocument();
  });

  it('shows "Create Network" button', () => {
    render(<NetworksListPage />);
    expect(screen.getByText('Create Network')).toBeInTheDocument();
  });

  it('shows empty state when no networks exist', () => {
    render(<NetworksListPage />);
    expect(screen.getByText('No networks found')).toBeInTheDocument();
  });
});
