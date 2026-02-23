import React, { useState, useCallback } from 'react';
import {
  Button,
  ButtonVariant,
  Modal,
  ModalVariant,
  Alert,
  AlertVariant,
  EmptyState,
  EmptyStateBody,
  EmptyStateIcon,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { PlusCircleIcon, ExclamationTriangleIcon } from '@patternfly/react-icons';
import { RuleForm, ResourceType, RuleFormValues } from './RuleForm';
import { apiClient } from '../api/client';
import { SecurityGroupRule, NetworkACLRule } from '../api/types';

export type Rule = SecurityGroupRule | NetworkACLRule;

interface RuleEditorProps {
  resourceType: ResourceType;
  resourceId: string;
  rules: Rule[];
  onRuleAdded?: (rule: Rule) => void;
  onRuleUpdated?: (rule: Rule) => void;
  onRuleDeleted?: (ruleId: string) => void;
  readOnly?: boolean;
}

interface DeleteConfirmation {
  open: boolean;
  ruleId?: string;
  ruleDescription?: string;
}

const isSecurityGroupRule = (rule: Rule): rule is SecurityGroupRule => {
  return !('action' in rule);
};

const isNetworkACLRule = (rule: Rule): rule is NetworkACLRule => {
  return 'action' in rule;
};

const getPortDisplay = (rule: Rule): string => {
  if (rule.protocol === 'icmp') {
    return `Type: ${rule.icmpType ?? '-'}, Code: ${rule.icmpCode ?? '-'}`;
  }

  if (rule.protocol === 'tcp' || rule.protocol === 'udp') {
    if (rule.portMin === rule.portMax) {
      return `${rule.portMin}`;
    }
    return `${rule.portMin}-${rule.portMax}`;
  }

  return '-';
};

const getRuleDescription = (rule: Rule): string => {
  if (isSecurityGroupRule(rule)) {
    const portInfo = rule.protocol === 'all' ? '' : ` ${getPortDisplay(rule)}`;
    return `${rule.direction} ${rule.protocol}${portInfo} from ${rule.remote ?? 'any'}`;
  } else {
    const portInfo = rule.protocol === 'all' ? '' : ` ${getPortDisplay(rule)}`;
    return `${rule.action} ${rule.protocol}${portInfo} from ${rule.source} to ${rule.destination}`;
  }
};

export const RuleEditor: React.FC<RuleEditorProps> = ({
  resourceType,
  resourceId,
  rules,
  onRuleAdded,
  onRuleUpdated,
  onRuleDeleted,
  readOnly = false,
}) => {
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<Rule | undefined>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteConfirmation, setDeleteConfirmation] = useState<DeleteConfirmation>({
    open: false,
  });

  const isACL = resourceType === 'network-acl';
  const isSG = resourceType === 'security-group';

  const handleAddRule = useCallback(() => {
    setEditingRule(undefined);
    setIsFormOpen(true);
  }, []);

  const handleEditRule = useCallback((rule: Rule) => {
    setEditingRule(rule);
    setIsFormOpen(true);
  }, []);

  const handleDeleteClick = useCallback((rule: Rule) => {
    setDeleteConfirmation({
      open: true,
      ruleId: rule.id,
      ruleDescription: getRuleDescription(rule),
    });
  }, []);

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteConfirmation.ruleId) return;

    setIsLoading(true);
    setError(null);

    try {
      if (resourceType === 'security-group') {
        await apiClient.deleteSecurityGroupRule(resourceId, deleteConfirmation.ruleId);
      } else {
        await apiClient.deleteNetworkACLRule(resourceId, deleteConfirmation.ruleId);
      }
      onRuleDeleted?.(deleteConfirmation.ruleId);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to delete rule'
      );
    } finally {
      setIsLoading(false);
      setDeleteConfirmation({ open: false });
    }
  }, [deleteConfirmation.ruleId, resourceId, resourceType, onRuleDeleted]);

  const handleFormSubmit = useCallback(
    async (values: RuleFormValues) => {
      setIsLoading(true);
      setError(null);

      try {
        const isEditing = !!editingRule;

        const payload: Record<string, unknown> = {
          direction: values.direction,
          protocol: values.protocol,
        };

        if (isSG) {
          payload.remote = values.remote;
          payload.remoteType = values.remoteType;
          if (values.protocol === 'tcp' || values.protocol === 'udp') {
            payload.portMin = values.portMin;
            payload.portMax = values.portMax;
          } else if (values.protocol === 'icmp') {
            payload.icmpType = values.icmpType;
            payload.icmpCode = values.icmpCode;
          }
        } else if (isACL) {
          payload.source = values.source;
          payload.destination = values.destination;
          payload.action = values.action;
          payload.priority = values.priority;
          if (values.protocol === 'tcp' || values.protocol === 'udp') {
            payload.portMin = values.portMin;
            payload.portMax = values.portMax;
          } else if (values.protocol === 'icmp') {
            payload.icmpType = values.icmpType;
            payload.icmpCode = values.icmpCode;
          }
        }

        let result;
        if (isSG) {
          if (isEditing && editingRule?.id) {
            const response = await apiClient.updateSecurityGroupRule(resourceId, editingRule.id, payload as any);
            result = response.data;
          } else {
            const response = await apiClient.addSecurityGroupRule(resourceId, payload as any);
            result = response.data;
          }
        } else {
          if (isEditing && editingRule?.id) {
            const response = await apiClient.updateNetworkACLRule(resourceId, editingRule.id, payload as any);
            result = response.data;
          } else {
            const response = await apiClient.addNetworkACLRule(resourceId, payload as any);
            result = response.data;
          }
        }

        if (result) {
          if (isEditing) {
            onRuleUpdated?.(result as Rule);
          } else {
            onRuleAdded?.(result as Rule);
          }
        }

        setIsFormOpen(false);
        setEditingRule(undefined);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to save rule');
      } finally {
        setIsLoading(false);
      }
    },
    [resourceType, resourceId, editingRule, isSG, isACL, onRuleAdded, onRuleUpdated]
  );

  const handleFormCancel = useCallback(() => {
    setIsFormOpen(false);
    setEditingRule(undefined);
  }, []);

  return (
    <div>
      {error && (
        <Alert variant={AlertVariant.danger} title="Error" isInline>
          {error}
        </Alert>
      )}

      <Toolbar>
        <ToolbarContent>
          <ToolbarItem>
            <Button
              icon={<PlusCircleIcon />}
              onClick={handleAddRule}
              isDisabled={readOnly || isLoading}
            >
              Add Rule
            </Button>
          </ToolbarItem>
        </ToolbarContent>
      </Toolbar>

      {rules.length === 0 ? (
        <EmptyState>
          <EmptyStateIcon icon={PlusCircleIcon} />
          <Title headingLevel="h4" size="lg">
            No rules
          </Title>
          <EmptyStateBody>
            {readOnly ? 'No rules defined for this resource.' : 'Add a rule to get started.'}
          </EmptyStateBody>
        </EmptyState>
      ) : (
        <Table aria-label="Rules table" borders={true}>
          <Thead>
            <Tr>
              <Th>Direction</Th>
              <Th>Protocol</Th>
              <Th>{isSG ? 'Ports' : 'Port Range'}</Th>
              {isSG && <Th>Remote</Th>}
              {isACL && <Th>Source</Th>}
              {isACL && <Th>Destination</Th>}
              {isACL && <Th>Action</Th>}
              {isACL && <Th>Priority</Th>}
              {!readOnly && <Th>Actions</Th>}
            </Tr>
          </Thead>
          <Tbody>
            {rules.map((rule) => (
              <Tr key={rule.id}>
                <Td>{rule.direction}</Td>
                <Td>{rule.protocol}</Td>
                <Td>{getPortDisplay(rule)}</Td>
                {isSG && isSecurityGroupRule(rule) && (
                  <Td>{rule.remote ?? '-'}</Td>
                )}
                {isACL && isNetworkACLRule(rule) && (
                  <>
                    <Td>{rule.source}</Td>
                    <Td>{rule.destination}</Td>
                    <Td>
                      <span className={`badge badge-${rule.action === 'allow' ? 'success' : 'danger'}`}>
                        {rule.action}
                      </span>
                    </Td>
                    <Td>{rule.priority ?? '-'}</Td>
                  </>
                )}
                {!readOnly && (
                  <Td>
                    <Button
                      variant={ButtonVariant.link}
                      onClick={() => handleEditRule(rule)}
                      isDisabled={isLoading}
                    >
                      Edit
                    </Button>
                    <Button
                      variant={ButtonVariant.link}
                      isDanger
                      onClick={() => handleDeleteClick(rule)}
                      isDisabled={isLoading}
                    >
                      Delete
                    </Button>
                  </Td>
                )}
              </Tr>
            ))}
          </Tbody>
        </Table>
      )}

      {/* Add/Edit Rule Modal */}
      <Modal
        title={editingRule ? 'Edit Rule' : 'Add Rule'}
        isOpen={isFormOpen}
        onClose={() => {
          setIsFormOpen(false);
          setEditingRule(undefined);
        }}
        variant={ModalVariant.medium}
      >
        <RuleForm
          resourceType={resourceType}
          initialValues={
            editingRule
              ? {
                  direction: editingRule.direction,
                  protocol: editingRule.protocol,
                  portMin: editingRule.portMin,
                  portMax: editingRule.portMax,
                  icmpType: editingRule.icmpType,
                  icmpCode: editingRule.icmpCode,
                  remote: isSecurityGroupRule(editingRule) ? editingRule.remote : undefined,
                  remoteType: isSecurityGroupRule(editingRule) ? (editingRule.remoteType as any) : 'cidr',
                  source: isNetworkACLRule(editingRule) ? editingRule.source : '',
                  destination: isNetworkACLRule(editingRule) ? editingRule.destination : '',
                  action: isNetworkACLRule(editingRule) ? editingRule.action : 'allow',
                  priority: isNetworkACLRule(editingRule) ? editingRule.priority : undefined,
                }
              : undefined
          }
          onSubmit={handleFormSubmit}
          onCancel={handleFormCancel}
        />
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        title="Delete Rule?"
        isOpen={deleteConfirmation.open}
        variant={ModalVariant.small}
        onClose={() => setDeleteConfirmation({ open: false })}
        actions={[
          <Button
            key="confirm"
            variant={ButtonVariant.danger}
            onClick={handleConfirmDelete}
            isDisabled={isLoading}
            isLoading={isLoading}
          >
            Delete
          </Button>,
          <Button
            key="cancel"
            variant={ButtonVariant.link}
            onClick={() => setDeleteConfirmation({ open: false })}
            isDisabled={isLoading}
          >
            Cancel
          </Button>,
        ]}
      >
        <div>
          <ExclamationTriangleIcon style={{ marginRight: '8px', color: '#f0ad4e' }} />
          Are you sure you want to delete this rule?
          <div style={{ marginTop: '8px', color: '#666' }}>
            {deleteConfirmation.ruleDescription}
          </div>
        </div>
      </Modal>
    </div>
  );
};
