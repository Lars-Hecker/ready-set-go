import { createClient } from "@connectrpc/connect";
import { WorkspaceService } from "../gen/workspace/workspace_pb";
import type { Role } from "../gen/workspace/workspace_pb";
import { getTransport } from "../transport";

function getWorkspaceClient() {
  return createClient(WorkspaceService, getTransport());
}

export const workspaceApi = {
  create: (name: string, slug: string) =>
    getWorkspaceClient().createWorkspace({ name, slug }),

  get: (workspaceId: string) =>
    getWorkspaceClient().getWorkspace({ workspaceId }),

  update: (workspaceId: string, data: { name?: string; slug?: string }) =>
    getWorkspaceClient().updateWorkspace({ workspaceId, ...data }),

  delete: (workspaceId: string) =>
    getWorkspaceClient().deleteWorkspace({ workspaceId }),

  list: () =>
    getWorkspaceClient().listWorkspaces({}),

  listMembers: (workspaceId: string) =>
    getWorkspaceClient().listMembers({ workspaceId }),

  updateMemberRole: (workspaceId: string, membershipId: string, role: Role) =>
    getWorkspaceClient().updateMemberRole({ workspaceId, membershipId, role }),

  transferOwnership: (workspaceId: string, newOwnerUserId: string) =>
    getWorkspaceClient().transferOwnership({ workspaceId, newOwnerUserId }),

  removeMember: (workspaceId: string, membershipId: string) =>
    getWorkspaceClient().removeMember({ workspaceId, membershipId }),

  leave: (workspaceId: string) =>
    getWorkspaceClient().leaveWorkspace({ workspaceId }),
};
