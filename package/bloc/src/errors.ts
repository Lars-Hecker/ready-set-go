import { ConnectError, Code } from "@connectrpc/connect";

export { ConnectError, Code };

export function isConnectError(error: unknown): error is ConnectError {
  return error instanceof ConnectError;
}

export function isUnauthenticated(error: unknown): boolean {
  return isConnectError(error) && error.code === Code.Unauthenticated;
}

export function isPermissionDenied(error: unknown): boolean {
  return isConnectError(error) && error.code === Code.PermissionDenied;
}

export function isNotFound(error: unknown): boolean {
  return isConnectError(error) && error.code === Code.NotFound;
}

export function isInvalidArgument(error: unknown): boolean {
  return isConnectError(error) && error.code === Code.InvalidArgument;
}

export function isInternal(error: unknown): boolean {
  return isConnectError(error) && error.code === Code.Internal;
}

export interface AppError {
  code: Code;
  message: string;
}

export function toAppError(error: unknown): AppError {
  if (isConnectError(error)) {
    return {
      code: error.code,
      message: error.message,
    };
  }
  return {
    code: Code.Unknown,
    message: error instanceof Error ? error.message : "An unknown error occurred",
  };
}
