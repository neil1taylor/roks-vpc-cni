import React, { useState } from 'react';
import {
  PageSection,
  EmptyState,
  EmptyStateBody,
  Grid,
  GridItem,
  FormSelect,
  FormSelectOption,
  Split,
  SplitItem,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { useRouters, useRouterHealth, useRouterInterfaces, useRouterConntrack, useRouterDHCP, useRouterNFT } from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import RouterHealthCard from '../components/charts/RouterHealthCard';
import ThroughputChart from '../components/charts/ThroughputChart';
import ConntrackGauge from '../components/charts/ConntrackGauge';
import DHCPPoolGauge from '../components/charts/DHCPPoolGauge';
import NFTCountersTable from '../components/charts/NFTCountersTable';
import TimeRangeSelector from '../components/charts/TimeRangeSelector';

const ObservabilityPage: React.FC = () => {
  const { routers, loading: routersLoading } = useRouters();
  const metricsRouters = (routers || []).filter((r) => r.metricsEnabled);

  const [selectedRouter, setSelectedRouter] = useState<string>('');
  const [selectedNs, setSelectedNs] = useState<string>('');
  const [range, setRange] = useState('1h');
  const [step, setStep] = useState('1m');

  // Auto-select first metrics-enabled router
  const activeRouter = selectedRouter || metricsRouters[0]?.name || '';
  const activeNs = selectedNs || metricsRouters.find((r) => r.name === activeRouter)?.namespace || '';

  const { data: health, loading: healthLoading } = useRouterHealth(activeRouter, activeNs);
  const { data: interfaces } = useRouterInterfaces(activeRouter, activeNs, range, step);
  const { data: conntrack } = useRouterConntrack(activeRouter, activeNs, range, step);
  const { data: dhcpPools } = useRouterDHCP(activeRouter, activeNs);
  const { data: nftRules } = useRouterNFT(activeRouter, activeNs);

  if (routersLoading) {
    return (
      <VPCNetworkingShell>
        <PageSection><Spinner size="lg" /></PageSection>
      </VPCNetworkingShell>
    );
  }

  if (metricsRouters.length === 0) {
    return (
      <VPCNetworkingShell>
        <PageSection>
          <EmptyState>
            <Title headingLevel="h2" size="lg">No Metrics Available</Title>
            <EmptyStateBody>
              No routers have metrics enabled. Set <code>spec.metrics.enabled: true</code> on a
              VPCRouter to start collecting observability data.
            </EmptyStateBody>
          </EmptyState>
        </PageSection>
      </VPCNetworkingShell>
    );
  }

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Split hasGutter>
          <SplitItem>
            <FormSelect
              value={activeRouter}
              onChange={(_e, val) => {
                setSelectedRouter(val);
                const r = metricsRouters.find((rt) => rt.name === val);
                setSelectedNs(r?.namespace || '');
              }}
              aria-label="Select router"
              style={{ width: '250px' }}
            >
              {metricsRouters.map((r) => (
                <FormSelectOption key={`${r.namespace}/${r.name}`} value={r.name} label={`${r.name} (${r.namespace})`} />
              ))}
            </FormSelect>
          </SplitItem>
          <SplitItem isFilled />
          <SplitItem>
            <TimeRangeSelector selected={range} onSelect={(r, s) => { setRange(r); setStep(s); }} />
          </SplitItem>
        </Split>
      </PageSection>

      <PageSection>
        <Grid hasGutter>
          <GridItem span={12}>
            <RouterHealthCard health={health} loading={healthLoading} />
          </GridItem>

          {conntrack && (
            <GridItem span={4}>
              <ConntrackGauge entries={conntrack.entries?.[conntrack.entries.length - 1]?.v || 0} max={conntrack.max} percentage={conntrack.percentage} />
            </GridItem>
          )}

          {dhcpPools && dhcpPools.length > 0 && dhcpPools.map((pool) => (
            <GridItem span={4} key={pool.name}>
              <DHCPPoolGauge pool={pool} />
            </GridItem>
          ))}
        </Grid>
      </PageSection>

      {interfaces && interfaces.length > 0 && (
        <PageSection>
          <Title headingLevel="h3" style={{ marginBottom: '16px' }}>Interface Throughput</Title>
          {interfaces.map((iface) => (
            <div key={iface.name} style={{ marginBottom: '16px' }}>
              <ThroughputChart data={iface} />
            </div>
          ))}
        </PageSection>
      )}

      {nftRules && nftRules.length > 0 && (
        <PageSection>
          <NFTCountersTable rules={nftRules} />
        </PageSection>
      )}
    </VPCNetworkingShell>
  );
};

export default ObservabilityPage;
