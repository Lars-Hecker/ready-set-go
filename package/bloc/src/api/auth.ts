import { createClient } from "@connectrpc/connect";
import { AuthService } from "../gen/user/auth_pb";
import { getTransport } from "../transport";

function getAuthClient() {
  return createClient(AuthService, getTransport());
}

export const authApi = {
  signup: (email: string, name: string) =>
    getAuthClient().signup({ email, name }),

  login: (email: string) =>
    getAuthClient().login({ email }),

  logout: () =>
    getAuthClient().logout({}),

  getActiveSessions: () =>
    getAuthClient().getActiveSessions({}),

  revokeSession: (sessionId: string) =>
    getAuthClient().revokeSession({ sessionId }),

  revokeAllSessions: (keepCurrent: boolean) =>
    getAuthClient().revokeAllSessions({ keepCurrent }),
};
