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
import { useVPCs } from '../api/hooks';

export interface CreateSecurityGroupModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export const CreateSecurityGroupModal: React.FC<CreateSecurityGroupModalProps> = ({
  isOpen,
  onClose,
  onCreated,
}) => {
  const [name, setName] = useState('');
  const [vpcId, setVpcId] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const { vpcs } = useVPCs();

  const resetForm = () => {
    setName('');
    setVpcId('');
    setDescription('');
    setError('');
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async () => {
    if (!name || !vpcId) {
      setError('Name and VPC are required.');
      return;
    }
    setIsSubmitting(true);
    setError('');
    try {
      const resp = await apiClient.createSecurityGroup({
        name,
        vpc_id: vpcId,
        description,
      });
      setIsSubmitting(false);
      if (resp.error) {
        const msg = resp.error.message || 'Failed to create security group';
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
      title="Create Security Group"
      isOpen={isOpen}
      onClose={handleClose}
      actions={[
        <Button key="cancel" variant={ButtonVariant.link} onClick={handleClose} isDisabled={isSubmitting}>
          Cancel
        </Button>,
        <Button key="create" variant={ButtonVariant.primary} onClick={handleSubmit} isDisabled={isSubmitting || !name || !vpcId} isLoading={isSubmitting}>
          Create
        </Button>,
      ]}
    >
      <Form>
        {error && <Alert variant="danger" isInline title={error} />}
        <FormGroup label="Name" isRequired fieldId="sg-name">
          <TextInput id="sg-name" value={name} onChange={(_e, v) => setName(v)} isRequired isDisabled={isSubmitting} />
        </FormGroup>
        <FormGroup label="VPC" isRequired fieldId="sg-vpc">
          <FormSelect id="sg-vpc" value={vpcId} onChange={(_e, v) => setVpcId(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="" label="Select a VPC" isDisabled />
            {(vpcs || []).map((vpc) => (
              <FormSelectOption key={vpc.id} value={vpc.id} label={vpc.name || vpc.id} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label="Description" fieldId="sg-desc">
          <TextInput id="sg-desc" value={description} onChange={(_e, v) => setDescription(v)} isDisabled={isSubmitting} />
        </FormGroup>
      </Form>
    </Modal>
  );
};

export default CreateSecurityGroupModal;
