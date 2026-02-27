import React, { useState } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  ButtonVariant,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  Alert,
} from '@patternfly/react-core';
import { apiClient } from '../api/client';
import { useZones } from '../api/hooks';

export interface CreateRouteModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
  routingTableId: string;
  routingTableName: string;
}

const CreateRouteModal: React.FC<CreateRouteModalProps> = ({
  isOpen,
  onClose,
  onCreated,
  routingTableId,
  routingTableName,
}) => {
  const [name, setName] = useState('');
  const [destination, setDestination] = useState('');
  const [action, setAction] = useState('deliver');
  const [nextHopIp, setNextHopIp] = useState('');
  const [zoneName, setZoneName] = useState('');
  const [priority, setPriority] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const { zones } = useZones();

  const resetForm = () => {
    setName('');
    setDestination('');
    setAction('deliver');
    setNextHopIp('');
    setZoneName('');
    setPriority('');
    setError('');
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async () => {
    if (!destination || !action || !zoneName) {
      setError('Destination, action, and zone are required.');
      return;
    }
    if (action === 'deliver' && !nextHopIp) {
      setError('Next hop IP is required for deliver action.');
      return;
    }

    setIsSubmitting(true);
    setError('');

    try {
      const resp = await apiClient.createRoute(routingTableId, {
        name: name || undefined,
        destination,
        action,
        nextHopIp: action === 'deliver' ? nextHopIp : undefined,
        zone: zoneName,
        priority: priority ? parseInt(priority, 10) : undefined,
      });

      setIsSubmitting(false);
      if (resp.error) {
        const msg = resp.error.message || 'Failed to create route';
        setError(typeof msg === 'string' ? msg : JSON.stringify(msg));
      } else {
        resetForm();
        onCreated();
      }
    } catch (e) {
      setIsSubmitting(false);
      setError(e instanceof Error ? e.message : typeof e === 'string' ? e : JSON.stringify(e));
    }
  };

  const isValid = destination && action && zoneName && (action !== 'deliver' || nextHopIp);

  return (
    <Modal
      variant={ModalVariant.small}
      title="Create Route"
      isOpen={isOpen}
      onClose={handleClose}
      actions={[
        <Button key="cancel" variant={ButtonVariant.link} onClick={handleClose} isDisabled={isSubmitting}>
          Cancel
        </Button>,
        <Button key="create" variant={ButtonVariant.primary} onClick={handleSubmit} isDisabled={isSubmitting || !isValid} isLoading={isSubmitting}>
          Create
        </Button>,
      ]}
    >
      <Form>
        {error && <Alert variant="danger" isInline title={error} />}
        <FormGroup label="Routing Table" fieldId="route-rt">
          <TextInput id="route-rt" value={routingTableName} isDisabled />
        </FormGroup>
        <FormGroup label="Name" fieldId="route-name">
          <TextInput id="route-name" value={name} onChange={(_e, v) => setName(v)} placeholder="Optional" isDisabled={isSubmitting} />
        </FormGroup>
        <FormGroup label="Destination CIDR" isRequired fieldId="route-destination">
          <TextInput id="route-destination" value={destination} onChange={(_e, v) => setDestination(v)} placeholder="e.g. 10.0.0.0/8" isRequired isDisabled={isSubmitting} />
        </FormGroup>
        <FormGroup label="Action" isRequired fieldId="route-action">
          <FormSelect id="route-action" value={action} onChange={(_e, v) => setAction(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="deliver" label="Deliver" />
            <FormSelectOption value="delegate" label="Delegate" />
            <FormSelectOption value="delegate_vpc" label="Delegate VPC" />
            <FormSelectOption value="drop" label="Drop" />
          </FormSelect>
        </FormGroup>
        {action === 'deliver' && (
          <FormGroup label="Next Hop IP" isRequired fieldId="route-nexthop">
            <TextInput id="route-nexthop" value={nextHopIp} onChange={(_e, v) => setNextHopIp(v)} placeholder="e.g. 10.240.64.5" isRequired isDisabled={isSubmitting} />
          </FormGroup>
        )}
        <FormGroup label="Zone" isRequired fieldId="route-zone">
          <FormSelect id="route-zone" value={zoneName} onChange={(_e, v) => setZoneName(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="" label="Select a zone" isDisabled />
            {(zones || []).map((z) => (
              <FormSelectOption key={z.name || z.id} value={z.name || z.id} label={z.name || z.id} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label="Priority" fieldId="route-priority">
          <TextInput id="route-priority" type="number" value={priority} onChange={(_e, v) => setPriority(v)} placeholder="0-65534 (optional)" isDisabled={isSubmitting} />
        </FormGroup>
      </Form>
    </Modal>
  );
};

export default CreateRouteModal;
