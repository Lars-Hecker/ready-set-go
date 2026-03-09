import { useMutation, useQuery } from "@tanstack/react-query";
import { fileApi } from "../api/file";

export const fileKeys = {
  all: ["file"] as const,
  downloadUrl: (fileId: string) => [...fileKeys.all, "downloadUrl", fileId] as const,
};

export function useDownloadURL(fileId: string) {
  return useQuery({
    queryKey: fileKeys.downloadUrl(fileId),
    queryFn: () => fileApi.getDownloadURL(fileId),
    enabled: !!fileId,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

export function useRequestUpload() {
  return useMutation({
    mutationFn: ({ workspaceId, filename, contentType, size }: {
      workspaceId: string;
      filename: string;
      contentType: string;
      size: bigint;
    }) =>
      fileApi.requestUpload(workspaceId, filename, contentType, size),
  });
}

export function useConfirmUpload() {
  return useMutation({
    mutationFn: ({ fileId }: { fileId: string }) =>
      fileApi.confirmUpload(fileId),
  });
}
