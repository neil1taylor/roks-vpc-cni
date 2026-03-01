import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Modal,
  ModalVariant,
  Wizard,
  WizardStep,
  Card,
  CardBody,
  CardTitle,
  Gallery,
  GalleryItem,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  Alert,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Label,
  Spinner,
  Title,
  Text,
  TextVariants,
  Divider,
  FormHelperText,
  HelperText,
  HelperTextItem,
  ExpandableSection,
  Checkbox,
  TextInputGroup,
  TextInputGroupMain,
  Button,
} from '@patternfly/react-core';
import { useNetworkTypes, useVPCs, useZones, useSecurityGroups, useNetworkACLs, usePublicGateways, useClusterInfo, useAddressPrefixes } from '../api/hooks';
import { apiClient } from '../api/client';
import { NetworkCombination, NetworkTier, CreateNetworkRequest, NamespaceInfo, NetworkDefinition } from '../api/types';
import TierBadge from './TierBadge';
import IPModeInfoAlert from './IPModeInfoAlert';

// CIDR parsing and containment helpers

/** Parse "10.0.0.0/24" → { ip: 32-bit number, bits: prefix length } or null */
function parseCIDR(cidr: string): { ip: number; bits: number } | null {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/.exec(cidr.trim());
  if (!m) return null;
  const octets = [+m[1], +m[2], +m[3], +m[4]];
  const bits = +m[5];
  if (octets.some((o) => o > 255) || bits > 32) return null;
  const ip = ((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0;
  return { ip, bits };
}

/** Does the given CIDR fit entirely within any of the address prefixes? */
function cidrFitsAnyPrefix(cidr: string, prefixes: { cidr: string }[]): boolean {
  const inner = parseCIDR(cidr);
  if (!inner) return false; // invalid CIDR — don't match
  for (const p of prefixes) {
    const outer = parseCIDR(p.cidr);
    if (!outer) continue;
    // inner must be same or more specific (larger bits) than outer
    if (inner.bits < outer.bits) continue;
    // Check that inner's network falls within outer's range
    const outerMask = outer.bits === 0 ? 0 : (0xffffffff << (32 - outer.bits)) >>> 0;
    if ((inner.ip & outerMask) === (outer.ip & outerMask)) return true;
  }
  return false;
}

interface NetworkCreationWizardProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
}

const tierLabels: Record<NetworkTier, string> = { recommended: 'Recommended', advanced: 'Advanced', expert: 'Expert' };

const ipModeLabels: Record<string, string> = {
  static_reserved: 'Static Reserved IP',
  dhcp: 'DHCP',
  none: 'Manual',
};

const selectedBorder = '2px solid var(--pf-v5-global--primary-color--100)';

const NetworkCreationWizard: React.FC<NetworkCreationWizardProps> = ({ isOpen, onClose, onCreated }) => {
  const { networkTypes, loading: typesLoading } = useNetworkTypes();
  const { isROKS } = useClusterInfo();
  const { vpcs } = useVPCs();
  const { zones } = useZones();
  const { securityGroups } = useSecurityGroups();
  const { networkAcls } = useNetworkACLs();

  // One-time snapshot of existing networks for VLAN conflict detection (no polling)
  const [networks, setNetworks] = useState<NetworkDefinition[] | null>(null);
  useEffect(() => {
    if (!isOpen) return;
    Promise.all([apiClient.listCUDNs(), apiClient.listUDNs()]).then(([cudnResp, udnResp]) => {
      const all: NetworkDefinition[] = [];
      if (cudnResp.data) all.push(...cudnResp.data);
      if (udnResp.data) all.push(...udnResp.data);
      setNetworks(all);
    }).catch(() => {});
  }, [isOpen]);

  // Compute VLAN IDs already in use by existing networks
  const usedVlanIds = useMemo(() => {
    const ids = new Set<number>();
    (networks || []).forEach((n) => {
      const parsed = parseInt(n.vlan_id || '', 10);
      if (!isNaN(parsed)) ids.add(parsed);
    });
    return ids;
  }, [networks]);

  // Suggest the next available VLAN ID starting from 100
  const suggestedVlanId = useMemo(() => {
    for (let i = 100; i <= 4094; i++) {
      if (!usedVlanIds.has(i)) return i;
    }
    return 1;
  }, [usedVlanIds]);

  // Guided flow state (Step 1)
  const [selectedTopology, setSelectedTopology] = useState<'LocalNet' | 'Layer2' | null>(null);
  const [selectedScope, setSelectedScope] = useState<'ClusterUserDefinedNetwork' | 'UserDefinedNetwork' | null>(null);
  const [selectedRole, setSelectedRole] = useState<'Primary' | 'Secondary' | null>(null);
  const [advancedExpanded, setAdvancedExpanded] = useState(false);

  // Step 2 state
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('');
  const [targetNamespaces, setTargetNamespaces] = useState<string[]>([]);
  const [nsFilter, setNsFilter] = useState('');

  // Fetch available namespaces with primary label info from BFF
  const [allNamespaceInfos, setAllNamespaceInfos] = useState<NamespaceInfo[]>([]);
  const [newNamespaceName, setNewNamespaceName] = useState('');
  const [createNewNs, setCreateNewNs] = useState(false);
  useEffect(() => {
    if (!isOpen) return;
    apiClient.listNamespaces().then((resp) => {
      if (resp.data) {
        const filtered = resp.data
          .filter((ns) => !ns.name.startsWith('openshift-') && !ns.name.startsWith('kube-'))
          .sort((a, b) => a.name.localeCompare(b.name));
        setAllNamespaceInfos(filtered);
      }
    }).catch(() => setAllNamespaceInfos([]));
  }, [isOpen]);
  const allNamespaces = allNamespaceInfos.map((ns) => ns.name);

  // Step 3 state (VPC config)
  const [vpcId, setVpcId] = useState('');
  const [zone, setZone] = useState('');
  const [cidr, setCidr] = useState('');
  const [vlanId, setVlanId] = useState('');
  const [securityGroupIds, setSecurityGroupIds] = useState<string[]>([]);
  const [aclId, setAclId] = useState('');
  const [publicGatewayId, setPublicGatewayId] = useState('');
  const [createPrefix, setCreatePrefix] = useState(false);

  // Fetch address prefixes for the selected VPC
  const { addressPrefixes, loading: prefixesLoading } = useAddressPrefixes(vpcId || undefined);
  const { publicGateways } = usePublicGateways(vpcId || undefined);

  // Submit state
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const combinations = networkTypes?.combinations || [];

  // Derive available scopes and roles from the combinations returned by the BFF
  const availableScopes = useMemo(() => {
    if (!selectedTopology) return [] as string[];
    return [...new Set(combinations.filter((c) => c.topology === selectedTopology).map((c) => c.scope))];
  }, [selectedTopology, combinations]);

  const availableRoles = useMemo(() => {
    if (!selectedTopology || !selectedScope) return [] as string[];
    return [...new Set(combinations.filter((c) => c.topology === selectedTopology && c.scope === selectedScope).map((c) => c.role))];
  }, [selectedTopology, selectedScope, combinations]);

  // Auto-select scope when only one option available
  useEffect(() => {
    if (availableScopes.length === 1 && selectedScope !== availableScopes[0]) {
      setSelectedScope(availableScopes[0] as 'ClusterUserDefinedNetwork' | 'UserDefinedNetwork');
    }
  }, [availableScopes, selectedScope]);

  // Auto-select role when only one option available
  useEffect(() => {
    if (availableRoles.length === 1 && selectedRole !== availableRoles[0]) {
      setSelectedRole(availableRoles[0] as 'Primary' | 'Secondary');
    }
  }, [availableRoles, selectedRole]);

  // Derive selected combination from guided flow choices
  const selectedCombination = useMemo(() => {
    if (!selectedTopology || !selectedScope || !selectedRole) return null;
    return combinations.find(
      (c) => c.topology === selectedTopology && c.scope === selectedScope && c.role === selectedRole,
    ) || null;
  }, [selectedTopology, selectedScope, selectedRole, combinations]);

  // Group combinations by tier for advanced card grid
  const grouped = useMemo(() => {
    const groups: Record<NetworkTier, NetworkCombination[]> = {
      recommended: [],
      advanced: [],
      expert: [],
    };
    combinations.forEach((c) => {
      if (groups[c.tier]) groups[c.tier].push(c);
    });
    return groups;
  }, [combinations]);

  // Guided flow handlers with cascading reset
  const handleTopologySelect = useCallback((topology: 'LocalNet' | 'Layer2') => {
    setSelectedTopology(topology);
    setSelectedScope(null);
    setSelectedRole(null);
  }, []);

  const handleScopeSelect = useCallback((scope: 'ClusterUserDefinedNetwork' | 'UserDefinedNetwork') => {
    setSelectedScope(scope);
    setSelectedRole(null);
  }, []);

  const handleRoleSelect = useCallback((role: 'Primary' | 'Secondary') => {
    setSelectedRole(role);
  }, []);

  // Advanced card click sets all 3 guided states at once (bidirectional sync)
  const handleCardSelect = useCallback((combo: NetworkCombination) => {
    setSelectedTopology(combo.topology);
    setSelectedScope(combo.scope);
    setSelectedRole(combo.role);
  }, []);

  const resetForm = () => {
    setSelectedTopology(null);
    setSelectedScope(null);
    setSelectedRole(null);
    setAdvancedExpanded(false);
    setName('');
    setNamespace('');
    setTargetNamespaces([]);
    setNsFilter('');
    setNewNamespaceName('');
    setCreateNewNs(false);
    setVpcId('');
    setZone('');
    setCidr('');
    setVlanId('');
    setSecurityGroupIds([]);
    setAclId('');
    setCreatePrefix(false);
    setIsCreating(false);
    setCreateError(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleCreate = async () => {
    if (!selectedCombination || !name) return;
    setIsCreating(true);
    setCreateError(null);

    try {
      // For Primary networks: create new namespaces with the required label
      const isPrimary = selectedCombination.role === 'Primary';
      let effectiveNamespace = namespace;

      if (isPrimary) {
        // UDN with "create new namespace" flow
        if (selectedCombination.scope === 'UserDefinedNetwork' && createNewNs && newNamespaceName) {
          const nsResp = await apiClient.createNamespace({
            name: newNamespaceName,
            labels: { 'k8s.ovn.org/primary-user-defined-network': '' },
          });
          if (nsResp.error) {
            setIsCreating(false);
            setCreateError(`Failed to create namespace: ${nsResp.error.message}`);
            return;
          }
          effectiveNamespace = newNamespaceName;
        }

        // CUDN: create any target namespaces that don't exist yet (they were added via "Create new")
        if (selectedCombination.scope === 'ClusterUserDefinedNetwork') {
          const newNs = targetNamespaces.filter((ns) => !allNamespaces.includes(ns));
          for (const ns of newNs) {
            const nsResp = await apiClient.createNamespace({
              name: ns,
              labels: { 'k8s.ovn.org/primary-user-defined-network': '' },
            });
            if (nsResp.error) {
              setIsCreating(false);
              setCreateError(`Failed to create namespace "${ns}": ${nsResp.error.message}`);
              return;
            }
          }
        }
      }

      // Create VPC address prefix first if user opted in
      if (createPrefix && selectedCombination.requires_vpc && cidr && zone) {
        const prefixResp = await apiClient.createAddressPrefix({
          vpcId: vpcId,
          cidr: cidr,
          zone: zone,
          name: `${name}-prefix`,
        });
        if (prefixResp.error) {
          setIsCreating(false);
          setCreateError(`Failed to create address prefix: ${prefixResp.error.message}`);
          return;
        }
      }

      const req: CreateNetworkRequest = {
        name,
        topology: selectedCombination.topology,
        role: selectedCombination.role,
      };

      if (selectedCombination.scope === 'UserDefinedNetwork') {
        req.namespace = effectiveNamespace;
      }

      if (selectedCombination.scope === 'ClusterUserDefinedNetwork' && targetNamespaces.length > 0) {
        req.target_namespaces = targetNamespaces;
      }

      if (selectedCombination.requires_vpc) {
        req.vpc_id = vpcId;
        req.zone = zone;
        req.cidr = cidr;
        req.vlan_id = vlanId;
        if (securityGroupIds.length > 0) {
          req.security_group_ids = securityGroupIds.join(',');
        }
        if (aclId) {
          req.acl_id = aclId;
        }
        if (publicGatewayId) {
          req.public_gateway_id = publicGatewayId;
        }
      } else if (cidr) {
        // Non-VPC network with a CIDR (e.g. Layer2 Primary needs subnets in CRD)
        req.cidr = cidr;
      }

      const response = selectedCombination.scope === 'UserDefinedNetwork'
        ? await apiClient.createUDN(req)
        : await apiClient.createCUDN(req);

      setIsCreating(false);
      if (response.error) {
        const msg = response.error.message || response.error.code || 'Creation failed';
        setCreateError(typeof msg === 'string' ? msg : JSON.stringify(msg));
        return;
      }

      resetForm();
      onCreated();
    } catch (e) {
      setIsCreating(false);
      if (e instanceof Error) {
        setCreateError(e.message);
      } else if (typeof e === 'string') {
        setCreateError(e);
      } else {
        try {
          setCreateError(JSON.stringify(e));
        } catch {
          setCreateError('An unexpected error occurred');
        }
      }
    }
  };

  const isNameValid = name.length > 0 && /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(name);
  const isNewNsNameValid = !createNewNs || (newNamespaceName.length > 0 && /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(newNamespaceName));
  const isNamespaceValid = selectedCombination?.scope !== 'UserDefinedNetwork' || (createNewNs ? newNamespaceName.length > 0 : namespace.length > 0);
  // Layer2 Primary requires a CIDR (subnets are mandatory in the CRD) and Step 3 (VPC Config) is hidden
  const needsCidrInStep2 = selectedCombination && !selectedCombination.requires_vpc && selectedCombination.role === 'Primary';
  const isCidrValidForStep2 = !needsCidrInStep2 || cidr.length > 0;
  const isStep2Valid = isNameValid && isNamespaceValid && isNewNsNameValid && isCidrValidForStep2;

  // CIDR must fit within a VPC address prefix unless the user opted to create one
  const cidrFitsPrefix = !addressPrefixes?.length || createPrefix || cidrFitsAnyPrefix(cidr, addressPrefixes);
  const cidrOutsidePrefix = cidr.length > 0 && addressPrefixes && addressPrefixes.length > 0 && !cidrFitsAnyPrefix(cidr, addressPrefixes);

  // VLAN ID validation
  const parsedVlanId = parseInt(vlanId, 10);
  const vlanIdValid = vlanId.length > 0 && !isNaN(parsedVlanId) && parsedVlanId >= 1 && parsedVlanId <= 4094;
  const vlanIdInUse = vlanIdValid && usedVlanIds.has(parsedVlanId);
  const vlanOk = vlanIdValid && !vlanIdInUse;

  const isStep3Valid = !selectedCombination?.requires_vpc || (
    vpcId.length > 0 && zone.length > 0 && cidr.length > 0 && vlanOk && cidrFitsPrefix
  );

  if (!isOpen) return null;

  return (
    <Modal
      variant={ModalVariant.large}
      title="Create Network"
      isOpen={isOpen}
      onClose={handleClose}
      showClose
      hasNoBodyWrapper
    >
      {typesLoading ? (
        <div style={{ padding: '48px', textAlign: 'center' }}>
          <Spinner size="xl" />
        </div>
      ) : (
        <Wizard onClose={handleClose} height={500}>
          {/* Step 1: Select Network Type */}
          <WizardStep name="Select Type" id="select-type" footer={{ isNextDisabled: !selectedCombination }}>
            <div style={{ padding: '16px' }}>
              <Text component={TextVariants.p} style={{ marginBottom: '16px' }}>
                Answer the questions below to choose the right network type, or expand the advanced section to pick directly.
              </Text>

              {/* Q1: What kind of network connectivity? */}
              <div data-testid="guided-q1" style={{ marginBottom: '24px' }}>
                <Title headingLevel="h4" style={{ marginBottom: '8px' }}>
                  What kind of network connectivity?
                </Title>
                <div style={{ display: 'flex', gap: '16px' }}>
                  <Card
                    isSelectable={!isROKS}
                    isSelected={selectedTopology === 'LocalNet'}
                    onClick={isROKS ? undefined : () => handleTopologySelect('LocalNet')}
                    data-testid="option-vpc-routable"
                    style={{
                      flex: 1,
                      cursor: isROKS ? 'not-allowed' : 'pointer',
                      border: selectedTopology === 'LocalNet' ? selectedBorder : undefined,
                      opacity: isROKS ? 0.6 : undefined,
                    }}
                  >
                    <CardTitle>
                      VPC-Routable
                      {isROKS && <Label isCompact color="orange" style={{ marginLeft: '8px' }}>Coming soon</Label>}
                    </CardTitle>
                    <CardBody>
                      <Text component={TextVariants.small}>
                        {isROKS
                          ? 'Requires ROKS platform API for VNI and VLAN attachment management. Coming in a future release.'
                          : 'Backed by a VPC subnet. VMs get static reserved IPs. Requires VPC configuration.'}
                      </Text>
                      {!isROKS && <Label isCompact color="blue" style={{ marginTop: '8px' }}>IP mode: Static Reserved</Label>}
                    </CardBody>
                  </Card>
                  <Card
                    isSelectable
                    isSelected={selectedTopology === 'Layer2'}
                    onClick={() => handleTopologySelect('Layer2')}
                    data-testid="option-cluster-internal"
                    style={{
                      flex: 1,
                      cursor: 'pointer',
                      border: selectedTopology === 'Layer2' ? selectedBorder : undefined,
                    }}
                  >
                    <CardTitle>Cluster-Internal Only</CardTitle>
                    <CardBody>
                      <Text component={TextVariants.small}>
                        OVN Layer2 network. VMs get IPs via DHCP. No VPC resources needed.
                      </Text>
                      <Label isCompact color="green" style={{ marginTop: '8px' }}>IP mode: DHCP</Label>
                    </CardBody>
                  </Card>
                </div>
              </div>

              {/* Q2: What scope? (appears after Q1 answered) */}
              {selectedTopology && (
                <div data-testid="guided-q2" style={{ marginBottom: '24px' }}>
                  <Title headingLevel="h4" style={{ marginBottom: '8px' }}>
                    What scope?
                    {availableScopes.length === 1 && (
                      <Label isCompact color="blue" style={{ marginLeft: '8px' }}>Auto-selected</Label>
                    )}
                  </Title>
                  <div style={{ display: 'flex', gap: '16px' }}>
                    <Card
                      isSelectable
                      isSelected={selectedScope === 'ClusterUserDefinedNetwork'}
                      isDisabled={!availableScopes.includes('ClusterUserDefinedNetwork')}
                      onClick={() => availableScopes.includes('ClusterUserDefinedNetwork') && handleScopeSelect('ClusterUserDefinedNetwork')}
                      data-testid="option-cluster-wide"
                      style={{
                        flex: 1,
                        cursor: availableScopes.includes('ClusterUserDefinedNetwork') ? 'pointer' : 'not-allowed',
                        border: selectedScope === 'ClusterUserDefinedNetwork' ? selectedBorder : undefined,
                        opacity: availableScopes.includes('ClusterUserDefinedNetwork') ? 1 : 0.5,
                      }}
                    >
                      <CardTitle>Cluster-wide</CardTitle>
                      <CardBody>
                        <Text component={TextVariants.small}>
                          Available in all namespaces (ClusterUserDefinedNetwork).
                        </Text>
                      </CardBody>
                    </Card>
                    <Card
                      isSelectable
                      isSelected={selectedScope === 'UserDefinedNetwork'}
                      isDisabled={!availableScopes.includes('UserDefinedNetwork')}
                      onClick={() => availableScopes.includes('UserDefinedNetwork') && handleScopeSelect('UserDefinedNetwork')}
                      data-testid="option-namespace-scoped"
                      style={{
                        flex: 1,
                        cursor: availableScopes.includes('UserDefinedNetwork') ? 'pointer' : 'not-allowed',
                        border: selectedScope === 'UserDefinedNetwork' ? selectedBorder : undefined,
                        opacity: availableScopes.includes('UserDefinedNetwork') ? 1 : 0.5,
                      }}
                    >
                      <CardTitle>Namespace-scoped</CardTitle>
                      <CardBody>
                        <Text component={TextVariants.small}>
                          Isolated to a single namespace (UserDefinedNetwork).
                        </Text>
                        {!availableScopes.includes('UserDefinedNetwork') && selectedTopology === 'LocalNet' && (
                          <Text component={TextVariants.small} style={{ marginTop: '4px', fontStyle: 'italic' }}>
                            UDN does not support LocalNet topology.
                          </Text>
                        )}
                      </CardBody>
                    </Card>
                  </div>
                </div>
              )}

              {/* Q3: What role? (appears after Q2 answered) */}
              {selectedScope && (
                <div data-testid="guided-q3" style={{ marginBottom: '24px' }}>
                  <Title headingLevel="h4" style={{ marginBottom: '8px' }}>
                    What role?
                    {availableRoles.length === 1 && (
                      <Label isCompact color="blue" style={{ marginLeft: '8px' }}>Auto-selected</Label>
                    )}
                  </Title>
                  <div style={{ display: 'flex', gap: '16px' }}>
                    <Card
                      isSelectable
                      isSelected={selectedRole === 'Secondary'}
                      isDisabled={!availableRoles.includes('Secondary')}
                      onClick={() => availableRoles.includes('Secondary') && handleRoleSelect('Secondary')}
                      data-testid="option-secondary"
                      style={{
                        flex: 1,
                        cursor: availableRoles.includes('Secondary') ? 'pointer' : 'not-allowed',
                        border: selectedRole === 'Secondary' ? selectedBorder : undefined,
                        opacity: availableRoles.includes('Secondary') ? 1 : 0.5,
                      }}
                    >
                      <CardTitle>
                        <span>Secondary</span>{' '}
                        <Label color="green" isCompact>Recommended</Label>
                      </CardTitle>
                      <CardBody>
                        <Text component={TextVariants.small}>
                          Additional network alongside the default pod network.
                        </Text>
                      </CardBody>
                    </Card>
                    <Card
                      isSelectable
                      isSelected={selectedRole === 'Primary'}
                      isDisabled={!availableRoles.includes('Primary')}
                      onClick={() => availableRoles.includes('Primary') && handleRoleSelect('Primary')}
                      data-testid="option-primary"
                      style={{
                        flex: 1,
                        cursor: availableRoles.includes('Primary') ? 'pointer' : 'not-allowed',
                        border: selectedRole === 'Primary' ? selectedBorder : undefined,
                        opacity: availableRoles.includes('Primary') ? 1 : 0.5,
                      }}
                    >
                      <CardTitle>
                        <span>Primary</span>{' '}
                        <Label color="orange" isCompact>Caution</Label>
                      </CardTitle>
                      <CardBody>
                        <Text component={TextVariants.small}>
                          Replaces the default pod network. Affects all workloads in scope.
                        </Text>
                        {!availableRoles.includes('Primary') && (
                          <Text component={TextVariants.small} style={{ marginTop: '4px', fontStyle: 'italic' }}>
                            Not available for this topology/scope combination.
                          </Text>
                        )}
                      </CardBody>
                    </Card>
                  </div>
                  {selectedRole === 'Primary' && (
                    <Alert
                      variant="warning"
                      isInline
                      title="Primary networks replace the default pod network"
                      style={{ marginTop: '8px' }}
                    >
                      All pods in the affected scope will use this network instead of the default cluster network.
                      Only use Primary if you understand the implications.
                    </Alert>
                  )}
                </div>
              )}

              {/* Summary confirmation box (shown when all 3 questions answered) */}
              {selectedCombination && (
                <div
                  data-testid="guided-summary"
                  style={{
                    padding: '16px',
                    border: '1px solid var(--pf-v5-global--BorderColor--100)',
                    borderRadius: '8px',
                    backgroundColor: 'var(--pf-v5-global--BackgroundColor--200)',
                    marginBottom: '24px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                    <Title headingLevel="h4">{selectedCombination.label}</Title>
                    <TierBadge tier={selectedCombination.tier} />
                  </div>
                  <Text component={TextVariants.small} style={{ marginBottom: '8px' }}>
                    {selectedCombination.description}
                  </Text>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '8px' }}>
                    <Label isCompact color="blue">{selectedCombination.topology}</Label>
                    <Label isCompact>{ipModeLabels[selectedCombination.ip_mode] || selectedCombination.ip_mode}</Label>
                    {selectedCombination.requires_vpc && <Label isCompact color="purple">VPC</Label>}
                  </div>
                  <IPModeInfoAlert
                    mode={selectedCombination.ip_mode}
                    description={selectedCombination.ip_mode_description}
                  />
                </div>
              )}

              {/* Advanced: Direct card selection */}
              <Divider style={{ marginBottom: '16px' }} />
              <ExpandableSection
                toggleText={advancedExpanded ? 'Hide all network types' : 'I know what I need \u2014 show all network types'}
                isExpanded={advancedExpanded}
                onToggle={(_e, expanded) => setAdvancedExpanded(expanded)}
                data-testid="advanced-section"
              >
                {(['recommended', 'advanced', 'expert'] as NetworkTier[]).map((tier) => {
                  const items = grouped[tier];
                  if (!items || items.length === 0) return null;
                  return (
                    <div key={tier} style={{ marginBottom: '24px' }}>
                      <Title headingLevel="h4" style={{ marginBottom: '8px' }}>
                        {tierLabels[tier]}
                      </Title>
                      <Gallery hasGutter minWidths={{ default: '280px' }}>
                        {items.map((combo) => {
                          const isSelected = selectedCombination?.id === combo.id;
                          return (
                            <GalleryItem key={combo.id}>
                              <Card
                                isSelectable
                                isSelected={isSelected}
                                onClick={() => handleCardSelect(combo)}
                                data-testid={`card-${combo.id}`}
                                style={{
                                  cursor: 'pointer',
                                  border: isSelected ? selectedBorder : undefined,
                                }}
                              >
                                <CardTitle>
                                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                    <span>{combo.label}</span>
                                    <TierBadge tier={combo.tier} />
                                  </div>
                                </CardTitle>
                                <CardBody>
                                  <Text component={TextVariants.small} style={{ marginBottom: '8px' }}>
                                    {combo.description}
                                  </Text>
                                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                                    <Label isCompact color="blue">{combo.topology}</Label>
                                    <Label isCompact>{ipModeLabels[combo.ip_mode] || combo.ip_mode}</Label>
                                    {combo.requires_vpc && <Label isCompact color="purple">VPC</Label>}
                                  </div>
                                </CardBody>
                              </Card>
                            </GalleryItem>
                          );
                        })}
                      </Gallery>
                    </div>
                  );
                })}
              </ExpandableSection>
            </div>
          </WizardStep>

          {/* Step 2: Name & Namespace */}
          <WizardStep name="Name & Namespace" id="name-namespace" footer={{ isNextDisabled: !isStep2Valid }}>
            <div style={{ padding: '16px' }}>
              {selectedCombination && (
                <Alert
                  variant="info"
                  isInline
                  isPlain
                  title={`${selectedCombination.label} — IP mode: ${ipModeLabels[selectedCombination.ip_mode] || selectedCombination.ip_mode}`}
                  style={{ marginBottom: '16px' }}
                />
              )}
              <Form>
                <FormGroup label="Network Name" isRequired fieldId="network-name">
                  <TextInput
                    id="network-name"
                    value={name}
                    onChange={(_e, val) => setName(val)}
                    placeholder="my-network"
                    isRequired
                    validated={name.length === 0 ? 'default' : isNameValid ? 'success' : 'error'}
                  />
                  <FormHelperText>
                    <HelperText>
                      <HelperTextItem variant={name.length > 0 && !isNameValid ? 'error' : 'default'}>
                        Must be lowercase alphanumeric with optional hyphens.
                      </HelperTextItem>
                    </HelperText>
                  </FormHelperText>
                </FormGroup>

                {selectedCombination?.scope === 'UserDefinedNetwork' && (
                  <>
                    {selectedCombination.role === 'Primary' && !createNewNs && (
                      <Alert variant="info" isInline title="Primary UDN requires namespace label" style={{ marginBottom: '16px' }}>
                        Only namespaces created with the <code>k8s.ovn.org/primary-user-defined-network</code> label are shown. This label is immutable after creation.
                      </Alert>
                    )}
                    {!createNewNs ? (
                      <FormGroup label="Namespace" isRequired fieldId="namespace">
                        <FormSelect
                          id="namespace"
                          value={namespace}
                          onChange={(_e, val) => {
                            if (val === '__create_new__') {
                              setCreateNewNs(true);
                              setNamespace('');
                            } else {
                              setNamespace(val);
                            }
                          }}
                        >
                          <FormSelectOption value="" label="Select a namespace" isPlaceholder />
                          {selectedCombination.role === 'Primary' && (
                            <FormSelectOption value="__create_new__" label="+ Create new namespace (with primary label)" />
                          )}
                          {allNamespaceInfos
                            .filter((ns) => selectedCombination.role !== 'Primary' || ns.hasPrimaryLabel)
                            .map((ns) => (
                              <FormSelectOption key={ns.name} value={ns.name} label={ns.name} />
                            ))}
                        </FormSelect>
                        {selectedCombination.role === 'Primary' && allNamespaceInfos.filter((ns) => ns.hasPrimaryLabel).length === 0 && (
                          <FormHelperText>
                            <HelperText>
                              <HelperTextItem variant="warning">
                                No namespaces with the primary label exist. Create a new one below.
                              </HelperTextItem>
                            </HelperText>
                          </FormHelperText>
                        )}
                      </FormGroup>
                    ) : (
                      <FormGroup label="New Namespace Name" isRequired fieldId="new-namespace">
                        <TextInput
                          id="new-namespace"
                          value={newNamespaceName}
                          onChange={(_e, val) => setNewNamespaceName(val)}
                          placeholder="my-namespace"
                          isRequired
                          validated={newNamespaceName.length === 0 ? 'default' : isNewNsNameValid ? 'success' : 'error'}
                        />
                        <FormHelperText>
                          <HelperText>
                            <HelperTextItem>
                              A new namespace will be created with the <code>k8s.ovn.org/primary-user-defined-network</code> label.
                            </HelperTextItem>
                          </HelperText>
                        </FormHelperText>
                        <Button variant="link" size="sm" onClick={() => { setCreateNewNs(false); setNewNamespaceName(''); }} style={{ marginTop: '4px' }}>
                          Select existing namespace instead
                        </Button>
                      </FormGroup>
                    )}
                  </>
                )}

                {selectedCombination?.scope === 'ClusterUserDefinedNetwork' && (
                  <FormGroup
                    label="Target Namespaces"
                    fieldId="target-namespaces"
                  >
                    <FormHelperText>
                      <HelperText>
                        <HelperTextItem>
                          {targetNamespaces.length === 0
                            ? 'No namespaces selected — NADs will be created in ALL namespaces. Select specific namespaces to restrict.'
                            : `NADs will be created in ${targetNamespaces.length} namespace(s).`}
                        </HelperTextItem>
                      </HelperText>
                    </FormHelperText>
                    {selectedCombination.role === 'Primary' && (
                      <Alert variant="warning" isInline isPlain title="Primary CUDN requires labeled namespaces" style={{ marginBottom: '8px', marginTop: '8px' }}>
                        Namespaces must have <code>k8s.ovn.org/primary-user-defined-network</code> label (set at creation time). Namespaces without the label are marked below.
                      </Alert>
                    )}
                    {selectedCombination.role === 'Primary' && (
                      <div style={{ marginBottom: '8px', marginTop: '8px' }}>
                        {!createNewNs ? (
                          <Button variant="link" size="sm" onClick={() => setCreateNewNs(true)}>
                            + Create new namespace with primary label
                          </Button>
                        ) : (
                          <div style={{ display: 'flex', gap: '8px', alignItems: 'flex-end' }}>
                            <TextInput
                              id="cudn-new-ns"
                              value={newNamespaceName}
                              onChange={(_e, val) => setNewNamespaceName(val)}
                              placeholder="new-namespace-name"
                              style={{ maxWidth: '300px' }}
                            />
                            <Button
                              variant="secondary"
                              size="sm"
                              isDisabled={!newNamespaceName || !/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(newNamespaceName)}
                              onClick={() => {
                                // Will be created on submit; add to targets for now
                                if (newNamespaceName && !targetNamespaces.includes(newNamespaceName)) {
                                  setTargetNamespaces((prev) => [...prev, newNamespaceName]);
                                }
                                setNewNamespaceName('');
                                setCreateNewNs(false);
                              }}
                            >
                              Add
                            </Button>
                            <Button variant="link" size="sm" onClick={() => { setCreateNewNs(false); setNewNamespaceName(''); }}>
                              Cancel
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                    <TextInputGroup style={{ marginBottom: '8px', marginTop: '8px' }}>
                      <TextInputGroupMain
                        placeholder="Filter namespaces..."
                        value={nsFilter}
                        onChange={(_e, val) => setNsFilter(val)}
                      />
                    </TextInputGroup>
                    <div style={{ maxHeight: '200px', overflow: 'auto', border: '1px solid var(--pf-v5-global--BorderColor--100)', borderRadius: '4px', padding: '8px' }}>
                      {allNamespaceInfos
                        .filter((ns) => !nsFilter || ns.name.includes(nsFilter))
                        .map((ns) => {
                          const isPrimary = selectedCombination.role === 'Primary';
                          const labelSuffix = isPrimary
                            ? ns.hasPrimaryLabel ? ' \u2713' : ' \u26a0 no primary label'
                            : '';
                          return (
                            <Checkbox
                              key={ns.name}
                              id={`ns-${ns.name}`}
                              label={`${ns.name}${labelSuffix}`}
                              isChecked={targetNamespaces.includes(ns.name)}
                              onChange={(_e, checked) => {
                                setTargetNamespaces((prev) =>
                                  checked ? [...prev, ns.name] : prev.filter((n) => n !== ns.name),
                                );
                              }}
                            />
                          );
                        })}
                      {/* Show namespaces that were added via "Create new" but don't exist yet */}
                      {targetNamespaces
                        .filter((ns) => !allNamespaces.includes(ns))
                        .map((ns) => (
                          <Checkbox
                            key={ns}
                            id={`ns-${ns}`}
                            label={`${ns} (new — will be created)`}
                            isChecked
                            onChange={(_e, checked) => {
                              if (!checked) {
                                setTargetNamespaces((prev) => prev.filter((n) => n !== ns));
                              }
                            }}
                          />
                        ))}
                    </div>
                    {selectedCombination.role === 'Primary' && targetNamespaces.length > 0 && (() => {
                      const unlabeled = targetNamespaces.filter((ns) => {
                        const info = allNamespaceInfos.find((n) => n.name === ns);
                        // New namespaces (not in allNamespaceInfos) will be created with label
                        return info && !info.hasPrimaryLabel;
                      });
                      if (unlabeled.length === 0) return null;
                      return (
                        <Alert variant="warning" isInline title="Namespace label warning" style={{ marginTop: '8px' }}>
                          These namespaces were not created with Primary network support and NADs will not be generated for them: <strong>{unlabeled.join(', ')}</strong>
                        </Alert>
                      );
                    })()}
                  </FormGroup>
                )}

                {/* CIDR for Layer2 Primary (non-VPC networks where subnets are required in the CRD) */}
                {needsCidrInStep2 && (
                  <FormGroup label="Subnet CIDR" isRequired fieldId="layer2-cidr-input">
                    <TextInput
                      id="layer2-cidr-input"
                      value={cidr}
                      onChange={(_e, val) => setCidr(val)}
                      placeholder="10.222.0.0/24"
                      isRequired
                    />
                    <FormHelperText>
                      <HelperText>
                        <HelperTextItem>
                          OVN will assign IPs from this subnet with persistent IPAM so VM addresses survive restarts.
                        </HelperTextItem>
                      </HelperText>
                    </FormHelperText>
                  </FormGroup>
                )}

                {selectedCombination && (
                  <div style={{ marginTop: '16px' }}>
                    <IPModeInfoAlert
                      mode={selectedCombination.ip_mode}
                      description={selectedCombination.ip_mode_description}
                    />
                  </div>
                )}
              </Form>
            </div>
          </WizardStep>

          {/* Step 3: VPC Configuration (conditional) */}
          <WizardStep
            name="VPC Configuration"
            id="vpc-config"
            isHidden={!selectedCombination?.requires_vpc}
            footer={{ isNextDisabled: !isStep3Valid }}
          >
            <div style={{ padding: '16px' }}>
              <Form>
                <FormGroup label="VPC" isRequired fieldId="vpc-select">
                  <FormSelect
                    id="vpc-select"
                    value={vpcId}
                    onChange={(_e, val) => setVpcId(val)}
                  >
                    <FormSelectOption value="" label="Select a VPC" isPlaceholder />
                    {(vpcs || []).map((v) => (
                      <FormSelectOption key={v.id} value={v.id} label={v.name || v.id} />
                    ))}
                  </FormSelect>
                </FormGroup>

                <FormGroup label="Zone" isRequired fieldId="zone-select">
                  <FormSelect
                    id="zone-select"
                    value={zone}
                    onChange={(_e, val) => setZone(val)}
                  >
                    <FormSelectOption value="" label="Select a zone" isPlaceholder />
                    {(zones || []).map((z) => (
                      <FormSelectOption key={z.name} value={z.name} label={z.name || ''} />
                    ))}
                  </FormSelect>
                </FormGroup>

                <FormGroup label="CIDR Block" isRequired fieldId="cidr-input">
                  <TextInput
                    id="cidr-input"
                    value={cidr}
                    onChange={(_e, val) => { setCidr(val); setCreatePrefix(false); }}
                    placeholder="10.240.0.0/24"
                    isRequired
                    validated={cidrOutsidePrefix && !createPrefix ? 'error' : 'default'}
                  />
                  {vpcId && (
                    <FormHelperText>
                      <HelperText>
                        {cidrOutsidePrefix && !createPrefix ? (
                          <HelperTextItem variant="error">
                            CIDR does not fit within any VPC address prefix. Choose a CIDR within one of the listed ranges, or check the box below to create a new prefix.
                          </HelperTextItem>
                        ) : (
                          <HelperTextItem>
                            {prefixesLoading ? (
                              'Loading address prefixes...'
                            ) : !addressPrefixes?.length ? (
                              'No address prefixes found for this VPC.'
                            ) : (
                              <>
                                CIDR must fit within a VPC address prefix:{' '}
                                {addressPrefixes.map((p, i) => (
                                  <span key={p.cidr}>
                                    {i > 0 && ', '}
                                    <code>{p.cidr}</code>
                                    <small> ({p.zone})</small>
                                  </span>
                                ))}
                              </>
                            )}
                          </HelperTextItem>
                        )}
                      </HelperText>
                    </FormHelperText>
                  )}
                  {cidrOutsidePrefix && (
                    <div style={{ marginTop: '8px' }}>
                      <Checkbox
                        id="create-prefix-checkbox"
                        label="Create a VPC address prefix for this CIDR"
                        isChecked={createPrefix}
                        onChange={(_e, checked) => setCreatePrefix(checked)}
                      />
                      {createPrefix && zone && (
                        <Alert variant="info" isInline isPlain title="" style={{ marginTop: '8px' }}>
                          A new VPC address prefix <code>{cidr}</code> will be created in zone <strong>{zone}</strong> before the network is created.
                        </Alert>
                      )}
                      {createPrefix && !zone && (
                        <FormHelperText>
                          <HelperText>
                            <HelperTextItem variant="warning">
                              Select a zone above before creating the prefix.
                            </HelperTextItem>
                          </HelperText>
                        </FormHelperText>
                      )}
                    </div>
                  )}
                </FormGroup>

                <FormGroup label="VLAN ID" isRequired fieldId="vlan-id-input">
                  <TextInput
                    id="vlan-id-input"
                    value={vlanId}
                    onChange={(_e, val) => setVlanId(val)}
                    placeholder={`e.g. ${suggestedVlanId}`}
                    isRequired
                    validated={vlanId.length === 0 ? 'default' : vlanOk ? 'success' : 'error'}
                  />
                  <FormHelperText>
                    <HelperText>
                      {vlanId.length > 0 && !vlanIdValid ? (
                        <HelperTextItem variant="error">
                          VLAN ID must be a number between 1 and 4094.
                        </HelperTextItem>
                      ) : vlanIdInUse ? (
                        <HelperTextItem variant="error">
                          VLAN ID {parsedVlanId} is already in use by another network.
                        </HelperTextItem>
                      ) : (
                        <HelperTextItem>
                          Valid range: 1–4094.
                          {usedVlanIds.size > 0 && ` In use: ${[...usedVlanIds].sort((a, b) => a - b).join(', ')}.`}
                          {vlanId.length === 0 && ` Suggested: ${suggestedVlanId}.`}
                        </HelperTextItem>
                      )}
                    </HelperText>
                  </FormHelperText>
                </FormGroup>

                <FormGroup label="Security Groups" fieldId="sg-select">
                  <FormSelect
                    id="sg-select"
                    value={securityGroupIds[0] || ''}
                    onChange={(_e, val) => setSecurityGroupIds(val ? [val] : [])}
                  >
                    <FormSelectOption value="" label="None (optional)" isPlaceholder />
                    {(securityGroups || []).map((sg) => (
                      <FormSelectOption key={sg.id} value={sg.id} label={sg.name || sg.id} />
                    ))}
                  </FormSelect>
                </FormGroup>

                <FormGroup label="Network ACL" fieldId="acl-select">
                  <FormSelect
                    id="acl-select"
                    value={aclId}
                    onChange={(_e, val) => setAclId(val)}
                  >
                    <FormSelectOption value="" label="None (optional)" isPlaceholder />
                    {(networkAcls || []).map((acl) => (
                      <FormSelectOption key={acl.id} value={acl.id} label={acl.name || acl.id} />
                    ))}
                  </FormSelect>
                </FormGroup>

                <FormGroup label="Public Gateway" fieldId="pgw-select">
                  <FormSelect
                    id="pgw-select"
                    value={publicGatewayId}
                    onChange={(_e, val) => setPublicGatewayId(val)}
                  >
                    <FormSelectOption value="" label="None (private only)" isPlaceholder />
                    {(publicGateways || []).map((pgw) => (
                      <FormSelectOption key={pgw.id} value={pgw.id} label={`${pgw.name} (${pgw.zone?.name || pgw.zone})`} />
                    ))}
                  </FormSelect>
                  <FormHelperText>
                    <HelperText>
                      <HelperTextItem>Optional: provides outbound internet for VMs without per-VM floating IPs</HelperTextItem>
                    </HelperText>
                  </FormHelperText>
                </FormGroup>
              </Form>
            </div>
          </WizardStep>

          {/* Step 4: Review & Create */}
          <WizardStep
            name="Review"
            id="review"
            footer={{
              nextButtonText: 'Create',
              onNext: handleCreate,
              isNextDisabled: isCreating,
            }}
          >
            <div style={{ padding: '16px' }}>
              {createError && (
                <Alert variant="danger" isInline title="Creation failed" style={{ marginBottom: '16px' }}>
                  {createError}
                </Alert>
              )}

              {isCreating ? (
                <div style={{ textAlign: 'center', padding: '32px' }}>
                  <Spinner size="lg" />
                  <Text component={TextVariants.p} style={{ marginTop: '8px' }}>Creating network...</Text>
                </div>
              ) : (
                <>
                  <DescriptionList isHorizontal>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Network Type</DescriptionListTerm>
                      <DescriptionListDescription>
                        {selectedCombination?.label}
                        {selectedCombination && (
                          <span style={{ marginLeft: '8px' }}>
                            <TierBadge tier={selectedCombination.tier} />
                          </span>
                        )}
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Topology</DescriptionListTerm>
                      <DescriptionListDescription>{selectedCombination?.topology}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Scope</DescriptionListTerm>
                      <DescriptionListDescription>
                        {selectedCombination?.scope === 'ClusterUserDefinedNetwork' ? 'Cluster-wide' : 'Namespace-scoped'}
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Role</DescriptionListTerm>
                      <DescriptionListDescription>{selectedCombination?.role}</DescriptionListDescription>
                    </DescriptionListGroup>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Name</DescriptionListTerm>
                      <DescriptionListDescription>{name}</DescriptionListDescription>
                    </DescriptionListGroup>
                    {selectedCombination?.scope === 'UserDefinedNetwork' && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Namespace</DescriptionListTerm>
                        <DescriptionListDescription>
                          {createNewNs ? `${newNamespaceName} (new)` : namespace}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {selectedCombination?.scope === 'ClusterUserDefinedNetwork' && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Target Namespaces</DescriptionListTerm>
                        <DescriptionListDescription>
                          {targetNamespaces.length === 0
                            ? 'All namespaces'
                            : targetNamespaces.join(', ')}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {cidr && !selectedCombination?.requires_vpc && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Subnet CIDR</DescriptionListTerm>
                        <DescriptionListDescription>{cidr}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                  </DescriptionList>

                  {selectedCombination?.requires_vpc && (
                    <>
                      <Divider style={{ marginTop: '16px', marginBottom: '16px' }} />
                      <Title headingLevel="h4" style={{ marginBottom: '8px' }}>VPC Configuration</Title>
                      <DescriptionList isHorizontal>
                        <DescriptionListGroup>
                          <DescriptionListTerm>VPC</DescriptionListTerm>
                          <DescriptionListDescription>
                            {vpcs?.find((v) => v.id === vpcId)?.name || vpcId}
                          </DescriptionListDescription>
                        </DescriptionListGroup>
                        <DescriptionListGroup>
                          <DescriptionListTerm>Zone</DescriptionListTerm>
                          <DescriptionListDescription>{zone}</DescriptionListDescription>
                        </DescriptionListGroup>
                        <DescriptionListGroup>
                          <DescriptionListTerm>CIDR</DescriptionListTerm>
                          <DescriptionListDescription>{cidr}</DescriptionListDescription>
                        </DescriptionListGroup>
                        <DescriptionListGroup>
                          <DescriptionListTerm>VLAN ID</DescriptionListTerm>
                          <DescriptionListDescription>{vlanId}</DescriptionListDescription>
                        </DescriptionListGroup>
                        {securityGroupIds.length > 0 && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>Security Groups</DescriptionListTerm>
                            <DescriptionListDescription>{securityGroupIds.join(', ')}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                        {aclId && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>Network ACL</DescriptionListTerm>
                            <DescriptionListDescription>
                              {networkAcls?.find((a) => a.id === aclId)?.name || aclId}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                        {publicGatewayId && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>Public Gateway</DescriptionListTerm>
                            <DescriptionListDescription>
                              {publicGateways?.find((p) => p.id === publicGatewayId)?.name || publicGatewayId}
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                      </DescriptionList>
                    </>
                  )}

                  {selectedCombination && (
                    <div style={{ marginTop: '16px' }}>
                      <IPModeInfoAlert
                        mode={selectedCombination.ip_mode}
                        description={selectedCombination.ip_mode_description}
                      />
                    </div>
                  )}
                </>
              )}
            </div>
          </WizardStep>
        </Wizard>
      )}
    </Modal>
  );
};

NetworkCreationWizard.displayName = 'NetworkCreationWizard';
export default NetworkCreationWizard;
