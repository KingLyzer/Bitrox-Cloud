"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type MutableRefObject } from "react";
import { abortUploadSession, createUploadSession, finalizeUploadSession, uploadChunk } from "@/lib/api";
import { generateClientID, sha256Hex } from "@/lib/browser";
import { useAppContext } from "@/components/app-context-provider";
import { normalizeErrorMessage, type Flash, type UploadJob } from "@/components/file-manager/helpers";

type Options = {
  defaultChunkSizeBytes: number;
  currentParentRef: MutableRefObject<string | null>;
  refreshCurrentFolder: () => Promise<void>;
  runWithRefresh: <T>(operation: () => Promise<T>) => Promise<T>;
  showFlash: (tone: Flash["tone"], message: string) => void;
};

type UploadHandleOptions = {
  targetType?: "new_file" | "new_version";
  targetNodeID?: string;
  successMessage?: string;
};

export function useUploadManager({
  defaultChunkSizeBytes,
  currentParentRef,
  refreshCurrentFolder,
  runWithRefresh,
  showFlash
}: Options) {
  const { t } = useAppContext();
  const [uploadJobs, setUploadJobs] = useState<UploadJob[]>([]);
  const activeSessionByJobRef = useRef<Map<string, string>>(new Map());
  const cancelRequestedJobIDsRef = useRef<Set<string>>(new Set());

  const runningUploadCount = useMemo(() => uploadJobs.filter((job) => job.status === "running").length, [uploadJobs]);
  const hasRunningUploads = runningUploadCount > 0;

  const confirmLeaveIfUploading = useCallback(() => {
    if (!hasRunningUploads) {
      return true;
    }
    return window.confirm(
      t(
        "upload.leaveConfirm",
        "Upload is still running. Leaving this page may cancel it. Do you want to continue?"
      )
    );
  }, [hasRunningUploads, t]);

  useEffect(() => {
    if (!hasRunningUploads) {
      return;
    }
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [hasRunningUploads]);

  const upsertUploadJob = useCallback((jobID: string, patch: Partial<UploadJob>) => {
    setUploadJobs((prev) => {
      const index = prev.findIndex((job) => job.id === jobID);
      if (index < 0) {
        return [
          {
            id: jobID,
            fileName: patch.fileName ?? "upload",
            progress: patch.progress ?? 0,
            status: patch.status ?? "running",
            detail: patch.detail ?? "",
            sourceFile: patch.sourceFile
          },
          ...prev
        ].slice(0, 30);
      }
      const next = [...prev];
      next[index] = { ...next[index], ...patch };
      return next;
    });
  }, []);

  const handleUploadFiles = useCallback(
    async (files: File[], options?: UploadHandleOptions): Promise<boolean> => {
      if (files.length === 0) {
        return true;
      }
      const targetType = options?.targetType ?? "new_file";

      let successCount = 0;
      let cancelledCount = 0;
      let failedCount = 0;

      for (const file of files) {
        if (file.size <= 0) {
          failedCount += 1;
          showFlash("error", t("upload.skipEmptyFile", "Skipped '{name}' because it is empty.", { name: file.name }));
          continue;
        }

        const jobID = generateClientID();
        upsertUploadJob(jobID, {
          fileName: file.name,
          progress: 0,
          status: "running",
          detail: "0%",
          sourceFile: file
        });

        let sessionID: string | null = null;
        let cancelled = false;

        try {
          const chunkSize = Math.min(defaultChunkSizeBytes, file.size);
          const session = await runWithRefresh(() =>
            createUploadSession({
              fileName: file.name,
              expectedSizeBytes: file.size,
              chunkSizeBytes: chunkSize,
              parentID: targetType === "new_version" ? null : currentParentRef.current,
              idempotencyKey: `${targetType}-${jobID}`,
              targetType,
              targetNodeID: targetType === "new_version" ? options?.targetNodeID ?? null : null
            })
          );
          sessionID = session.id;
          activeSessionByJobRef.current.set(jobID, session.id);

          for (let index = 0; index < session.expected_chunks; index += 1) {
            if (cancelRequestedJobIDsRef.current.has(jobID)) {
              cancelled = true;
              throw new Error("__UPLOAD_CANCELLED__");
            }
            const start = index * session.chunk_size_bytes;
            const end = Math.min(start + session.chunk_size_bytes, file.size);
            const chunk = file.slice(start, end);
            const hash = await sha256Hex(chunk);

            await runWithRefresh(() =>
              uploadChunk({
                uploadSessionID: session.id,
                chunkIndex: index,
                chunkBody: chunk,
                chunkHash: hash
              })
            );

            const progress = Math.round(((index + 1) / session.expected_chunks) * 100);
            upsertUploadJob(jobID, {
              progress,
              detail: `${progress}%`
            });
          }

          if (cancelRequestedJobIDsRef.current.has(jobID)) {
            cancelled = true;
            throw new Error("__UPLOAD_CANCELLED__");
          }

          await runWithRefresh(() => finalizeUploadSession(session.id));
          upsertUploadJob(jobID, {
            progress: 100,
            status: "done",
            detail: "100%",
            sourceFile: undefined
          });
          successCount += 1;
        } catch (error) {
          const cancellationRequested = cancelled || cancelRequestedJobIDsRef.current.has(jobID);
          if (sessionID) {
            try {
              await runWithRefresh(() => abortUploadSession(sessionID as string));
            } catch {
              // best effort
            }
          }
          if (cancellationRequested) {
            cancelledCount += 1;
            upsertUploadJob(jobID, {
              status: "cancelled",
              detail: t("upload.cancelledByUser", "Cancelled by user.")
            });
          } else {
            failedCount += 1;
            upsertUploadJob(jobID, {
              status: "error",
              detail: normalizeErrorMessage(error)
            });
          }
        } finally {
          activeSessionByJobRef.current.delete(jobID);
          cancelRequestedJobIDsRef.current.delete(jobID);
        }
      }

      await refreshCurrentFolder();

      if (failedCount === 0 && cancelledCount === 0) {
        showFlash("success", options?.successMessage ?? t("upload.allCompleted", "All files uploaded."));
        return true;
      }

      const parts: string[] = [];
      if (successCount > 0) {
        parts.push(t("upload.summary.success", "{count} succeeded", { count: successCount }));
      }
      if (cancelledCount > 0) {
        parts.push(t("upload.summary.cancelled", "{count} cancelled", { count: cancelledCount }));
      }
      if (failedCount > 0) {
        parts.push(t("upload.summary.failed", "{count} failed", { count: failedCount }));
      }
      showFlash("info", t("upload.summary.result", "Upload finished: {summary}.", { summary: parts.join(", ") }));
      return false;
    },
    [currentParentRef, defaultChunkSizeBytes, refreshCurrentFolder, runWithRefresh, showFlash, t, upsertUploadJob]
  );

  const retryUploadJob = useCallback(
    (job: UploadJob) => {
      if (!job.sourceFile) {
        showFlash("error", t("upload.retryMissingSource", "Source file is missing for retry. Please pick the file again."));
        return;
      }
      void handleUploadFiles([job.sourceFile]);
    },
    [handleUploadFiles, showFlash, t]
  );

  const cancelUploadJob = useCallback(
    async (job: UploadJob) => {
      if (job.status !== "running") {
        return;
      }
      cancelRequestedJobIDsRef.current.add(job.id);
      upsertUploadJob(job.id, {
        detail: t("upload.cancelling", "Cancelling...")
      });

      const activeSessionID = activeSessionByJobRef.current.get(job.id);
      if (!activeSessionID) {
        return;
      }

      try {
        await runWithRefresh(() => abortUploadSession(activeSessionID));
        upsertUploadJob(job.id, {
          status: "cancelled",
          detail: t("upload.cancelledByUser", "Cancelled by user.")
        });
      } catch (error) {
        upsertUploadJob(job.id, {
          status: "error",
          detail: normalizeErrorMessage(error)
        });
      } finally {
        activeSessionByJobRef.current.delete(job.id);
        cancelRequestedJobIDsRef.current.delete(job.id);
      }
    },
    [runWithRefresh, t, upsertUploadJob]
  );

  return {
    uploadJobs,
    runningUploadCount,
    hasRunningUploads,
    confirmLeaveIfUploading,
    handleUploadFiles,
    retryUploadJob,
    cancelUploadJob
  };
}
