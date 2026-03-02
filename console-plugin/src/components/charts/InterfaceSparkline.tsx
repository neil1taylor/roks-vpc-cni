import React from 'react';
import { ChartArea, ChartGroup, ChartThemeColor } from '@patternfly/react-charts';
import { DataPoint } from '../../api/types';

interface InterfaceSparklineProps {
  rxBps: DataPoint[];
  txBps: DataPoint[];
  width?: number;
  height?: number;
}

const InterfaceSparkline: React.FC<InterfaceSparklineProps> = ({
  rxBps,
  txBps,
  width = 120,
  height = 40,
}) => {
  const rxData = (rxBps || []).map((p) => ({ x: p.t, y: p.v }));
  const txData = (txBps || []).map((p) => ({ x: p.t, y: p.v }));

  if (rxData.length === 0 && txData.length === 0) {
    return <span style={{ color: 'var(--pf-v5-global--Color--200)' }}>—</span>;
  }

  return (
    <div style={{ width, height }}>
      <ChartGroup
        height={height}
        width={width}
        padding={0}
        themeColor={ChartThemeColor.multiUnordered}
      >
        <ChartArea data={rxData} style={{ data: { fillOpacity: 0.3, strokeWidth: 1 } }} />
        <ChartArea data={txData} style={{ data: { fillOpacity: 0.3, strokeWidth: 1 } }} />
      </ChartGroup>
    </div>
  );
};

export default InterfaceSparkline;
