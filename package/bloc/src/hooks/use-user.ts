import { useQuery } from "@tanstack/react-query";
import { userApi } from "../api/user";

export const userKeys = {
  all: ["user"] as const,
  me: () => [...userKeys.all, "me"] as const,
  profile: (userId: string) => [...userKeys.all, "profile", userId] as const,
};

export function useMe() {
  return useQuery({
    queryKey: userKeys.me(),
    queryFn: () => userApi.getMe(),
  });
}

export function useProfile(userId: string) {
  return useQuery({
    queryKey: userKeys.profile(userId),
    queryFn: () => userApi.getProfile(userId),
    enabled: !!userId,
  });
}
