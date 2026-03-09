import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { authApi } from "../api/auth";

export const authKeys = {
  all: ["auth"] as const,
  sessions: () => [...authKeys.all, "sessions"] as const,
};

export function useActiveSessions() {
  return useQuery({
    queryKey: authKeys.sessions(),
    queryFn: () => authApi.getActiveSessions(),
  });
}

export function useSignup() {
  return useMutation({
    mutationFn: ({ email, name }: { email: string; name: string }) =>
      authApi.signup(email, name),
  });
}

export function useLogin() {
  return useMutation({
    mutationFn: ({ email }: { email: string }) =>
      authApi.login(email),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => authApi.logout(),
    onSuccess: () => {
      queryClient.clear();
    },
  });
}

export function useRevokeSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      authApi.revokeSession(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.sessions() });
    },
  });
}

export function useRevokeAllSessions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ keepCurrent }: { keepCurrent: boolean }) =>
      authApi.revokeAllSessions(keepCurrent),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.sessions() });
    },
  });
}
