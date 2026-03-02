import React from 'react';
import { ChartDonutUtilization, ChartThemeColor } from '@patternfly/react-charts';
import { Card, CardBody, CardTitle } from '@patternfly/react-core';
import { DHCPPoolMetrics } from '../../api/types';

interface DHCPPoolGaugeProps {
  pool: DHCPPoolMetrics;
  height?: number;
  width?: number;
}

const thresholds = [
  { value: 60, color: ChartThemeColor.green },
  { value: 80, color: ChartThemeColor.gold },
  { value: 100, color: ChartThemeColor.orange },
];

const DHCPPoolGauge: React.FC<DHCPPoolGaugeProps> = ({ pool, height = 180, width = 180 }) => {
  const pct = Math.round(pool.utilization * 100) / 100;

  return (
    <Card isCompact>
      <CardTitle>{pool.name} DHCP Pool</CardTitle>
      <CardBody>
        <div style={{ height, width, margin: '0 auto' }}>
          <ChartDonutUtilization
            data={{ x: 'Leases', y: pct }}
            height={height}
            width={width}
            title={`${pct}%`}
            subTitle={`${pool.activeLeases} / ${pool.poolSize}`}
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

export { thresholds };
export default DHCPPoolGauge;
