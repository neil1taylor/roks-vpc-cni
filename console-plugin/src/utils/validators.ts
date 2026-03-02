/**
 * Utility functions for validating network configuration
 */

/**
 * Validate IPv4 address
 */
export function isValidIPv4(ip: string): boolean {
  const ipv4Regex = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/;
  return ipv4Regex.test(ip);
}

/**
 * Validate IPv6 address
 */
export function isValidIPv6(ip: string): boolean {
  const ipv6Regex = /^(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$/;
  return ipv6Regex.test(ip);
}

/**
 * Validate IP address (IPv4 or IPv6)
 */
export function isValidIPAddress(ip: string): boolean {
  return isValidIPv4(ip) || isValidIPv6(ip);
}

/**
 * Validate CIDR block (IPv4)
 */
export function isValidCIDRv4(cidr: string): boolean {
  const cidrRegex = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])(\/([0-9]|[1-2][0-9]|3[0-2]))$/;
  return cidrRegex.test(cidr);
}

/**
 * Validate CIDR block (IPv6)
 */
export function isValidCIDRv6(cidr: string): boolean {
  const parts = cidr.split('/');
  if (parts.length !== 2) {
    return false;
  }

  const [ip, prefix] = parts;
  const prefixNum = parseInt(prefix, 10);

  if (isNaN(prefixNum) || prefixNum < 0 || prefixNum > 128) {
    return false;
  }

  return isValidIPv6(ip);
}

/**
 * Validate CIDR block (IPv4 or IPv6)
 */
export function isValidCIDRBlock(cidr: string): boolean {
  return isValidCIDRv4(cidr) || isValidCIDRv6(cidr);
}

/**
 * Validate port number
 */
export function isValidPort(port: number | string): boolean {
  const portNum = typeof port === 'string' ? parseInt(port, 10) : port;
  return !isNaN(portNum) && portNum > 0 && portNum <= 65535;
}

/**
 * Validate port range
 */
export function isValidPortRange(min: number | string, max: number | string): boolean {
  const minNum = typeof min === 'string' ? parseInt(min, 10) : min;
  const maxNum = typeof max === 'string' ? parseInt(max, 10) : max;

  return (
    isValidPort(minNum)
    && isValidPort(maxNum)
    && minNum <= maxNum
  );
}

/**
 * Validate protocol
 */
export function isValidProtocol(protocol: string): boolean {
  const validProtocols = ['tcp', 'udp', 'icmp', 'all'];
  return validProtocols.includes(protocol.toLowerCase());
}

/**
 * Validate MAC address (colon-separated hex, e.g. "fa:16:3e:aa:bb:cc")
 */
export function isValidMAC(mac: string): boolean {
  return /^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/.test(mac);
}

/**
 * Validate VLAN ID
 */
export function isValidVLANId(vlan: number | string): boolean {
  const vlanNum = typeof vlan === 'string' ? parseInt(vlan, 10) : vlan;
  return !isNaN(vlanNum) && vlanNum > 0 && vlanNum <= 4094;
}

/**
 * Validate resource name
 */
export function isValidResourceName(name: string): boolean {
  // Names should be alphanumeric with hyphens, underscores, and periods
  const nameRegex = /^[a-zA-Z0-9_.-]+$/;
  return name.length > 0 && name.length <= 255 && nameRegex.test(name);
}

/**
 * Validate tag format
 */
export function isValidTag(tag: string): boolean {
  // Tag format: key=value
  const parts = tag.split('=');
  return parts.length === 2 && parts[0].length > 0 && parts[1].length > 0;
}

/**
 * Validate tags array
 */
export function isValidTags(tags: string[]): boolean {
  if (!Array.isArray(tags)) {
    return false;
  }

  return tags.every(tag => isValidTag(tag));
}

/**
 * Validate action value for ACL rules
 */
export function isValidACLAction(action: string): boolean {
  return ['allow', 'deny'].includes(action.toLowerCase());
}

/**
 * Validate direction value
 */
export function isValidDirection(direction: string): boolean {
  return ['inbound', 'outbound', 'ingress', 'egress'].includes(direction.toLowerCase());
}

/**
 * Validate IP version
 */
export function isValidIPVersion(version: string): boolean {
  return ['ipv4', 'ipv6'].includes(version.toLowerCase());
}

/**
 * Validate ICMP type
 */
export function isValidICMPType(type: number | string): boolean {
  const typeNum = typeof type === 'string' ? parseInt(type, 10) : type;
  return !isNaN(typeNum) && typeNum >= 0 && typeNum <= 255;
}

/**
 * Validate ICMP code
 */
export function isValidICMPCode(code: number | string): boolean {
  const codeNum = typeof code === 'string' ? parseInt(code, 10) : code;
  return !isNaN(codeNum) && codeNum >= 0 && codeNum <= 255;
}

/**
 * Validate network configuration
 */
export interface NetworkValidationResult {
  valid: boolean;
  errors: string[];
}

export function validateSubnetConfiguration(
  cidrBlock: string,
  minIPs: number = 4,
): NetworkValidationResult {
  const errors: string[] = [];

  if (!isValidCIDRBlock(cidrBlock)) {
    errors.push('Invalid CIDR block format');
  }

  // Extract prefix length
  const parts = cidrBlock.split('/');
  if (parts.length === 2) {
    const prefixLength = parseInt(parts[1], 10);
    const isIPv4 = isValidCIDRv4(cidrBlock);

    if (isIPv4 && prefixLength < 16) {
      // IPv4 with /16 or larger (more than 65k addresses)
      // This is acceptable but large
    }

    // Check if there are enough addresses
    const availableAddresses = isIPv4
      ? Math.pow(2, 32 - prefixLength)
      : Math.pow(2, 128 - prefixLength);

    if (availableAddresses < minIPs) {
      errors.push(`Subnet must have at least ${minIPs} available addresses`);
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

export function validateSecurityGroupRuleConfiguration(
  protocol: string,
  portMin?: number,
  portMax?: number,
  icmpType?: number,
  icmpCode?: number,
): NetworkValidationResult {
  const errors: string[] = [];

  if (!isValidProtocol(protocol)) {
    errors.push('Invalid protocol');
  }

  const protocolLower = protocol.toLowerCase();

  if (protocolLower === 'icmp') {
    if (icmpType !== undefined && !isValidICMPType(icmpType)) {
      errors.push('Invalid ICMP type');
    }
    if (icmpCode !== undefined && !isValidICMPCode(icmpCode)) {
      errors.push('Invalid ICMP code');
    }
  } else if (protocolLower === 'tcp' || protocolLower === 'udp' || protocolLower === 'all') {
    if (portMin !== undefined && portMax !== undefined) {
      if (!isValidPortRange(portMin, portMax)) {
        errors.push('Invalid port range');
      }
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}
