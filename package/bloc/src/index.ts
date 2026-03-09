// Transport
export { getTransport } from "./transport";

// Error handling
export {
  ConnectError,
  Code,
  isConnectError,
  isUnauthenticated,
  isPermissionDenied,
  isNotFound,
  isInvalidArgument,
  isInternal,
  toAppError,
} from "./errors";
export type { AppError } from "./errors";

// API clients
export { authApi, userApi, workspaceApi, fileApi } from "./api";

// TanStack Query hooks
export * from "./hooks";

// Re-export generated types and services
export * from "./gen/user/auth_pb";
export * from "./gen/user/user_pb";
export * from "./gen/workspace/workspace_pb";
export * from "./gen/file/file_pb";
