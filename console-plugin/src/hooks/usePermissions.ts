import { useState, useEffect } from 'react';
import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';
import { UserPermissions } from '../api/types';

/**
 * Hook for checking user permissions
 * Calls the auth API to determine user's role and capabilities
 */
export function usePermissions() {
  const [permissions, setPermissions] = useState<UserPermissions | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let mounted = true;

    const fetchPermissions = async () => {
      try {
        setLoading(true);
        const response = await consoleFetch('/api/v1/auth/permissions');

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data = (await response.json()) as UserPermissions;

        if (mounted) {
          setPermissions(data);
          setError(null);
        }
      } catch (err) {
        const error = err as Error;
        if (mounted) {
          setError(error);
          // Set default permissions (read-only)
          setPermissions({
            isAdmin: false,
            canWrite: false,
            canDelete: false,
          });
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    fetchPermissions();

    return () => {
      mounted = false;
    };
  }, []);

  return {
    ...permissions,
    loading,
    error,
    isAdmin: permissions?.isAdmin ?? false,
    canWrite: permissions?.canWrite ?? false,
    canDelete: permissions?.canDelete ?? false,
  };
}

export default usePermissions;
