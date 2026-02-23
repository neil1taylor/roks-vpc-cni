import React, { useState } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  ButtonVariant,
  Form,
  FormGroup,
  TextInput,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { ExclamationTriangleIcon } from '@patternfly/react-icons';

export interface DeleteConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  resourceName?: string;
  confirmText?: string;
  isLoading?: boolean;
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
}

export const DeleteConfirmModal: React.FC<DeleteConfirmModalProps> = ({
  isOpen,
  title,
  message,
  resourceName,
  confirmText = 'Delete',
  isLoading = false,
  onConfirm,
  onCancel,
}) => {
  const [confirmationInput, setConfirmationInput] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);

  const requiresConfirmation = !!resourceName;
  const isConfirmationValid = !requiresConfirmation || confirmationInput === resourceName;
  const isDeleteDisabled = isLoading || isDeleting || !isConfirmationValid;

  const handleConfirm = async () => {
    setIsDeleting(true);
    try {
      await onConfirm();
    } finally {
      setIsDeleting(false);
      setConfirmationInput('');
    }
  };

  const handleCancel = () => {
    setConfirmationInput('');
    onCancel();
  };

  return (
    <Modal
      variant={ModalVariant.small}
      title={title}
      isOpen={isOpen}
      onClose={handleCancel}
      actions={[
        <Button
          key="cancel"
          variant={ButtonVariant.link}
          onClick={handleCancel}
          isDisabled={isDeleting}
        >
          Cancel
        </Button>,
        <Button
          key="delete"
          variant={ButtonVariant.danger}
          onClick={handleConfirm}
          isDisabled={isDeleteDisabled}
          isLoading={isDeleting}
        >
          {confirmText}
        </Button>,
      ]}
    >
      <Form>
        <div style={{ marginBottom: '16px' }}>
          <ExclamationTriangleIcon color="var(--pf-v5-global--danger-color--100)" />
          {' '}
          <Text component={TextVariants.p} style={{ display: 'inline' }}>
            {message}
          </Text>
        </div>

        {requiresConfirmation && (
          <FormGroup label="Confirm by typing the resource name">
            <TextInput
              type="text"
              value={confirmationInput}
              onChange={(_event, value) => setConfirmationInput(value)}
              placeholder={resourceName}
              isRequired
              isDisabled={isDeleting}
            />
            <Text component={TextVariants.small} style={{ marginTop: '8px' }}>
              Type
              {' '}
              <strong>{resourceName}</strong>
              {' '}
              to confirm deletion.
            </Text>
          </FormGroup>
        )}
      </Form>
    </Modal>
  );
};

DeleteConfirmModal.displayName = 'DeleteConfirmModal';

export default DeleteConfirmModal;
