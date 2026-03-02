import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  ButtonVariant,
  Form,
  FormGroup,
  TextInput,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { PlusCircleIcon, TrashIcon } from '@patternfly/react-icons';
import { DHCPReservation } from '../api/types';
import { apiClient } from '../api/client';
import { isValidMAC, isValidIPv4 } from '../utils/validators';

export interface EditReservationsModalProps {
  isOpen: boolean;
  onClose: (updated?: boolean) => void;
  routerName: string;
  routerNamespace: string;
  networkName: string;
  currentReservations: DHCPReservation[];
}

interface ReservationRow {
  mac: string;
  ip: string;
  hostname: string;
}

export const EditReservationsModal: React.FC<EditReservationsModalProps> = ({
  isOpen,
  onClose,
  routerName,
  routerNamespace,
  networkName,
  currentReservations,
}) => {
  const [rows, setRows] = useState<ReservationRow[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setRows(
        currentReservations.length > 0
          ? currentReservations.map((r) => ({ mac: r.mac, ip: r.ip, hostname: r.hostname || '' }))
          : [{ mac: '', ip: '', hostname: '' }],
      );
      setError(null);
    }
  }, [isOpen, currentReservations]);

  const updateRow = (index: number, field: keyof ReservationRow, value: string) => {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  };

  const addRow = () => {
    setRows((prev) => [...prev, { mac: '', ip: '', hostname: '' }]);
  };

  const removeRow = (index: number) => {
    setRows((prev) => prev.filter((_, i) => i !== index));
  };

  const validate = (): string | null => {
    const nonEmpty = rows.filter((r) => r.mac || r.ip);
    for (let i = 0; i < nonEmpty.length; i++) {
      const r = nonEmpty[i];
      if (!r.mac || !r.ip) {
        return `Row ${i + 1}: MAC and IP are both required`;
      }
      if (!isValidMAC(r.mac)) {
        return `Row ${i + 1}: Invalid MAC address "${r.mac}"`;
      }
      if (!isValidIPv4(r.ip)) {
        return `Row ${i + 1}: Invalid IP address "${r.ip}"`;
      }
    }
    return null;
  };

  const handleSave = async () => {
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }

    setSaving(true);
    setError(null);

    const reservations: DHCPReservation[] = rows
      .filter((r) => r.mac && r.ip)
      .map((r) => ({
        mac: r.mac,
        ip: r.ip,
        ...(r.hostname ? { hostname: r.hostname } : {}),
      }));

    const resp = await apiClient.updateRouterReservations(routerName, routerNamespace, networkName, reservations);
    setSaving(false);

    if (resp.error) {
      setError(resp.error.message);
    } else {
      onClose(true);
    }
  };

  return (
    <Modal
      variant={ModalVariant.large}
      title={`Edit Reservations — ${networkName}`}
      isOpen={isOpen}
      onClose={() => onClose()}
      actions={[
        <Button key="cancel" variant={ButtonVariant.link} onClick={() => onClose()} isDisabled={saving}>
          Cancel
        </Button>,
        <Button key="save" variant={ButtonVariant.primary} onClick={handleSave} isLoading={saving} isDisabled={saving}>
          Save
        </Button>,
      ]}
    >
      {error && <Alert variant="danger" title={error} isInline style={{ marginBottom: '16px' }} />}

      <Form>
        <FormGroup>
          <Table aria-label="DHCP reservations" variant="compact">
            <Thead>
              <Tr>
                <Th>MAC Address</Th>
                <Th>IP Address</Th>
                <Th>Hostname</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {rows.map((row, i) => (
                <Tr key={i}>
                  <Td>
                    <TextInput
                      value={row.mac}
                      onChange={(_e, val) => updateRow(i, 'mac', val)}
                      placeholder="fa:16:3e:aa:bb:cc"
                      isDisabled={saving}
                    />
                  </Td>
                  <Td>
                    <TextInput
                      value={row.ip}
                      onChange={(_e, val) => updateRow(i, 'ip', val)}
                      placeholder="10.0.0.10"
                      isDisabled={saving}
                    />
                  </Td>
                  <Td>
                    <TextInput
                      value={row.hostname}
                      onChange={(_e, val) => updateRow(i, 'hostname', val)}
                      placeholder="(optional)"
                      isDisabled={saving}
                    />
                  </Td>
                  <Td>
                    <Button
                      variant={ButtonVariant.plain}
                      onClick={() => removeRow(i)}
                      isDisabled={saving || rows.length <= 1}
                      aria-label="Remove reservation"
                    >
                      <TrashIcon />
                    </Button>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
          <FormHelperText>
            <HelperText>
              <HelperTextItem>
                Static DHCP reservations bind a MAC address to a fixed IP. Changes take effect when dnsmasq reloads (on pod recreation).
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>
        <Button variant={ButtonVariant.link} icon={<PlusCircleIcon />} onClick={addRow} isDisabled={saving}>
          Add reservation
        </Button>
      </Form>
    </Modal>
  );
};

export default EditReservationsModal;
