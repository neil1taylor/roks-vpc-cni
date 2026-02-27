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

export interface CreateFloatingIPModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export const CreateFloatingIPModal: React.FC<CreateFloatingIPModalProps> = ({
  isOpen,
  onClose,
  onCreated,
}) => {
  const [name, setName] = useState('');
  const [zoneName, setZoneName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const { zones } = useZones();

  const resetForm = () => {
    setName('');
    setZoneName('');
    setError('');
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async () => {
    if (!name || !zoneName) {
      setError('All fields are required.');
      return;
    }
    setIsSubmitting(true);
    setError('');
    try {
      const resp = await apiClient.createFloatingIP({
        name,
        zone: zoneName,
      });
      setIsSubmitting(false);
      if (resp.error) {
        const msg = resp.error.message || 'Failed to reserve floating IP';
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

  return (
    <Modal
      variant={ModalVariant.small}
      title="Reserve Floating IP"
      isOpen={isOpen}
      onClose={handleClose}
      actions={[
        <Button key="cancel" variant={ButtonVariant.link} onClick={handleClose} isDisabled={isSubmitting}>
          Cancel
        </Button>,
        <Button key="create" variant={ButtonVariant.primary} onClick={handleSubmit} isDisabled={isSubmitting || !name || !zoneName} isLoading={isSubmitting}>
          Reserve
        </Button>,
      ]}
    >
      <Form>
        {error && <Alert variant="danger" isInline title={error} />}
        <FormGroup label="Name" isRequired fieldId="fip-name">
          <TextInput id="fip-name" value={name} onChange={(_e, v) => setName(v)} isRequired isDisabled={isSubmitting} />
        </FormGroup>
        <FormGroup label="Zone" isRequired fieldId="fip-zone">
          <FormSelect id="fip-zone" value={zoneName} onChange={(_e, v) => setZoneName(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="" label="Select a zone" isDisabled />
            {(zones || []).map((z) => (
              <FormSelectOption key={z.name || z.id} value={z.name || z.id} label={z.name || z.id} />
            ))}
          </FormSelect>
        </FormGroup>
      </Form>
    </Modal>
  );
};

export default CreateFloatingIPModal;
