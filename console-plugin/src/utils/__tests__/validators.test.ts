import {
  isValidIPv4,
  isValidIPv6,
  isValidIPAddress,
  isValidCIDRv4,
  isValidCIDRBlock,
  isValidPort,
  isValidPortRange,
  isValidProtocol,
  isValidVLANId,
  isValidResourceName,
  isValidTag,
  isValidTags,
  isValidACLAction,
  isValidDirection,
  isValidIPVersion,
  isValidICMPType,
  isValidICMPCode,
  validateSubnetConfiguration,
  validateSecurityGroupRuleConfiguration,
} from '../validators';

describe('validators', () => {
  describe('isValidIPv4', () => {
    it('accepts valid IPv4 addresses', () => {
      expect(isValidIPv4('192.168.1.1')).toBe(true);
      expect(isValidIPv4('10.0.0.0')).toBe(true);
      expect(isValidIPv4('255.255.255.255')).toBe(true);
      expect(isValidIPv4('0.0.0.0')).toBe(true);
    });

    it('rejects invalid IPv4 addresses', () => {
      expect(isValidIPv4('256.0.0.0')).toBe(false);
      expect(isValidIPv4('192.168.1')).toBe(false);
      expect(isValidIPv4('not-an-ip')).toBe(false);
      expect(isValidIPv4('')).toBe(false);
    });
  });

  describe('isValidIPv6', () => {
    it('accepts valid IPv6 addresses', () => {
      expect(isValidIPv6('::1')).toBe(true);
      expect(isValidIPv6('2001:0db8:85a3:0000:0000:8a2e:0370:7334')).toBe(true);
    });

    it('rejects invalid IPv6 addresses', () => {
      expect(isValidIPv6('192.168.1.1')).toBe(false);
      expect(isValidIPv6('not-an-ip')).toBe(false);
    });
  });

  describe('isValidIPAddress', () => {
    it('accepts both valid IPv4 and IPv6', () => {
      expect(isValidIPAddress('192.168.1.1')).toBe(true);
      expect(isValidIPAddress('::1')).toBe(true);
    });

    it('rejects invalid addresses', () => {
      expect(isValidIPAddress('invalid')).toBe(false);
    });
  });

  describe('isValidCIDRv4', () => {
    it('accepts valid CIDR blocks', () => {
      expect(isValidCIDRv4('10.240.0.0/24')).toBe(true);
      expect(isValidCIDRv4('192.168.0.0/16')).toBe(true);
      expect(isValidCIDRv4('0.0.0.0/0')).toBe(true);
    });

    it('rejects invalid CIDR blocks', () => {
      expect(isValidCIDRv4('10.240.0.0')).toBe(false);
      expect(isValidCIDRv4('10.240.0.0/33')).toBe(false);
      expect(isValidCIDRv4('invalid/24')).toBe(false);
    });
  });

  describe('isValidCIDRBlock', () => {
    it('accepts both IPv4 and IPv6 CIDR blocks', () => {
      expect(isValidCIDRBlock('10.0.0.0/8')).toBe(true);
    });
  });

  describe('isValidPort', () => {
    it('accepts valid port numbers', () => {
      expect(isValidPort(1)).toBe(true);
      expect(isValidPort(80)).toBe(true);
      expect(isValidPort(443)).toBe(true);
      expect(isValidPort(65535)).toBe(true);
      expect(isValidPort('8080')).toBe(true);
    });

    it('rejects invalid port numbers', () => {
      expect(isValidPort(0)).toBe(false);
      expect(isValidPort(65536)).toBe(false);
      expect(isValidPort(-1)).toBe(false);
      expect(isValidPort('abc')).toBe(false);
    });
  });

  describe('isValidPortRange', () => {
    it('accepts valid port ranges', () => {
      expect(isValidPortRange(80, 443)).toBe(true);
      expect(isValidPortRange(80, 80)).toBe(true);
      expect(isValidPortRange('1', '65535')).toBe(true);
    });

    it('rejects invalid port ranges', () => {
      expect(isValidPortRange(443, 80)).toBe(false);
      expect(isValidPortRange(0, 80)).toBe(false);
    });
  });

  describe('isValidProtocol', () => {
    it('accepts valid protocols', () => {
      expect(isValidProtocol('tcp')).toBe(true);
      expect(isValidProtocol('udp')).toBe(true);
      expect(isValidProtocol('icmp')).toBe(true);
      expect(isValidProtocol('all')).toBe(true);
      expect(isValidProtocol('TCP')).toBe(true);
    });

    it('rejects invalid protocols', () => {
      expect(isValidProtocol('http')).toBe(false);
      expect(isValidProtocol('')).toBe(false);
    });
  });

  describe('isValidVLANId', () => {
    it('accepts valid VLAN IDs', () => {
      expect(isValidVLANId(1)).toBe(true);
      expect(isValidVLANId(100)).toBe(true);
      expect(isValidVLANId(4094)).toBe(true);
      expect(isValidVLANId('100')).toBe(true);
    });

    it('rejects invalid VLAN IDs', () => {
      expect(isValidVLANId(0)).toBe(false);
      expect(isValidVLANId(4095)).toBe(false);
      expect(isValidVLANId(-1)).toBe(false);
    });
  });

  describe('isValidResourceName', () => {
    it('accepts valid resource names', () => {
      expect(isValidResourceName('my-resource')).toBe(true);
      expect(isValidResourceName('resource_1')).toBe(true);
      expect(isValidResourceName('a.b.c')).toBe(true);
    });

    it('rejects invalid resource names', () => {
      expect(isValidResourceName('')).toBe(false);
      expect(isValidResourceName('has spaces')).toBe(false);
    });
  });

  describe('isValidTag', () => {
    it('accepts valid tags', () => {
      expect(isValidTag('env=prod')).toBe(true);
      expect(isValidTag('cluster=my-cluster')).toBe(true);
    });

    it('rejects invalid tags', () => {
      expect(isValidTag('notag')).toBe(false);
      expect(isValidTag('=value')).toBe(false);
      expect(isValidTag('key=')).toBe(false);
    });
  });

  describe('isValidTags', () => {
    it('accepts valid tags arrays', () => {
      expect(isValidTags(['env=prod', 'cluster=test'])).toBe(true);
      expect(isValidTags([])).toBe(true);
    });

    it('rejects invalid inputs', () => {
      expect(isValidTags(['invalid'])).toBe(false);
    });
  });

  describe('isValidACLAction', () => {
    it('accepts valid actions', () => {
      expect(isValidACLAction('allow')).toBe(true);
      expect(isValidACLAction('deny')).toBe(true);
      expect(isValidACLAction('ALLOW')).toBe(true);
    });

    it('rejects invalid actions', () => {
      expect(isValidACLAction('reject')).toBe(false);
    });
  });

  describe('isValidDirection', () => {
    it('accepts valid directions', () => {
      expect(isValidDirection('inbound')).toBe(true);
      expect(isValidDirection('outbound')).toBe(true);
      expect(isValidDirection('ingress')).toBe(true);
      expect(isValidDirection('egress')).toBe(true);
    });

    it('rejects invalid directions', () => {
      expect(isValidDirection('left')).toBe(false);
    });
  });

  describe('isValidIPVersion', () => {
    it('accepts valid IP versions', () => {
      expect(isValidIPVersion('ipv4')).toBe(true);
      expect(isValidIPVersion('ipv6')).toBe(true);
    });

    it('rejects invalid IP versions', () => {
      expect(isValidIPVersion('ipv5')).toBe(false);
    });
  });

  describe('isValidICMPType', () => {
    it('accepts valid ICMP types', () => {
      expect(isValidICMPType(0)).toBe(true);
      expect(isValidICMPType(8)).toBe(true);
      expect(isValidICMPType(255)).toBe(true);
    });

    it('rejects invalid ICMP types', () => {
      expect(isValidICMPType(-1)).toBe(false);
      expect(isValidICMPType(256)).toBe(false);
    });
  });

  describe('isValidICMPCode', () => {
    it('accepts valid ICMP codes', () => {
      expect(isValidICMPCode(0)).toBe(true);
      expect(isValidICMPCode(255)).toBe(true);
    });

    it('rejects invalid ICMP codes', () => {
      expect(isValidICMPCode(-1)).toBe(false);
      expect(isValidICMPCode(256)).toBe(false);
    });
  });

  describe('validateSubnetConfiguration', () => {
    it('accepts valid subnet configurations', () => {
      const result = validateSubnetConfiguration('10.240.0.0/24');
      expect(result.valid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });

    it('rejects invalid CIDR blocks', () => {
      const result = validateSubnetConfiguration('invalid');
      expect(result.valid).toBe(false);
      expect(result.errors).toContain('Invalid CIDR block format');
    });

    it('rejects subnets with too few addresses', () => {
      const result = validateSubnetConfiguration('10.0.0.0/32', 4);
      expect(result.valid).toBe(false);
    });
  });

  describe('validateSecurityGroupRuleConfiguration', () => {
    it('accepts valid TCP rule configuration', () => {
      const result = validateSecurityGroupRuleConfiguration('tcp', 80, 443);
      expect(result.valid).toBe(true);
    });

    it('accepts valid ICMP rule configuration', () => {
      const result = validateSecurityGroupRuleConfiguration('icmp', undefined, undefined, 8, 0);
      expect(result.valid).toBe(true);
    });

    it('rejects invalid protocol', () => {
      const result = validateSecurityGroupRuleConfiguration('http');
      expect(result.valid).toBe(false);
      expect(result.errors).toContain('Invalid protocol');
    });

    it('rejects invalid port range', () => {
      const result = validateSecurityGroupRuleConfiguration('tcp', 443, 80);
      expect(result.valid).toBe(false);
    });

    it('rejects invalid ICMP type', () => {
      const result = validateSecurityGroupRuleConfiguration('icmp', undefined, undefined, 256, 0);
      expect(result.valid).toBe(false);
    });
  });
});
