import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';

// Must import after jest.mock is set up by setupTests
const { apiClient } = require('../client');

const mockConsoleFetch = consoleFetch as jest.MockedFunction<typeof consoleFetch>;

describe('VPCNetworkClient', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  const mockOkResponse = (data: unknown) => {
    mockConsoleFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    } as unknown as Response);
  };

  const mockErrorResponse = (status: number, statusText: string, body?: unknown) => {
    mockConsoleFetch.mockResolvedValue({
      ok: false,
      status,
      statusText,
      json: () => Promise.resolve(body || { code: 'ERR', message: statusText }),
    } as unknown as Response);
  };

  describe('listVPCs', () => {
    it('calls GET /vpcs', async () => {
      const vpcs = [{ id: 'vpc-1', name: 'my-vpc' }];
      mockOkResponse(vpcs);

      const result = await apiClient.listVPCs();
      expect(result.data).toEqual(vpcs);
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/vpcs'),
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('passes region as query param', async () => {
      mockOkResponse([]);
      await apiClient.listVPCs('us-south');
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/vpcs?region=us-south'),
        expect.any(Object),
      );
    });
  });

  describe('getVPC', () => {
    it('calls GET /vpcs/:id', async () => {
      const vpc = { id: 'vpc-1', name: 'my-vpc' };
      mockOkResponse(vpc);

      const result = await apiClient.getVPC('vpc-1');
      expect(result.data).toEqual(vpc);
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/vpcs/vpc-1'),
        expect.any(Object),
      );
    });
  });

  describe('error handling', () => {
    it('returns API error on non-ok response', async () => {
      mockErrorResponse(404, 'Not Found', { code: 'NOT_FOUND', message: 'VPC not found' });

      const result = await apiClient.getVPC('nonexistent');
      expect(result.error).toEqual({ code: 'NOT_FOUND', message: 'VPC not found' });
      expect(result.data).toBeUndefined();
    });

    it('returns network error on fetch failure', async () => {
      mockConsoleFetch.mockRejectedValue(new Error('Network timeout'));

      const result = await apiClient.listVPCs();
      expect(result.error?.code).toBe('NETWORK_ERROR');
      expect(result.error?.message).toBe('Network timeout');
    });
  });

  describe('CUDN operations', () => {
    it('listCUDNs calls GET /cudns', async () => {
      const cudns = [{ name: 'net-1', kind: 'ClusterUserDefinedNetwork', topology: 'LocalNet' }];
      mockOkResponse(cudns);

      const result = await apiClient.listCUDNs();
      expect(result.data).toEqual(cudns);
    });

    it('getCUDN calls GET /cudns/:name', async () => {
      const cudn = { name: 'net-1', kind: 'ClusterUserDefinedNetwork', topology: 'LocalNet' };
      mockOkResponse(cudn);

      const result = await apiClient.getCUDN('net-1');
      expect(result.data).toEqual(cudn);
    });

    it('createCUDN calls POST /cudns', async () => {
      const cudn = { name: 'new-net', kind: 'ClusterUserDefinedNetwork', topology: 'LocalNet' };
      mockOkResponse(cudn);

      const result = await apiClient.createCUDN({ name: 'new-net', topology: 'LocalNet' });
      expect(result.data).toEqual(cudn);
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/cudns'),
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('deleteCUDN calls DELETE /cudns/:name', async () => {
      mockOkResponse(undefined);

      await apiClient.deleteCUDN('net-1');
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/cudns/net-1'),
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('UDN operations', () => {
    it('getUDN calls GET /udns/:namespace/:name', async () => {
      const udn = { name: 'net-1', namespace: 'ns-1', kind: 'UserDefinedNetwork', topology: 'Layer2' };
      mockOkResponse(udn);

      const result = await apiClient.getUDN('ns-1', 'net-1');
      expect(result.data).toEqual(udn);
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/udns/ns-1/net-1'),
        expect.any(Object),
      );
    });

    it('deleteUDN calls DELETE /udns/:namespace/:name', async () => {
      mockOkResponse(undefined);

      await apiClient.deleteUDN('ns-1', 'net-1');
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/udns/ns-1/net-1'),
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('getNetworkTypes', () => {
    it('returns network type combinations', async () => {
      const types = {
        topologies: ['LocalNet', 'Layer2'],
        scopes: ['ClusterUserDefinedNetwork', 'UserDefinedNetwork'],
        roles: ['Primary', 'Secondary'],
        combinations: [{ id: 'c1', topology: 'LocalNet', tier: 'recommended' }],
      };
      mockOkResponse(types);

      const result = await apiClient.getNetworkTypes();
      expect(result.data).toEqual(types);
    });
  });

  describe('security group operations', () => {
    it('listSecurityGroups with vpcId filter', async () => {
      mockOkResponse([]);
      await apiClient.listSecurityGroups('vpc-1');
      expect(mockConsoleFetch).toHaveBeenCalledWith(
        expect.stringContaining('/security-groups?vpcId=vpc-1'),
        expect.any(Object),
      );
    });

    it('addSecurityGroupRule calls POST /security-groups/:id/rules', async () => {
      const rule = { id: 'rule-1', direction: 'inbound', protocol: 'tcp' };
      mockOkResponse(rule);

      const result = await apiClient.addSecurityGroupRule('sg-1', {
        direction: 'inbound',
        protocol: 'tcp',
        portMin: 443,
        portMax: 443,
      });
      expect(result.data).toEqual(rule);
    });
  });
});
