import React, { useState } from 'react';
import {
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Spinner,
  Switch,
  Stack,
  StackItem,
  Title,
} from '@patternfly/react-core';
import {
  Chart,
  ChartArea,
  ChartAxis,
  ChartGroup,
  ChartLegend,
  ChartThemeColor,
  ChartVoronoiContainer,
  ChartDonutUtilization,
} from '@patternfly/react-charts';
import TimeRangeSelector from './charts/TimeRangeSelector';
import { useSubnetMetrics } from '../api/hooks';
import { formatBytesPerSec } from '../utils/formatters';

interface SubnetMetricsTabProps {
  subnetName: string;
  namespace?: string;
}

const SubnetMetricsTab: React.FC<SubnetMetricsTabProps> = ({ subnetName, namespace }) => {
  const [range, setRange] = useState('1h');
  const [, setStep] = useState('1m');
  const [autoRefresh, setAutoRefresh] = useState(true);

  const { data: metrics, loading, error } = useSubnetMetrics(
    autoRefresh ? subnetName : '',
    namespace,
    range,
  );

  // When auto-refresh is toggled off, we still want to keep the last data visible.
  // useBFFDataPolling returns null when name is empty, but we only set name to empty
  // when autoRefresh is off. So let's use subnetName always but control via polling hook.
  const { data: staticMetrics } = useSubnetMetrics(
    !autoRefresh ? subnetName : '',
    namespace,
    range,
  );

  const displayMetrics = autoRefresh ? metrics : (staticMetrics || metrics);

  const handleRangeChange = (newRange: string, newStep: string) => {
    setRange(newRange);
    setStep(newStep);
  };

  const formatTime = (d: Date) =>
    d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  const rxData = (displayMetrics?.throughputRx || []).map((p) => ({
    x: new Date(p.t * 1000),
    y: p.v,
  }));
  const txData = (displayMetrics?.throughputTx || []).map((p) => ({
    x: new Date(p.t * 1000),
    y: p.v,
  }));

  const dhcpPoolSize = displayMetrics?.dhcpPoolSize ?? 0;
  const dhcpActive = displayMetrics?.dhcpActiveLeases ?? 0;
  const dhcpUtilPct = Math.round((displayMetrics?.dhcpUtilizationPct ?? 0) * 100) / 100;
  const hasDHCP = dhcpPoolSize > 0;
  const hasThroughput = rxData.length > 0 || txData.length > 0;

  return (
    <Stack hasGutter>
      <StackItem>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <TimeRangeSelector selected={range} onSelect={handleRangeChange} />
          <Switch
            id="subnet-metrics-auto-refresh"
            label="Auto-refresh"
            isChecked={autoRefresh}
            onChange={(_event, checked) => setAutoRefresh(checked)}
          />
        </div>
      </StackItem>

      {loading && !displayMetrics && (
        <StackItem>
          <div style={{ textAlign: 'center', padding: '40px' }}>
            <Spinner size="lg" />
          </div>
        </StackItem>
      )}

      {error && (
        <StackItem>
          <Card>
            <CardBody>
              Failed to load subnet metrics: {error.message}
            </CardBody>
          </Card>
        </StackItem>
      )}

      {displayMetrics && (
        <>
          {/* Throughput Chart */}
          <StackItem>
            <Card>
              <CardTitle>Throughput</CardTitle>
              <CardBody>
                {hasThroughput ? (
                  <div style={{ height: 280, width: '100%' }}>
                    <Chart
                      height={280}
                      padding={{ top: 10, bottom: 50, left: 80, right: 20 }}
                      themeColor={ChartThemeColor.multiUnordered}
                      containerComponent={
                        <ChartVoronoiContainer
                          labels={({
                            datum,
                          }: {
                            datum: { childName?: string; y?: number };
                          }) =>
                            `${datum.childName === 'rx' ? 'RX' : 'TX'}: ${formatBytesPerSec(datum.y)}`
                          }
                        />
                      }
                    >
                      <ChartAxis tickFormat={formatTime} fixLabelOverlap />
                      <ChartAxis dependentAxis tickFormat={formatBytesPerSec} />
                      <ChartGroup>
                        <ChartArea
                          name="rx"
                          data={rxData}
                          style={{ data: { fillOpacity: 0.2 } }}
                        />
                        <ChartArea
                          name="tx"
                          data={txData}
                          style={{ data: { fillOpacity: 0.2 } }}
                        />
                      </ChartGroup>
                      <ChartLegend
                        data={[{ name: 'RX' }, { name: 'TX' }]}
                        orientation="horizontal"
                        gutter={20}
                      />
                    </Chart>
                  </div>
                ) : (
                  <div style={{ padding: '24px', textAlign: 'center', color: '#6a6e73' }}>
                    No throughput data available. The subnet may not be attached to a router,
                    or metrics collection has not started.
                  </div>
                )}
              </CardBody>
            </Card>
          </StackItem>

          {/* DHCP Pool Utilization */}
          <StackItem>
            <Card>
              <CardTitle>DHCP Pool Utilization</CardTitle>
              <CardBody>
                {hasDHCP ? (
                  <Grid hasGutter>
                    <GridItem span={4}>
                      <div style={{ height: 200, width: 200, margin: '0 auto' }}>
                        <ChartDonutUtilization
                          data={{ x: 'Leases', y: dhcpUtilPct }}
                          height={200}
                          width={200}
                          title={`${dhcpUtilPct}%`}
                          subTitle={`${dhcpActive} / ${dhcpPoolSize}`}
                          thresholds={[
                            { value: 60, color: '#3E8635' },
                            { value: 80, color: '#F0AB00' },
                            { value: 100, color: '#C9190B' },
                          ]}
                        />
                      </div>
                    </GridItem>
                    <GridItem span={8}>
                      <Stack hasGutter>
                        <StackItem>
                          <Title headingLevel="h4">Pool Details</Title>
                        </StackItem>
                        <StackItem>
                          <strong>Pool size:</strong> {dhcpPoolSize} addresses
                        </StackItem>
                        <StackItem>
                          <strong>Active leases:</strong> {dhcpActive}
                        </StackItem>
                        <StackItem>
                          <strong>Utilization:</strong> {dhcpUtilPct}%
                        </StackItem>
                      </Stack>
                    </GridItem>
                  </Grid>
                ) : (
                  <div style={{ padding: '24px', textAlign: 'center', color: '#6a6e73' }}>
                    No DHCP pool data available. DHCP may not be enabled for this subnet on the router.
                  </div>
                )}
              </CardBody>
            </Card>
          </StackItem>
        </>
      )}

      {!loading && !displayMetrics && !error && (
        <StackItem>
          <Card>
            <CardBody>
              <div style={{ padding: '24px', textAlign: 'center', color: '#6a6e73' }}>
                No metrics data available for this subnet. Ensure that:
                <ul style={{ listStyle: 'disc', textAlign: 'left', maxWidth: '500px', margin: '12px auto' }}>
                  <li>The subnet is attached to a VPCRouter via spec.networks</li>
                  <li>The router metrics exporter sidecar is running</li>
                  <li>Thanos/Prometheus is configured (THANOS_URL environment variable)</li>
                </ul>
              </div>
            </CardBody>
          </Card>
        </StackItem>
      )}
    </Stack>
  );
};

SubnetMetricsTab.displayName = 'SubnetMetricsTab';

export default SubnetMetricsTab;
