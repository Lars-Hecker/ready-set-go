import { createConnectTransport } from "@connectrpc/connect-web";

let transport: ReturnType<typeof createConnectTransport> | null = null;

export function getTransport() {
  if (!transport) {
    transport = createConnectTransport({
      baseUrl: "/api",
    });
  }
  return transport;
}
