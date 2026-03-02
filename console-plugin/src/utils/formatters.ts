/**
 * Utility functions for formatting data for display
 */

/**
 * Format timestamp to readable date string
 */
export function formatTimestamp(timestamp?: string | number | Date): string {
  if (!timestamp) {
    return 'N/A';
  }

  try {
    const date = typeof timestamp === 'string' || typeof timestamp === 'number'
      ? new Date(timestamp)
      : timestamp;

    if (isNaN(date.getTime())) {
      return 'Invalid date';
    }

    return date.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      timeZoneName: 'short',
    });
  } catch (e) {
    return 'N/A';
  }
}

/**
 * Format relative time (e.g., "2 hours ago")
 */
export function formatRelativeTime(timestamp?: string | number | Date): string {
  if (!timestamp) {
    return 'Unknown';
  }

  try {
    const date = typeof timestamp === 'string' || typeof timestamp === 'number'
      ? new Date(timestamp)
      : timestamp;

    if (isNaN(date.getTime())) {
      return 'Invalid date';
    }

    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) {
      return 'Just now';
    }
    if (diffMins < 60) {
      return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
    }
    if (diffHours < 24) {
      return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    }
    if (diffDays < 30) {
      return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
    }

    return formatTimestamp(timestamp);
  } catch (e) {
    return 'N/A';
  }
}

/**
 * Format IP address (simple validation and display)
 */
export function formatIPAddress(ip?: string): string {
  if (!ip) {
    return 'Not assigned';
  }

  return ip.trim();
}

/**
 * Format CIDR block with validation
 */
export function formatCIDRBlock(cidr?: string): string {
  if (!cidr) {
    return 'N/A';
  }

  const trimmed = cidr.trim();
  const parts = trimmed.split('/');

  if (parts.length === 2) {
    const [ip, prefix] = parts;
    return `${ip}/${prefix}`;
  }

  return trimmed;
}

/**
 * Format port range for display
 */
export function formatPortRange(min?: number, max?: number): string {
  if (min === undefined || max === undefined) {
    return 'All ports';
  }

  if (min === max) {
    return `${min}`;
  }

  return `${min}-${max}`;
}

/**
 * Format protocol name for display
 */
export function formatProtocol(protocol?: string): string {
  if (!protocol) {
    return 'N/A';
  }

  const protocolMap: Record<string, string> = {
    tcp: 'TCP',
    udp: 'UDP',
    icmp: 'ICMP',
    all: 'All protocols',
  };

  return protocolMap[protocol.toLowerCase()] || protocol.toUpperCase();
}

/**
 * Format bytes to human-readable format
 */
export function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes === null) {
    return 'N/A';
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = Math.abs(bytes);
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }

  return `${size.toFixed(2)} ${units[unitIndex]}`;
}

/**
 * Format status for display with capitalization
 */
export function formatStatus(status?: string): string {
  if (!status) {
    return 'Unknown';
  }

  return status
    .split('-')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
}

/**
 * Format resource name for display
 */
export function formatResourceName(name?: string, id?: string): string {
  if (name) {
    return name;
  }

  if (id) {
    return id.substring(0, 8);
  }

  return 'Unknown';
}

/**
 * Format tags array to display string
 */
export function formatTags(tags?: string[]): string {
  if (!tags || tags.length === 0) {
    return 'None';
  }

  return tags.join(', ');
}

/**
 * Truncate long text with ellipsis
 */
export function truncateText(text?: string, maxLength: number = 50): string {
  if (!text) {
    return '';
  }

  if (text.length <= maxLength) {
    return text;
  }

  return `${text.substring(0, maxLength)}...`;
}

/**
 * Format number with thousands separator
 */
export function formatNumber(num?: number): string {
  if (num === undefined || num === null) {
    return 'N/A';
  }

  return num.toLocaleString();
}

/**
 * Format percentage
 */
export function formatPercentage(value?: number, decimals: number = 2): string {
  if (value === undefined || value === null) {
    return 'N/A';
  }

  return `${value.toFixed(decimals)}%`;
}

/**
 * Format bytes per second to human-readable rate
 */
export function formatBytesPerSec(bps?: number): string {
  if (bps === undefined || bps === null) {
    return '0 B/s';
  }
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
  let size = Math.abs(bps);
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return unitIndex === 0 ? `${Math.round(size)} ${units[unitIndex]}` : `${size.toFixed(1)} ${units[unitIndex]}`;
}

/**
 * Format seconds to human-readable uptime (e.g., "3d 14h 22m")
 */
export function formatUptime(seconds?: number): string {
  if (seconds === undefined || seconds === null || seconds < 0) {
    return 'N/A';
  }
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}
