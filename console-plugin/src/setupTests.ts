import '@testing-library/jest-dom';

// Mock @openshift-console/dynamic-plugin-sdk
jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  consoleFetch: jest.fn(),
  useK8sWatchResource: jest.fn(() => [[], true, undefined]),
  k8sCreate: jest.fn(),
  k8sUpdate: jest.fn(),
  k8sDelete: jest.fn(),
  k8sGet: jest.fn(),
  k8sList: jest.fn(),
}));
