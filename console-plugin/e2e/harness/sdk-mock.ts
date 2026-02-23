/**
 * Mock for @openshift-console/dynamic-plugin-sdk
 *
 * NormalModuleReplacementPlugin swaps the real SDK for this file.
 * - consoleFetch delegates to window.fetch so Playwright page.route() can intercept
 * - useK8sWatchResource reads from window.__K8S_WATCH_DATA__
 * - k8s CRUD functions are no-op stubs
 */

import { useState } from 'react';

/* ---------- global store for K8s watch data ---------- */

declare global {
  interface Window {
    __K8S_WATCH_DATA__: Record<string, unknown[]>;
  }
}

/* ---------- consoleFetch ---------- */

export async function consoleFetch(
  url: string,
  options?: RequestInit,
): Promise<Response> {
  return window.fetch(url, options);
}

/* ---------- useK8sWatchResource ---------- */

interface WatchResourceParams {
  groupVersionKind?: {
    group: string;
    version: string;
    kind: string;
  };
  namespace?: string;
  isList?: boolean;
  name?: string;
}

export function useK8sWatchResource<T = unknown>(
  params: WatchResourceParams,
): [T, boolean, unknown] {
  const key = params.groupVersionKind
    ? `${params.groupVersionKind.group}/${params.groupVersionKind.version}/${params.groupVersionKind.kind}`
    : 'unknown';

  const store = window.__K8S_WATCH_DATA__ || {};
  const data = (store[key] ?? (params.isList ? [] : null)) as T;

  // Always return loaded=true so pages render immediately
  return [data, true, undefined];
}

/* ---------- K8s CRUD stubs ---------- */

export async function k8sCreate(options: { model: unknown; data: unknown }) {
  return options.data;
}

export async function k8sUpdate(options: {
  model: unknown;
  data: unknown;
  ns?: string;
}) {
  return options.data;
}

export async function k8sDelete(_options: {
  model: unknown;
  resource: unknown;
}) {
  return {};
}

export async function k8sGet(options: {
  model: unknown;
  ns?: string;
  name?: string;
}) {
  return { metadata: { name: options.name, namespace: options.ns } };
}

export async function k8sList(options: {
  model: unknown;
  queryParams?: unknown;
}) {
  return { items: [] };
}

/* ---------- re-export anything else pages may reference ---------- */

// useResolvedExtensions — not used by our pages but exported just in case
export function useResolvedExtensions() {
  return [[], true, undefined];
}
