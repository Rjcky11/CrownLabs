import type { IQuota } from '../contexts/OwnedInstancesContext';
import { Phase2, type TenantQuery } from '../generated-types';
import { convertToGiB, getOriginalK8sKey, type Instance } from '../utils';

// Internal helper to convert GraphQL camelCase keys into standard K8s format
function normalizeTotalOtherResources(rawResources?: Record<string, any> | null): Record<string, number> {
  if (!rawResources) return {};
  const normalized: Record<string, number> = {};
  Object.keys(rawResources).forEach(key => {
    const normalizedKey = getOriginalK8sKey(key);
    normalized[normalizedKey] = Number(rawResources[key]) || 0;
  });
  return normalized;
}

export function calculateWorkspaceConsumedQuota(
  instances?: Instance[],
): Record<string, IQuota> {
  if (!instances) return {};

  const workspaceUsedResources: Record<string, IQuota> = {};

  // Skip paused instances when calculating consumed quota
  instances
    .filter(instance => instance.status !== Phase2.Off)
    .forEach(instance => {
      if (!workspaceUsedResources[instance.workspaceName]) {
        workspaceUsedResources[instance.workspaceName] = {
          instances: 0,
          cpu: 0,
          memory: 0,
          disk: 0,
          otherResources: {},
        };
      }

      const current = workspaceUsedResources[instance.workspaceName];
      current.instances += 1;
      current.cpu += instance.resources.cpu;
      current.memory += instance.resources.memory;
      current.disk += instance.resources.disk;

      if (instance.resources.otherResources) {
        const extResources = instance.resources.otherResources;
        
        if (!current.otherResources) current.otherResources = {};
        Object.keys(extResources).forEach(key => {
          // Normalize the instance resource key to match the standard K8s format used in total quotas
          const normalizedKey = getOriginalK8sKey(key);
          
          current.otherResources![normalizedKey] =
            (current.otherResources![normalizedKey] || 0) +
            Number(extResources[key]);
        });
      }
    });

  return workspaceUsedResources;
}

export function calculateWorkspaceTotalQuota(
  tenantData: TenantQuery | undefined,
): Record<string, IQuota> {
  if (!tenantData) return {};

  const quotas =
    tenantData?.tenant?.spec?.workspaces?.reduce(
      (map, workspace) => {
        const workspaceName = workspace?.name || '';
        const workspaceQuota =
          workspace?.workspaceWrapperTenantV1alpha2
            ?.itPolitoCrownlabsV1alpha1Workspace?.spec?.quota;

        return {
          ...map,
          [workspaceName]: {
            instances: workspaceQuota?.instances || 0,
            cpu: workspaceQuota?.cpu ? parseFloat(workspaceQuota.cpu) || 0 : 0,
            memory: workspaceQuota?.memory
              ? convertToGiB(workspaceQuota.memory)
              : 0,
            disk: workspaceQuota?.disk ? convertToGiB(workspaceQuota.disk) : 0,
            // Normalize keys coming from GraphQL before saving them to totals
            otherResources: normalizeTotalOtherResources(workspaceQuota?.otherResources),
          },
        };
      },
      {} as Record<string, IQuota>,
    ) || {};

  // Add personal workspace quota (if enabled)
  const personalWorkspaceQuota = tenantData?.tenant?.spec?.personalWorkspace;
  if (personalWorkspaceQuota) {
    quotas['personal'] = {
      instances: personalWorkspaceQuota?.instances || 0,
      cpu: personalWorkspaceQuota?.cpu
        ? parseFloat(personalWorkspaceQuota?.cpu)
        : 0,
      memory: personalWorkspaceQuota?.memory
        ? convertToGiB(personalWorkspaceQuota?.memory)
        : 0,
      disk: personalWorkspaceQuota?.disk ? convertToGiB(personalWorkspaceQuota.disk) : 0,
      // Normalize personal workspace keys as well
      otherResources: normalizeTotalOtherResources(personalWorkspaceQuota?.otherResources),
    };
  }

  return quotas;
}

export function calculateAvailableQuota(
  totalQuota: Record<string, IQuota>,
  consumedQuota: Record<string, IQuota>,
): Record<string, IQuota> {
  const availableQuota: Record<string, IQuota> = {};

  for (const workspace in totalQuota) {
    const totalOther = totalQuota[workspace]?.otherResources || {};
    const consumedOther = consumedQuota[workspace]?.otherResources || {};
    const availableOther: Record<string, number> = {};

    Object.keys(totalOther).forEach(key => {
      availableOther[key] = (totalOther[key] || 0) - (consumedOther[key] || 0);
    });

    availableQuota[workspace] = {
      instances:
        (totalQuota[workspace]?.instances || 0) -
        (consumedQuota[workspace]?.instances || 0),
      cpu:
        (totalQuota[workspace]?.cpu || 0) -
        (consumedQuota[workspace]?.cpu || 0),
      memory:
        (totalQuota[workspace]?.memory || 0) -
        (consumedQuota[workspace]?.memory || 0),
      disk:
        (totalQuota[workspace]?.disk || 0) -
        (consumedQuota[workspace]?.disk || 0),
      otherResources: availableOther,
    };
  }

  return availableQuota;
}