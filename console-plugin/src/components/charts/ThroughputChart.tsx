import React from 'react';
import { Chart, ChartArea, ChartAxis, ChartGroup, ChartLegend, ChartThemeColor, ChartVoronoiContainer } from '@patternfly/react-charts';
import { Card, CardBody, CardTitle } from '@patternfly/react-core';
import { InterfaceTimeSeries } from '../../api/types';
import { formatBytesPerSec } from '../../utils/formatters';

interface ThroughputChartProps {
  data: InterfaceTimeSeries;
  height?: number;
}

const ThroughputChart: React.FC<ThroughputChartProps> = ({ data, height = 250 }) => {
  const rxData = (data.rxBps || []).map((p) => ({ x: new Date(p.t * 1000), y: p.v }));
  const txData = (data.txBps || []).map((p) => ({ x: new Date(p.t * 1000), y: p.v }));

  if (rxData.length === 0 && txData.length === 0) {
    return (
      <Card isCompact>
        <CardTitle>{data.name} — Throughput</CardTitle>
        <CardBody>No data available</CardBody>
      </Card>
    );
  }

  const formatTime = (d: Date) => d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  return (
    <Card isCompact>
      <CardTitle>{data.name} — Throughput</CardTitle>
      <CardBody>
        <div style={{ height, width: '100%' }}>
          <Chart
            height={height}
            padding={{ top: 10, bottom: 40, left: 80, right: 20 }}
            themeColor={ChartThemeColor.multiUnordered}
            containerComponent={
              <ChartVoronoiContainer
                labels={({ datum }: { datum: { childName?: string; y?: number } }) =>
                  `${datum.childName === 'rx' ? 'RX' : 'TX'}: ${formatBytesPerSec(datum.y)}`
                }
              />
            }
          >
            <ChartAxis tickFormat={formatTime} fixLabelOverlap />
            <ChartAxis dependentAxis tickFormat={formatBytesPerSec} />
            <ChartGroup>
              <ChartArea name="rx" data={rxData} style={{ data: { fillOpacity: 0.2 } }} />
              <ChartArea name="tx" data={txData} style={{ data: { fillOpacity: 0.2 } }} />
            </ChartGroup>
            <ChartLegend
              data={[{ name: 'RX' }, { name: 'TX' }]}
              orientation="horizontal"
              gutter={20}
            />
          </Chart>
        </div>
      </CardBody>
    </Card>
  );
};

export default ThroughputChart;
