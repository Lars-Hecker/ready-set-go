import { createClient } from "@connectrpc/connect";
import { UserService } from "../gen/user/user_pb";
import { getTransport } from "../transport";

function getUserClient() {
  return createClient(UserService, getTransport());
}

export const userApi = {
  getProfile: (userId: string) =>
    getUserClient().getProfile({ userId }),

  getMe: () =>
    getUserClient().getMe({}),
};
