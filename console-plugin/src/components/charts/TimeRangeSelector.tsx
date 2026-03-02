import React from 'react';
import { ToggleGroup, ToggleGroupItem } from '@patternfly/react-core';

export interface TimeRange {
  label: string;
  range: string;
  step: string;
}

const TIME_RANGES: TimeRange[] = [
  { label: '5m', range: '5m', step: '15s' },
  { label: '15m', range: '15m', step: '30s' },
  { label: '1h', range: '1h', step: '1m' },
  { label: '6h', range: '6h', step: '5m' },
  { label: '24h', range: '24h', step: '15m' },
];

interface TimeRangeSelectorProps {
  selected: string;
  onSelect: (range: string, step: string) => void;
}

const TimeRangeSelector: React.FC<TimeRangeSelectorProps> = ({ selected, onSelect }) => (
  <ToggleGroup aria-label="Time range selector">
    {TIME_RANGES.map((tr) => (
      <ToggleGroupItem
        key={tr.range}
        text={tr.label}
        buttonId={`tr-${tr.range}`}
        isSelected={selected === tr.range}
        onChange={() => onSelect(tr.range, tr.step)}
      />
    ))}
  </ToggleGroup>
);

export default TimeRangeSelector;
