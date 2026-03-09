import { createClient } from "@connectrpc/connect";
import { FileService } from "../gen/file/file_pb";
import { getTransport } from "../transport";

function getFileClient() {
  return createClient(FileService, getTransport());
}

export const fileApi = {
  requestUpload: (workspaceId: string, filename: string, contentType: string, size: bigint) =>
    getFileClient().requestUpload({ workspaceId, filename, contentType, size }),

  confirmUpload: (fileId: string) =>
    getFileClient().confirmUpload({ fileId }),

  getDownloadURL: (fileId: string) =>
    getFileClient().getDownloadURL({ fileId }),
};
