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
import { useVPCs, useZones } from '../api/hooks';

export interface CreateSubnetModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export const CreateSubnetModal: React.FC<CreateSubnetModalProps> = ({
  isOpen,
  onClose,
  onCreated,
}) => {
  const [name, setName] = useState('');
  const [vpcId, setVpcId] = useState('');
  const [zoneName, setZoneName] = useState('');
  const [cidr, setCidr] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const { vpcs } = useVPCs();
  const { zones } = useZones();

  // Auto-select the first (cluster) VPC when loaded
  React.useEffect(() => {
    if (vpcs && vpcs.length > 0 && !vpcId) {
      setVpcId(vpcs[0].id);
    }
  }, [vpcs, vpcId]);

  const resetForm = () => {
    setName('');
    setVpcId('');
    setZoneName('');
    setCidr('');
    setError('');
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async () => {
    if (!name || !vpcId || !zoneName || !cidr) {
      setError('All fields are required.');
      return;
    }
    setIsSubmitting(true);
    setError('');
    try {
      const resp = await apiClient.createSubnet({
        name,
        vpc: { id: vpcId },
        zone: { id: zoneName, name: zoneName },
        ipv4CidrBlock: cidr,
      });
      setIsSubmitting(false);
      if (resp.error) {
        const msg = resp.error.message || 'Failed to create subnet';
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
      title="Create Subnet"
      isOpen={isOpen}
      onClose={handleClose}
      actions={[
        <Button key="cancel" variant={ButtonVariant.link} onClick={handleClose} isDisabled={isSubmitting}>
          Cancel
        </Button>,
        <Button key="create" variant={ButtonVariant.primary} onClick={handleSubmit} isDisabled={isSubmitting || !name || !vpcId || !zoneName || !cidr} isLoading={isSubmitting}>
          Create
        </Button>,
      ]}
    >
      <Form>
        {error && <Alert variant="danger" isInline title={error} />}
        <FormGroup label="Name" isRequired fieldId="subnet-name">
          <TextInput id="subnet-name" value={name} onChange={(_e, v) => setName(v)} isRequired isDisabled={isSubmitting} />
        </FormGroup>
        <FormGroup label="VPC" isRequired fieldId="subnet-vpc">
          <FormSelect id="subnet-vpc" value={vpcId} onChange={(_e, v) => setVpcId(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="" label="Select a VPC" isDisabled />
            {(vpcs || []).map((vpc) => (
              <FormSelectOption key={vpc.id} value={vpc.id} label={vpc.name || vpc.id} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label="Zone" isRequired fieldId="subnet-zone">
          <FormSelect id="subnet-zone" value={zoneName} onChange={(_e, v) => setZoneName(v)} isDisabled={isSubmitting}>
            <FormSelectOption value="" label="Select a zone" isDisabled />
            {(zones || []).map((z) => (
              <FormSelectOption key={z.name || z.id} value={z.name || z.id} label={z.name || z.id} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label="IPv4 CIDR Block" isRequired fieldId="subnet-cidr">
          <TextInput id="subnet-cidr" value={cidr} onChange={(_e, v) => setCidr(v)} placeholder="10.240.0.0/24" isRequired isDisabled={isSubmitting} />
        </FormGroup>
      </Form>
    </Modal>
  );
};

export default CreateSubnetModal;
