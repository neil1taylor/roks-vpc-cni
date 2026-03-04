import React, { useState, useEffect, useCallback, useRef } from 'react';
import './topology.css';
import {
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  ToolbarGroup,
  Button,
  ButtonVariant,
  Spinner,
  Alert,
  AlertVariant,
  Drawer,
  DrawerContent,
  DrawerPanelContent,
  DrawerHead,
  DrawerActions,
  DrawerCloseButton,
  Title,
  Divider,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Checkbox,
  Split,
  SplitItem,
  Switch,
  Flex,
  FlexItem,
  Label,
} from '@patternfly/react-core';
import {
  CompressIcon,
  UndoIcon,
  SearchPlusIcon,
  SearchMinusIcon,
} from '@patternfly/react-icons';
import {
  VisualizationSurface,
  TopologyView,
  useVisualizationController,
  VisualizationProvider,
  ColaLayout,
  DefaultNode,
  DefaultGroup,
  DefaultEdge,
  ModelKind,
  GraphComponent,
  withPanZoom,
  withSelection,
  ComponentFactory,
  Graph,
} from '@patternfly/react-topology';
import { apiClient } from '../api/client';
import { useClusterInfo } from '../api/hooks';
import {
  transformTopologyData,
  createTopologyGraph,
  findTopologyNode,
  getConnectedNodes,
  TopologyData,
  TopologyStats,
  calculateTopologyStats,
} from './layout';
import { NODE_TYPES, getNodeTypeLabel, HealthStatus } from './nodes';

interface SelectedNodeInfo {
  id: string;
  name: string;
  type: string;
  resourceId?: string;
  details?: Record<string, string>;
  connectedNodeCount: number;
  healthStatus?: HealthStatus;
}

/** Auto-refresh interval in milliseconds (30 seconds) */
const HEALTH_REFRESH_INTERVAL = 30000;

/** Health status color mapping */
const HEALTH_COLORS: Record<HealthStatus, string> = {
  healthy: '#3E8635',   // PatternFly green-500
  warning: '#F0AB00',   // PatternFly gold-400
  critical: '#C9190B',  // PatternFly red-500
};

/** Health status label mapping */
const HEALTH_LABELS: Record<HealthStatus, string> = {
  healthy: 'Healthy',
  warning: 'Warning',
  critical: 'Critical',
};

/** Leaf node types shown in the filter toolbar */
const FILTER_NODE_TYPES = [
  NODE_TYPES.VNI,
  NODE_TYPES.FLOATING_IP,
  NODE_TYPES.SECURITY_GROUP,
  NODE_TYPES.ACL,
  NODE_TYPES.CUDN,
  NODE_TYPES.UDN,
];

const layoutFactory = (type: string, graph: Graph) => {
  if (type === 'Cola') {
    return new ColaLayout(graph, {
      layoutOnDrag: false,
    });
  }
  return undefined;
};

const componentFactory: ComponentFactory = (kind: ModelKind, type: string) => {
  switch (kind) {
    case ModelKind.graph:
      return withPanZoom()(GraphComponent);
    case ModelKind.node:
      if (type === 'vpc-group' || type === 'subnet-group') {
        return DefaultGroup;
      }
      return withSelection()(DefaultNode);
    case ModelKind.edge:
      return DefaultEdge;
    default:
      return undefined;
  }
};

