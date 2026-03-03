import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalVariant,
  Button,
  ButtonVariant,
  Form,
  FormGroup,
  FormSelect,
  FormSelectOption,
  TextInput,
  TextArea,
  Switch,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';
import { RouterIDS } from '../api/types';
import { apiClient } from '../api/client';

export interface EditIDSModalProps {
  isOpen: boolean;
  onClose: (updated?: boolean) => void;
  routerName: string;
  routerNamespace: string;
  currentIDS?: RouterIDS;
}

export const EditIDSModal: React.FC<EditIDSModalProps> = ({
  isOpen,
  onClose,
  routerName,
  routerNamespace,
  currentIDS,
}) => {
  const [enabled, setEnabled] = useState(false);
  const [mode, setMode] = useState('ids');
  const [interfaces, setInterfaces] = useState('all');
  const [syslogTarget, setSyslogTarget] = useState('');
  const [customRules, setCustomRules] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setEnabled(currentIDS?.enabled ?? false);
      setMode(currentIDS?.mode || 'ids');
      setInterfaces(currentIDS?.interfaces || 'all');
      setSyslogTarget(currentIDS?.syslogTarget || '');
      setCustomRules(currentIDS?.customRules || '');
      setError(null);
    }
  }, [isOpen, currentIDS]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    const resp = await apiClient.updateRouterIDS(routerName, routerNamespace, {
      enabled,
      mode,
      interfaces: enabled ? interfaces : undefined,
      customRules: enabled && customRules ? customRules : undefined,
      syslogTarget: enabled && syslogTarget ? syslogTarget : undefined,
    });

    setSaving(false);
    if (resp.error) {
      setError(resp.error.message);
    } else {
      onClose(true);
    }
  };

  return (
    <Modal
      variant={ModalVariant.medium}
      title="Edit IDS/IPS Configuration"
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
        <FormGroup fieldId="ids-enabled">
          <Switch
            id="ids-enabled"
            label="IDS/IPS Enabled"
            labelOff="IDS/IPS Disabled"
            isChecked={enabled}
            onChange={(_e, checked) => setEnabled(checked)}
            isDisabled={saving}
          />
          <FormHelperText>
            <HelperText>
              <HelperTextItem>
                Deploys a Suricata sidecar container on the router pod for network intrusion detection/prevention.
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>

        <FormGroup label="Mode" fieldId="ids-mode">
          <FormSelect
            id="ids-mode"
            value={mode}
            onChange={(_e, v) => setMode(v)}
            isDisabled={!enabled || saving}
          >
            <FormSelectOption value="ids" label="IDS — Passive Monitoring" />
            <FormSelectOption value="ips" label="IPS — Inline Blocking" />
          </FormSelect>
          <FormHelperText>
            <HelperText>
              <HelperTextItem>
                IDS passively monitors traffic (AF_PACKET). IPS inspects and blocks traffic inline (NFQUEUE). Changing mode recreates the router pod.
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>

        <FormGroup label="Interfaces" fieldId="ids-interfaces">
          <FormSelect
            id="ids-interfaces"
            value={interfaces}
            onChange={(_e, v) => setInterfaces(v)}
            isDisabled={!enabled || saving}
          >
            <FormSelectOption value="all" label="All interfaces" />
            <FormSelectOption value="uplink" label="Uplink only" />
            <FormSelectOption value="workload" label="Workload only" />
          </FormSelect>
          <FormHelperText>
            <HelperText>
              <HelperTextItem>
                &quot;All&quot; monitors both uplink and workload traffic. &quot;Uplink&quot; monitors external-facing traffic to/from the VPC. &quot;Workload&quot; monitors VM-to-VM traffic on internal networks.
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>

        <FormGroup label="Syslog Target" fieldId="ids-syslog">
          <TextInput
            id="ids-syslog"
            value={syslogTarget}
            onChange={(_e, v) => setSyslogTarget(v)}
            placeholder="syslog-host:514"
            isDisabled={!enabled || saving}
          />
          <FormHelperText>
            <HelperText>
              <HelperTextItem>Optional syslog destination for EVE JSON alerts (host:port)</HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>

        <FormGroup label="Custom Rules" fieldId="ids-custom-rules">
          <TextArea
            id="ids-custom-rules"
            value={customRules}
            onChange={(_e, v) => setCustomRules(v)}
            placeholder={'alert tcp any any -> any any (msg:"Test"; sid:1000001; rev:1;)'}
            rows={4}
            isDisabled={!enabled || saving}
            resizeOrientation="vertical"
          />
          <FormHelperText>
            <HelperText>
              <HelperTextItem>Additional Suricata rules, one per line. These are appended to the default ET Open ruleset.</HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>
      </Form>
    </Modal>
  );
};

export default EditIDSModal;
