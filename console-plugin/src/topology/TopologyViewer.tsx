import React, { useState, useEffect, useCallback } from 'react';
import {
  Toolbar,
  ToolbarContent,
  ToolbarItem,
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
} from '@patternfly/react-topology';
import { apiClient } from '../api/client';
import {
  transformTopologyData,
  createTopologyGraph,
  findTopologyNode,
  getConnectedNodes,
  TopologyData,
  TopologyStats,
  calculateTopologyStats,
} from './layout';
import { NODE_TYPES, getNodeTypeLabel } from './nodes';

interface SelectedNodeInfo {
  id: string;
  name: string;
  type: string;
  resourceId?: string;
  details?: Record<string, string>;
  connectedNodeCount: number;
}

const TOPOLOGY_NODE_TYPES = [
  NODE_TYPES.VPC,
  NODE_TYPES.SUBNET,
  NODE_TYPES.VNI,
  NODE_TYPES.VM,
  NODE_TYPES.SECURITY_GROUP,
  NODE_TYPES.ACL,
];

const TopologyViewerInner: React.FC = () => {
  const controller = useVisualizationController();
  const [topologyData, setTopologyData] = useState<TopologyData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<SelectedNodeInfo | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [stats, setStats] = useState<TopologyStats | null>(null);
  const [visibleTypes, setVisibleTypes] = useState<Set<string>>(
    new Set(TOPOLOGY_NODE_TYPES)
  );

  // Fetch topology data
  useEffect(() => {
    const fetchTopology = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const response = await apiClient.getTopology();
        if (response.data) {
          const topoData: TopologyData = {
            nodes: response.data.nodes.map((n) => ({
              id: n.id,
              name: n.label,
              type: n.type as any,
              details: n.metadata as Record<string, string> | undefined,
            })),
            edges: response.data.edges.map((e) => ({
              id: e.id,
              source: e.source,
              target: e.target,
            })),
          };
          setTopologyData(topoData);
          setStats(calculateTopologyStats(topoData));
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load topology');
      } finally {
        setIsLoading(false);
      }
    };

    fetchTopology();
  }, []);

  // Transform and render topology when data changes
  useEffect(() => {
    if (!topologyData || !controller) {
      return;
    }

    try {
      const transformedData = transformTopologyData(topologyData);

      // Filter nodes based on visible types
      const filteredNodes = transformedData.nodes.filter((node) =>
        visibleTypes.has(node.type)
      );

      // Filter edges to only include those where both source and target are visible
      const visibleNodeIds = new Set(filteredNodes.map((n) => n.id));
      const filteredEdges = transformedData.edges.filter(
        (edge) => visibleNodeIds.has(edge.source!) && visibleNodeIds.has(edge.target!)
      );

      const model = createTopologyGraph(filteredNodes, filteredEdges);
      controller.fromModel(model, true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to render topology');
    }
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

        const filteredNodes = transformedData.nodes.filter((node) =>
          visibleTypes.has(node.type)
        );

        const visibleNodeIds = new Set(filteredNodes.map((n) => n.id));
        const filteredEdges = transformedData.edges.filter(
          (edge) => visibleNodeIds.has(edge.source!) && visibleNodeIds.has(edge.target!)
        );

        const model = createTopologyGraph(filteredNodes, filteredEdges);
        controller.fromModel(model, true);
        controller.getGraph().layout();
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

  return (
    <Drawer isExpanded={isDrawerOpen}>
      <DrawerContent
        panelContent={
          isDrawerOpen && selectedNode ? (
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
          ) : null
        }
      >
        <TopologyView
          controlBar={false}
          sideBar={false}
          sideBarResizable={false}
        >
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
                  {TOPOLOGY_NODE_TYPES.map((nodeType) => (
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

              {stats && (
                <ToolbarItem>
                  <div style={{ fontSize: '0.875rem', color: '#666' }}>
                    {stats.totalNodes} nodes | {stats.totalEdges} connections
                  </div>
                </ToolbarItem>
              )}
            </ToolbarContent>
          </Toolbar>

          {isLoading && (
            <div style={{ textAlign: 'center', padding: '50px' }}>
              <Spinner size="lg" />
            </div>
          )}

          {error && (
            <Alert variant={AlertVariant.danger} title="Error" isInline>
              {error}
            </Alert>
          )}

          {!isLoading && !error && (
            <VisualizationSurface state={{}} />
          )}
        </TopologyView>
      </DrawerContent>
    </Drawer>
  );
};

export const TopologyViewer: React.FC = () => {
  return (
    <VisualizationProvider>
      <TopologyViewerInner />
    </VisualizationProvider>
  );
};
