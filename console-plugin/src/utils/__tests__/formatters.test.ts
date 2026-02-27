import {
  formatTimestamp,
  formatRelativeTime,
  formatIPAddress,
  formatCIDRBlock,
  formatPortRange,
  formatProtocol,
  formatBytes,
  formatStatus,
  formatResourceName,
  formatTags,
  truncateText,
  formatNumber,
  formatPercentage,
} from '../formatters';

describe('formatters', () => {
  describe('formatTimestamp', () => {
    it('returns "N/A" for undefined input', () => {
      expect(formatTimestamp()).toBe('N/A');
    });

    it('returns "Invalid date" for invalid date strings', () => {
      expect(formatTimestamp('not-a-date')).toBe('Invalid date');
    });

    it('formats a valid date string', () => {
      const result = formatTimestamp('2024-01-15T10:30:00Z');
      expect(result).not.toBe('N/A');
      expect(result).not.toBe('Invalid date');
    });
  });

  describe('formatRelativeTime', () => {
    it('returns "Unknown" for undefined input', () => {
      expect(formatRelativeTime()).toBe('Unknown');
    });

    it('returns "Invalid date" for invalid date strings', () => {
      expect(formatRelativeTime('not-a-date')).toBe('Invalid date');
    });

    it('returns "Just now" for very recent timestamps', () => {
      const now = new Date();
      expect(formatRelativeTime(now)).toBe('Just now');
    });
  });

  describe('formatIPAddress', () => {
    it('returns "Not assigned" for undefined', () => {
      expect(formatIPAddress()).toBe('Not assigned');
    });

    it('returns the trimmed IP address', () => {
      expect(formatIPAddress('  10.0.0.1  ')).toBe('10.0.0.1');
    });
  });

  describe('formatCIDRBlock', () => {
    it('returns "N/A" for undefined', () => {
      expect(formatCIDRBlock()).toBe('N/A');
    });

    it('formats a valid CIDR block', () => {
      expect(formatCIDRBlock('10.240.0.0/24')).toBe('10.240.0.0/24');
    });
  });

  describe('formatPortRange', () => {
    it('returns "All ports" when no range specified', () => {
      expect(formatPortRange()).toBe('All ports');
    });

    it('returns single port when min equals max', () => {
      expect(formatPortRange(80, 80)).toBe('80');
    });

    it('returns range when min and max differ', () => {
      expect(formatPortRange(80, 443)).toBe('80-443');
    });
  });

  describe('formatProtocol', () => {
    it('returns "N/A" for undefined', () => {
      expect(formatProtocol()).toBe('N/A');
    });

    it('formats known protocols', () => {
      expect(formatProtocol('tcp')).toBe('TCP');
      expect(formatProtocol('udp')).toBe('UDP');
      expect(formatProtocol('icmp')).toBe('ICMP');
      expect(formatProtocol('all')).toBe('All protocols');
    });

    it('uppercases unknown protocols', () => {
      expect(formatProtocol('gre')).toBe('GRE');
    });
  });

  describe('formatBytes', () => {
    it('returns "N/A" for undefined', () => {
      expect(formatBytes()).toBe('N/A');
    });

    it('formats bytes correctly', () => {
      expect(formatBytes(0)).toBe('0.00 B');
      expect(formatBytes(1024)).toBe('1.00 KB');
      expect(formatBytes(1048576)).toBe('1.00 MB');
      expect(formatBytes(1073741824)).toBe('1.00 GB');
    });
  });

  describe('formatStatus', () => {
    it('returns "Unknown" for undefined', () => {
      expect(formatStatus()).toBe('Unknown');
    });

    it('capitalizes status words', () => {
      expect(formatStatus('available')).toBe('Available');
      expect(formatStatus('in-progress')).toBe('In Progress');
    });
  });

  describe('formatResourceName', () => {
    it('returns name when present', () => {
      expect(formatResourceName('my-resource', 'abc12345-xyz')).toBe('my-resource');
    });

    it('returns truncated ID when no name', () => {
      expect(formatResourceName(undefined, 'abc12345-xyz')).toBe('abc12345');
    });

    it('returns "Unknown" when both are missing', () => {
      expect(formatResourceName()).toBe('Unknown');
    });
  });

  describe('formatTags', () => {
    it('returns "None" for undefined or empty', () => {
      expect(formatTags()).toBe('None');
      expect(formatTags([])).toBe('None');
    });

    it('joins tags with comma', () => {
      expect(formatTags(['env=prod', 'team=net'])).toBe('env=prod, team=net');
    });
  });

  describe('truncateText', () => {
    it('returns empty string for undefined', () => {
      expect(truncateText()).toBe('');
    });

    it('returns text as-is when under max length', () => {
      expect(truncateText('short', 50)).toBe('short');
    });

    it('truncates long text with ellipsis', () => {
      expect(truncateText('a'.repeat(60), 50)).toBe('a'.repeat(50) + '...');
    });
  });

  describe('formatNumber', () => {
    it('returns "N/A" for undefined', () => {
      expect(formatNumber()).toBe('N/A');
    });

    it('formats numbers with locale separators', () => {
      const result = formatNumber(1000);
      // The exact format depends on locale, but it should not be 'N/A'
      expect(result).not.toBe('N/A');
    });
  });

  describe('formatPercentage', () => {
    it('returns "N/A" for undefined', () => {
      expect(formatPercentage()).toBe('N/A');
    });

    it('formats percentage with decimals', () => {
      expect(formatPercentage(95.5)).toBe('95.50%');
      expect(formatPercentage(100, 0)).toBe('100%');
    });
  });
});
