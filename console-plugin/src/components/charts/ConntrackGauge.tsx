import React from 'react';
import { ChartDonutUtilization } from '@patternfly/react-charts';
import { Card, CardBody, CardTitle } from '@patternfly/react-core';
import { formatNumber } from '../../utils/formatters';

interface ConntrackGaugeProps {
  entries: number;
  max: number;
  percentage: number;
  height?: number;
  width?: number;
}

const ConntrackGauge: React.FC<ConntrackGaugeProps> = ({
  entries,
  max,
  percentage,
  height = 180,
  width = 180,
}) => {
  const pct = Math.round(percentage * 100) / 100;

  return (
    <Card isCompact>
      <CardTitle>Conntrack Usage</CardTitle>
      <CardBody>
        <div style={{ height, width, margin: '0 auto' }}>
          <ChartDonutUtilization
            data={{ x: 'Entries', y: pct }}
            height={height}
            width={width}
            title={`${pct}%`}
            subTitle={`${formatNumber(entries)} / ${formatNumber(max)}`}
            thresholds={[
              { value: 60, color: '#3E8635' },
              { value: 80, color: '#F0AB00' },
              { value: 100, color: '#C9190B' },
            ]}
          />
        </div>
      </CardBody>
    </Card>
  );
};

export default ConntrackGauge;