const TopologyViewerInner: React.FC = () => {
  const controller = useVisualizationController();
  const { clusterInfo } = useClusterInfo();
  const isROKSManaged =
    clusterInfo?.features.vniManagement === false &&
    clusterInfo?.features.roksAPIAvailable === false;

  // In ROKS mode, exclude VNI from the filter list
  const filterNodeTypes = isROKSManaged
    ? FILTER_NODE_TYPES.filter((t) => t !== NODE_TYPES.VNI)
    : FILTER_NODE_TYPES;

  const [topologyData, setTopologyData] = useState<TopologyData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<SelectedNodeInfo | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [stats, setStats] = useState<TopologyStats | null>(null);
  const [visibleTypes, setVisibleTypes] = useState<Set<string>>(
    new Set(FILTER_NODE_TYPES)
  );
  const [autoRefresh, setAutoRefresh] = useState(false);
  const refreshTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Register layout and component factories
  useEffect(() => {
    if (controller) {
      controller.registerLayoutFactory(layoutFactory);
      controller.registerComponentFactory(componentFactory);
    }
  }, [controller]);

  // Shared fetch function for topology data
  const fetchTopology = useCallback(async (includeHealth?: boolean) => {
    try {
      const response = await apiClient.getTopology(undefined, includeHealth);
      if (response.data) {
        const topoData: TopologyData = {
          nodes: response.data.nodes.map((n) => ({
            id: n.id,
            name: n.label,
            type: n.type as any,
            status: n.status,
            details: n.metadata as Record<string, string> | undefined,
            health: n.health ? { status: n.health.status, metrics: n.health.metrics } : undefined,
          })),
          edges: response.data.edges.map((e) => ({
            id: e.id,
            source: e.source,
            target: e.target,
            type: e.type,
          })),
        };
        setTopologyData(topoData);
        setStats(calculateTopologyStats(topoData));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load topology');
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    setIsLoading(true);
    setError(null);
    fetchTopology(false).finally(() => setIsLoading(false));
  }, [fetchTopology]);

  // Auto-refresh with health data
  useEffect(() => {
    if (autoRefresh) {
      // Fetch immediately with health when toggling on
      fetchTopology(true);
      refreshTimerRef.current = setInterval(() => {
        fetchTopology(true);
      }, HEALTH_REFRESH_INTERVAL);
    } else {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    }
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [autoRefresh, fetchTopology]);

  // Transform and render topology when data changes
  useEffect(() => {
    if (!topologyData || !controller) {
      return;
    }

    let fitTimer: ReturnType<typeof setTimeout> | undefined;

    try {
      const transformedData = transformTopologyData(topologyData);

      // Filter leaf nodes based on visible types (groups always visible)
      const filteredNodes = transformedData.nodes.filter((node) => {
        if (node.group) return true; // always show groups
        const nodeType = (node.data as any)?.nodeType || node.type;
        return visibleTypes.has(nodeType);
      });

      // Rebuild group children to exclude hidden nodes
      const visibleNodeIds = new Set(filteredNodes.map((n) => n.id));
      const adjustedNodes = filteredNodes.map((node) => {
        if (node.group && node.children) {
          return {
            ...node,
            children: node.children.filter((childId) => visibleNodeIds.has(childId)),
          };
        }
        return node;
      });

      // Remove empty groups (groups with no visible children)
      const nonEmptyNodes = adjustedNodes.filter((node) => {
        if (node.group && (!node.children || node.children.length === 0)) {
          visibleNodeIds.delete(node.id);
          return false;
        }
        return true;
      });

      // Filter edges to only include those where both ends are visible
      const filteredEdges = transformedData.edges.filter(
        (edge) => visibleNodeIds.has(edge.source!) && visibleNodeIds.has(edge.target!)
      );

      const model = createTopologyGraph(nonEmptyNodes, filteredEdges);
      controller.fromModel(model, true);

      // DagreGroups layout is synchronous — short timeout for fit
      fitTimer = setTimeout(() => {
        try {
          controller.getGraph().fit(80);
        } catch {
          // graph may not be ready yet
        }
      }, 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to render topology');
    }

    return () => {
      if (fitTimer) clearTimeout(fitTimer);
    };
  }, [topologyData, controller, visibleTypes]);

  // Register node click handler on controller
  useEffect(() => {
    if (!controller || !topologyData) return;

    const onNodeSelect = (nodeId: string) => {
      const node = findTopologyNode(topologyData, nodeId);
      if (!node) return;

      const connectedIds = getConnectedNodes(topologyData, nodeId, 'both');

      setSelectedNode({
        id: node.id,
        name: node.name,
        type: node.type,
        resourceId: node.resourceId,
        details: node.details,
        connectedNodeCount: connectedIds.length,
        healthStatus: node.health?.status as HealthStatus | undefined,
      });
      setIsDrawerOpen(true);
    };

    controller.addEventListener('node:click', onNodeSelect);
    return () => { controller.removeEventListener('node:click', onNodeSelect); };
  }, [controller, topologyData]);

  const handleZoomIn = useCallback(() => {
    if (controller) {
      controller.getGraph().scaleBy(1.2);
    }
  }, [controller]);

  const handleZoomOut = useCallback(() => {
    if (controller) {
      controller.getGraph().scaleBy(1 / 1.2);
    }
  }, [controller]);

  const handleFitToScreen = useCallback(() => {
    if (controller) {
      controller.getGraph().fit(50);
    }
  }, [controller]);

  const handleResetLayout = useCallback(() => {
    if (topologyData && controller) {
      try {
        const transformedData = transformTopologyData(topologyData);

        const filteredNodes = transformedData.nodes.filter((node) => {
          if (node.group) return true;
          const nodeType = (node.data as any)?.nodeType || node.type;
          return visibleTypes.has(nodeType);
        });

        const visibleNodeIds = new Set(filteredNodes.map((n) => n.id));
        const adjustedNodes = filteredNodes.map((node) => {
          if (node.group && node.children) {
            return { ...node, children: node.children.filter((id) => visibleNodeIds.has(id)) };
          }
          return node;
        });

        const nonEmptyNodes = adjustedNodes.filter((node) => {
          if (node.group && (!node.children || node.children.length === 0)) {
            visibleNodeIds.delete(node.id);
            return false;
          }
          return true;
        });

        const filteredEdges = transformedData.edges.filter(
          (edge) => visibleNodeIds.has(edge.source!) && visibleNodeIds.has(edge.target!)
        );

        const model = createTopologyGraph(nonEmptyNodes, filteredEdges);
        controller.fromModel(model, true);
        controller.getGraph().layout();
        setTimeout(() => {
          try {
            controller.getGraph().fit(80);
          } catch {
            // ignore
          }
        }, 1500);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to reset layout');
      }
    }
  }, [topologyData, controller, visibleTypes]);

  const handleTypeToggle = useCallback(
    (nodeType: string) => {
      const newTypes = new Set(visibleTypes);
      if (newTypes.has(nodeType)) {
        newTypes.delete(nodeType);
      } else {
        newTypes.add(nodeType);
      }
      setVisibleTypes(newTypes);
    },
    [visibleTypes]
  );

  const topologyToolbar = (
    <Toolbar>
      <ToolbarContent>
        <ToolbarItem>
          <Split hasGutter>
            <SplitItem>
              <Button
                icon={<SearchPlusIcon />}
                variant={ButtonVariant.control}
                onClick={handleZoomIn}
                aria-label="Zoom in"
              />
            </SplitItem>
            <SplitItem>
              <Button
                icon={<SearchMinusIcon />}
                variant={ButtonVariant.control}
                onClick={handleZoomOut}
                aria-label="Zoom out"
              />
            </SplitItem>
            <SplitItem>
              <Button
                icon={<CompressIcon />}
                variant={ButtonVariant.control}
                onClick={handleFitToScreen}
                aria-label="Fit to screen"
              />
            </SplitItem>
            <SplitItem>
              <Button
                icon={<UndoIcon />}
                variant={ButtonVariant.control}
                onClick={handleResetLayout}
                aria-label="Reset layout"
              />
            </SplitItem>
          </Split>
        </ToolbarItem>

        <ToolbarItem>
          <div style={{ marginLeft: '16px', display: 'flex', gap: '8px' }}>
            {filterNodeTypes.map((nodeType) => (
              <Checkbox
                key={nodeType}
                id={`filter-${nodeType}`}
                label={getNodeTypeLabel(nodeType)}
                isChecked={visibleTypes.has(nodeType)}
                onChange={() => handleTypeToggle(nodeType)}
              />
            ))}
          </div>
        </ToolbarItem>

        <ToolbarItem variant="separator" />

        <ToolbarGroup>
          <ToolbarItem>
            <Switch
              id="auto-refresh-toggle"
              label="Health monitoring"
              isChecked={autoRefresh}
              onChange={(_event, checked) => setAutoRefresh(checked)}
              isReversed
            />
          </ToolbarItem>

          {autoRefresh && (
            <ToolbarItem>
              <Flex spaceItems={{ default: 'spaceItemsSm' }} alignItems={{ default: 'alignItemsCenter' }}>
                {(Object.keys(HEALTH_COLORS) as HealthStatus[]).map((status) => (
                  <FlexItem key={status}>
                    <Flex spaceItems={{ default: 'spaceItemsXs' }} alignItems={{ default: 'alignItemsCenter' }}>
                      <FlexItem>
                        <span
                          style={{
                            display: 'inline-block',
                            width: '10px',
                            height: '10px',
                            borderRadius: '50%',
                            backgroundColor: HEALTH_COLORS[status],
                          }}
                        />
                      </FlexItem>
                      <FlexItem>
                        <span style={{ fontSize: '0.8rem' }}>{HEALTH_LABELS[status]}</span>
                      </FlexItem>
                    </Flex>
                  </FlexItem>
                ))}
              </Flex>
            </ToolbarItem>
          )}
        </ToolbarGroup>

        {stats && (
          <ToolbarItem>
            <div style={{ fontSize: '0.875rem', color: '#666' }}>
              {stats.totalNodes} nodes | {stats.totalEdges} connections
            </div>
          </ToolbarItem>
        )}
      </ToolbarContent>
    </Toolbar>
  );

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant={AlertVariant.danger} title="Error" isInline>
        {error}
      </Alert>
    );
  }

  const drawerPanel = isDrawerOpen && selectedNode ? (
    <DrawerPanelContent>
      <DrawerHead>
        <Title headingLevel="h2">{selectedNode.name}</Title>
        <DrawerActions>
          <DrawerCloseButton onClick={() => setIsDrawerOpen(false)} />
        </DrawerActions>
      </DrawerHead>
      <Divider />
      <div style={{ padding: '16px' }}>
        <DescriptionList>
          <DescriptionListGroup>
            <DescriptionListTerm>Type</DescriptionListTerm>
            <DescriptionListDescription>
              {getNodeTypeLabel(selectedNode.type as any)}
            </DescriptionListDescription>
          </DescriptionListGroup>

          {selectedNode.resourceId && (
            <DescriptionListGroup>
              <DescriptionListTerm>Resource ID</DescriptionListTerm>
              <DescriptionListDescription>
                <code>{selectedNode.resourceId}</code>
              </DescriptionListDescription>
            </DescriptionListGroup>
          )}

          {selectedNode.healthStatus && (
            <DescriptionListGroup>
              <DescriptionListTerm>Health</DescriptionListTerm>
              <DescriptionListDescription>
                <Label
                  color={
                    selectedNode.healthStatus === 'healthy' ? 'green' :
                    selectedNode.healthStatus === 'warning' ? 'gold' :
                    'red'
                  }
                >
                  {HEALTH_LABELS[selectedNode.healthStatus]}
                </Label>
              </DescriptionListDescription>
            </DescriptionListGroup>
          )}

          <DescriptionListGroup>
            <DescriptionListTerm>Connected Resources</DescriptionListTerm>
            <DescriptionListDescription>
              {selectedNode.connectedNodeCount}
            </DescriptionListDescription>
          </DescriptionListGroup>

          {selectedNode.details && Object.keys(selectedNode.details).length > 0 && (
            <>
              <Divider style={{ margin: '16px 0' }} />
              <Title headingLevel="h3" size="md">
                Details
              </Title>
              {Object.entries(selectedNode.details).map(([key, value]) => (
                <DescriptionListGroup key={key}>
                  <DescriptionListTerm>{key}</DescriptionListTerm>
                  <DescriptionListDescription>{value}</DescriptionListDescription>
                </DescriptionListGroup>
              ))}
            </>
          )}
        </DescriptionList>
      </div>
    </DrawerPanelContent>
  ) : undefined;

  return (
    <div style={{ height: 'calc(100vh - 220px)', display: 'flex', flexDirection: 'column' }}>
      {isROKSManaged && (
        <Alert variant={AlertVariant.info} isInline title="ROKS-managed VNIs" isPlain style={{ marginBottom: '8px' }}>
          VNI nodes reflect Kubernetes CRD status. VNI details are managed by the ROKS platform.
        </Alert>
      )}
      <Drawer isExpanded={isDrawerOpen}>
        <DrawerContent panelContent={drawerPanel}>
          <TopologyView viewToolbar={topologyToolbar}>
            <VisualizationSurface state={{}} />
          </TopologyView>
        </DrawerContent>
      </Drawer>
    </div>
  );
};

export const TopologyViewer: React.FC = () => {
  return (
    <VisualizationProvider>
      <TopologyViewerInner />
    </VisualizationProvider>
  );
};
