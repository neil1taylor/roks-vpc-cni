import React, { useState, useMemo } from 'react';
import {
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  NumberInput,
  Radio,
  Button,
  ButtonVariant,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';
import { isValidCIDRBlock as validateCIDR, isValidICMPType as validateICMPType } from '../utils/validators';

export type ResourceType = 'security-group' | 'network-acl';
export type Direction = 'inbound' | 'outbound';
export type Protocol = 'tcp' | 'udp' | 'icmp' | 'all';
export type RemoteType = 'cidr' | 'security-group';
export type RuleAction = 'allow' | 'deny';

export interface RuleFormValues {
  direction?: Direction;
  protocol?: Protocol;
  portMin?: number;
  portMax?: number;
  icmpType?: number;
  icmpCode?: number;
  remote?: string;
  remoteType?: RemoteType;
  source?: string;
  destination?: string;
  action?: RuleAction;
  priority?: number;
}

interface RuleFormProps {
  resourceType: ResourceType;
  initialValues?: RuleFormValues;
  onSubmit: (values: RuleFormValues) => Promise<void>;
  onCancel: () => void;
}

const PROTOCOLS: { value: Protocol; label: string }[] = [
  { value: 'tcp', label: 'TCP' },
  { value: 'udp', label: 'UDP' },
  { value: 'icmp', label: 'ICMP' },
  { value: 'all', label: 'All' },
];

const DIRECTIONS: { value: Direction; label: string }[] = [
  { value: 'inbound', label: 'Inbound' },
  { value: 'outbound', label: 'Outbound' },
];

const ACTIONS: { value: RuleAction; label: string }[] = [
  { value: 'allow', label: 'Allow' },
  { value: 'deny', label: 'Deny' },
];

export const RuleForm: React.FC<RuleFormProps> = ({
  resourceType,
  initialValues,
  onSubmit,
  onCancel,
}) => {
  const [values, setValues] = useState<RuleFormValues>(
    initialValues || {
      direction: 'inbound',
      protocol: 'tcp',
      portMin: undefined,
      portMax: undefined,
      icmpType: undefined,
      icmpCode: undefined,
      remote: '',
      remoteType: 'cidr',
      source: '',
      destination: '',
      action: 'allow',
      priority: undefined,
    }
  );

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isACL = resourceType === 'network-acl';
  const isSG = resourceType === 'security-group';

  const showPortFields = useMemo(
    () => values.protocol === 'tcp' || values.protocol === 'udp',
    [values.protocol]
  );

  const showICMPFields = useMemo(() => values.protocol === 'icmp', [values.protocol]);

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Common validations
    if (!values.direction) {
      newErrors.direction = 'Direction is required';
    }
    if (!values.protocol) {
      newErrors.protocol = 'Protocol is required';
    }

    // Port validations
    if (showPortFields) {
      if (values.portMin === undefined || values.portMin === null) {
        newErrors.portMin = 'Port Min is required';
      } else if (values.portMin < 0 || values.portMin > 65535) {
        newErrors.portMin = 'Port must be between 0 and 65535';
      }

      if (values.portMax === undefined || values.portMax === null) {
        newErrors.portMax = 'Port Max is required';
      } else if (values.portMax < 0 || values.portMax > 65535) {
        newErrors.portMax = 'Port must be between 0 and 65535';
      }

      if (
        values.portMin !== undefined &&
        values.portMax !== undefined &&
        values.portMin > values.portMax
      ) {
        newErrors.portRange = 'Port Min must be less than or equal to Port Max';
      }
    }

    // ICMP validations
    if (showICMPFields) {
      if (values.icmpType === undefined || values.icmpType === null) {
        newErrors.icmpType = 'ICMP Type is required';
      } else if (!validateICMPType(values.icmpType)) {
        newErrors.icmpType = 'ICMP Type must be between 0 and 255';
      }

      if (values.icmpCode === undefined || values.icmpCode === null) {
        newErrors.icmpCode = 'ICMP Code is required';
      } else if (values.icmpCode < 0 || values.icmpCode > 255) {
        newErrors.icmpCode = 'ICMP Code must be between 0 and 255';
      }
    }

    // SG-specific validations
    if (isSG) {
      if (!values.remote) {
        newErrors.remote = 'Remote is required';
      } else if (values.remoteType === 'cidr') {
        if (!validateCIDR(values.remote)) {
          newErrors.remote = 'Invalid CIDR notation';
        }
      }
    }

    // ACL-specific validations
    if (isACL) {
      if (!values.source) {
        newErrors.source = 'Source CIDR is required';
      } else if (!validateCIDR(values.source)) {
        newErrors.source = 'Invalid source CIDR notation';
      }

      if (!values.destination) {
        newErrors.destination = 'Destination CIDR is required';
      } else if (!validateCIDR(values.destination)) {
        newErrors.destination = 'Invalid destination CIDR notation';
      }

      if (!values.action) {
        newErrors.action = 'Action is required';
      }

      if (values.priority === undefined || values.priority === null) {
        newErrors.priority = 'Priority is required';
      } else if (values.priority < 1 || values.priority > 32767) {
        newErrors.priority = 'Priority must be between 1 and 32767';
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);
    try {
      await onSubmit(values);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Form onSubmit={handleSubmit}>
      {/* Direction Field */}
      <FormGroup label="Direction" isRequired fieldId="direction">
        <FormSelect
          id="direction"
          value={values.direction}
          onChange={(_event, value) => setValues({ ...values, direction: value as Direction })}
        >
          {DIRECTIONS.map((opt) => (
            <FormSelectOption key={opt.value} value={opt.value} label={opt.label} />
          ))}
        </FormSelect>
        {errors.direction && (
          <FormHelperText>
            <HelperText>
              <HelperTextItem variant="error">{errors.direction}</HelperTextItem>
            </HelperText>
          </FormHelperText>
        )}
      </FormGroup>

      {/* Protocol Field */}
      <FormGroup label="Protocol" isRequired fieldId="protocol">
        <FormSelect
          id="protocol"
          value={values.protocol}
          onChange={(_event, value) => setValues({ ...values, protocol: value as Protocol })}
        >
          {PROTOCOLS.map((opt) => (
            <FormSelectOption key={opt.value} value={opt.value} label={opt.label} />
          ))}
        </FormSelect>
        {errors.protocol && (
          <FormHelperText>
            <HelperText>
              <HelperTextItem variant="error">{errors.protocol}</HelperTextItem>
            </HelperText>
          </FormHelperText>
        )}
      </FormGroup>

      {/* Port Fields (TCP/UDP) */}
      {showPortFields && (
        <>
          <FormGroup label="Port Min" isRequired fieldId="portMin">
            <NumberInput
              id="portMin"
              value={values.portMin}
              onMinus={() => setValues({ ...values, portMin: Math.max(0, (values.portMin || 0) - 1) })}
              onChange={(e) => {
                const val = parseInt((e.target as HTMLInputElement).value, 10);
                setValues({ ...values, portMin: isNaN(val) ? undefined : val });
              }}
              min={0}
              max={65535}
            />
            {errors.portMin && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.portMin}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          <FormGroup label="Port Max" isRequired fieldId="portMax">
            <NumberInput
              id="portMax"
              value={values.portMax}
              onMinus={() => setValues({ ...values, portMax: Math.max(0, (values.portMax || 0) - 1) })}
              onChange={(e) => {
                const val = parseInt((e.target as HTMLInputElement).value, 10);
                setValues({ ...values, portMax: isNaN(val) ? undefined : val });
              }}
              min={0}
              max={65535}
            />
            {errors.portMax && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.portMax}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          {errors.portRange && (
            <FormGroup fieldId="portRange">
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.portRange}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            </FormGroup>
          )}
        </>
      )}

      {/* ICMP Fields */}
      {showICMPFields && (
        <>
          <FormGroup label="ICMP Type" isRequired fieldId="icmpType">
            <NumberInput
              id="icmpType"
              value={values.icmpType}
              onMinus={() => setValues({ ...values, icmpType: Math.max(0, (values.icmpType || 0) - 1) })}
              onChange={(e) => {
                const val = parseInt((e.target as HTMLInputElement).value, 10);
                setValues({ ...values, icmpType: isNaN(val) ? undefined : val });
              }}
              min={0}
              max={255}
            />
            {errors.icmpType && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.icmpType}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          <FormGroup label="ICMP Code" isRequired fieldId="icmpCode">
            <NumberInput
              id="icmpCode"
              value={values.icmpCode}
              onMinus={() => setValues({ ...values, icmpCode: Math.max(0, (values.icmpCode || 0) - 1) })}
              onChange={(e) => {
                const val = parseInt((e.target as HTMLInputElement).value, 10);
                setValues({ ...values, icmpCode: isNaN(val) ? undefined : val });
              }}
              min={0}
              max={255}
            />
            {errors.icmpCode && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.icmpCode}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>
        </>
      )}

      {/* Security Group: Remote */}
      {isSG && (
        <>
          <FormGroup label="Remote Type" isRequired>
            <div>
              <Radio
                id="remote-cidr"
                name="remoteType"
                label="CIDR Block"
                isChecked={values.remoteType === 'cidr'}
                onChange={() => setValues({ ...values, remoteType: 'cidr' })}
              />
              <Radio
                id="remote-sg"
                name="remoteType"
                label="Security Group"
                isChecked={values.remoteType === 'security-group'}
                onChange={() => setValues({ ...values, remoteType: 'security-group' })}
              />
            </div>
          </FormGroup>

          <FormGroup
            label={values.remoteType === 'cidr' ? 'CIDR Block' : 'Security Group ID'}
            isRequired
            fieldId="remote"
          >
            <TextInput
              id="remote"
              value={values.remote || ''}
              onChange={(e) => setValues({ ...values, remote: e.currentTarget.value })}
              placeholder={
                values.remoteType === 'cidr' ? '10.0.0.0/8' : 'sg-0123456789abcdef0'
              }
            />
            {errors.remote && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.remote}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>
        </>
      )}

      {/* ACL: Source and Destination */}
      {isACL && (
        <>
          <FormGroup label="Source CIDR" isRequired fieldId="source">
            <TextInput
              id="source"
              value={values.source || ''}
              onChange={(e) => setValues({ ...values, source: e.currentTarget.value })}
              placeholder="10.0.0.0/16"
            />
            {errors.source && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.source}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          <FormGroup label="Destination CIDR" isRequired fieldId="destination">
            <TextInput
              id="destination"
              value={values.destination || ''}
              onChange={(e) => setValues({ ...values, destination: e.currentTarget.value })}
              placeholder="0.0.0.0/0"
            />
            {errors.destination && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.destination}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          <FormGroup label="Action" isRequired fieldId="action">
            <FormSelect
              id="action"
              value={values.action}
              onChange={(_event, value) => setValues({ ...values, action: value as RuleAction })}
            >
              {ACTIONS.map((opt) => (
                <FormSelectOption key={opt.value} value={opt.value} label={opt.label} />
              ))}
            </FormSelect>
            {errors.action && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.action}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>

          <FormGroup label="Priority" isRequired fieldId="priority">
            <NumberInput
              id="priority"
              value={values.priority}
              onMinus={() => setValues({ ...values, priority: Math.max(1, (values.priority || 1) - 1) })}
              onChange={(e) => {
                const val = parseInt((e.currentTarget as HTMLInputElement).value, 10);
                setValues({ ...values, priority: isNaN(val) ? undefined : val });
              }}
              min={1}
              max={32767}
            />
            {errors.priority && (
              <FormHelperText>
                <HelperText>
                  <HelperTextItem variant="error">{errors.priority}</HelperTextItem>
                </HelperText>
              </FormHelperText>
            )}
          </FormGroup>
        </>
      )}

      {/* Action Buttons */}
      <FormGroup>
        <Button
          type="submit"
          variant={ButtonVariant.primary}
          isLoading={isSubmitting}
          isDisabled={isSubmitting}
        >
          {initialValues ? 'Update Rule' : 'Add Rule'}
        </Button>
        <Button
          type="button"
          variant={ButtonVariant.link}
          onClick={onCancel}
          isDisabled={isSubmitting}
        >
          Cancel
        </Button>
      </FormGroup>
    </Form>
  );
};
