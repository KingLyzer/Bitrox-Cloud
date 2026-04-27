"use client";

import type { NodeRecord } from "@/lib/api";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  rootLabel: string;
  actionBusy: boolean;
  showCreateModal: boolean;
  createFolderName: string;
  onChangeCreateFolderName: (value: string) => void;
  onCloseCreateModal: () => void;
  onSubmitCreateFolder: () => void;
  showCreateTextModal: boolean;
  createTextFileName: string;
  onChangeCreateTextFileName: (value: string) => void;
  onCloseCreateTextModal: () => void;
  onSubmitCreateTextFile: () => void;
  textEditorTarget: NodeRecord | null;
  textEditorContent: string;
  textEditorLoading: boolean;
  textEditorSaving: boolean;
  onChangeTextEditorContent: (value: string) => void;
  onCloseTextEditor: () => void;
  onSubmitSaveTextFile: () => void;
  renameTarget: NodeRecord | null;
  renameName: string;
  onChangeRenameName: (value: string) => void;
  onCloseRenameModal: () => void;
  onSubmitRename: () => void;
  deleteTargets: NodeRecord[];
  onCloseDeleteModal: () => void;
  onSubmitDelete: () => void;
  showMoveModal: boolean;
  moveTargets: NodeRecord[];
  moveDestinationID: string | null;
  allFoldersForMove: NodeRecord[];
  moveValidationMessage: string | null;
  onChangeMoveDestinationID: (value: string | null) => void;
  onCloseMoveModal: () => void;
  onSubmitMove: () => void;
};

export function FileActionModals({
  rootLabel,
  actionBusy,
  showCreateModal,
  createFolderName,
  onChangeCreateFolderName,
  onCloseCreateModal,
  onSubmitCreateFolder,
  showCreateTextModal,
  createTextFileName,
  onChangeCreateTextFileName,
  onCloseCreateTextModal,
  onSubmitCreateTextFile,
  textEditorTarget,
  textEditorContent,
  textEditorLoading,
  textEditorSaving,
  onChangeTextEditorContent,
  onCloseTextEditor,
  onSubmitSaveTextFile,
  renameTarget,
  renameName,
  onChangeRenameName,
  onCloseRenameModal,
  onSubmitRename,
  deleteTargets,
  onCloseDeleteModal,
  onSubmitDelete,
  showMoveModal,
  moveTargets,
  moveDestinationID,
  allFoldersForMove,
  moveValidationMessage,
  onChangeMoveDestinationID,
  onCloseMoveModal,
  onSubmitMove
}: Props) {
  const { t } = useAppContext();

  return (
    <>
      {showCreateModal ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4" data-testid="create-folder-modal">
          <div className="surface-card w-full max-w-md rounded-2xl p-5">
            <h2 className="text-lg font-semibold">{t("modal.newFolder.title", "New Folder")}</h2>
            <input
              value={createFolderName}
              onChange={(event) => onChangeCreateFolderName(event.target.value)}
              placeholder={t("modal.newFolder.placeholder", "Folder name")}
              data-testid="create-folder-input"
              className="focus-ring mt-3 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 outline-none"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={onCloseCreateModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm">
                {t("common.cancel", "Cancel")}
              </button>
              <button
                type="button"
                onClick={onSubmitCreateFolder}
                disabled={actionBusy}
                data-testid="create-folder-submit"
                className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
              >
                {t("common.create", "Create")}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {showCreateTextModal ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4">
          <div className="surface-card w-full max-w-md rounded-2xl p-5">
            <h2 className="text-lg font-semibold">{t("modal.newTextFile.title", "New text file")}</h2>
            <input
              value={createTextFileName}
              onChange={(event) => onChangeCreateTextFileName(event.target.value)}
              placeholder={t("modal.newTextFile.placeholder", "File name (e.g. notes.txt)")}
              className="focus-ring mt-3 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 outline-none"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={onCloseCreateTextModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm">
                {t("common.cancel", "Cancel")}
              </button>
              <button
                type="button"
                onClick={onSubmitCreateTextFile}
                disabled={actionBusy}
                className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
              >
                {t("common.create", "Create")}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {textEditorTarget ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4">
          <div className="surface-card flex h-[80vh] w-full max-w-4xl flex-col rounded-2xl p-5">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold">{t("modal.textEditor.title", "Text Editor")}</h2>
                <p className="text-sm text-[var(--text-muted)]">{textEditorTarget.name}</p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={onCloseTextEditor}
                  disabled={textEditorSaving}
                  className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)] disabled:opacity-50"
                >
                  {t("common.close", "Close")}
                </button>
                <button
                  type="button"
                  onClick={onSubmitSaveTextFile}
                  disabled={textEditorLoading || textEditorSaving}
                  className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
                >
                  {textEditorSaving ? t("common.saving", "Saving...") : t("common.save", "Save")}
                </button>
              </div>
            </div>
            <div className="mt-4 min-h-0 flex-1">
              {textEditorLoading ? (
                <div className="grid h-full place-items-center text-sm text-[var(--text-muted)]">{t("modal.textEditor.loading", "Loading file content...")}</div>
              ) : (
                <textarea
                  value={textEditorContent}
                  onChange={(event) => onChangeTextEditorContent(event.target.value)}
                  className="focus-ring custom-scrollbar h-full w-full resize-none rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-3 text-sm leading-6 outline-none"
                  spellCheck={false}
                />
              )}
            </div>
          </div>
        </div>
      ) : null}

      {renameTarget ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4" data-testid="rename-modal">
          <div className="surface-card w-full max-w-md rounded-2xl p-5">
            <h2 className="text-lg font-semibold">{t("modal.rename.title", "Rename")}</h2>
            <p className="mt-1 text-sm text-[var(--text-muted)]">{t("modal.rename.target", "Target: {name}", { name: renameTarget.name })}</p>
            <input
              value={renameName}
              onChange={(event) => onChangeRenameName(event.target.value)}
              data-testid="rename-input"
              className="focus-ring mt-3 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 outline-none"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={onCloseRenameModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm">
                {t("common.cancel", "Cancel")}
              </button>
              <button
                type="button"
                onClick={onSubmitRename}
                disabled={actionBusy}
                data-testid="rename-submit"
                className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
              >
                {t("common.save", "Save")}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {deleteTargets.length > 0 ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4" data-testid="delete-modal">
          <div className="surface-card w-full max-w-md rounded-2xl p-5">
            <h2 className="text-lg font-semibold">{t("modal.delete.title", "Delete confirmation")}</h2>
            <p className="mt-1 text-sm text-[var(--text-muted)]">{t("modal.delete.count", "{count} item(s) will be permanently deleted.", { count: deleteTargets.length })}</p>
            <div className="mt-3 max-h-32 overflow-y-auto rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-2 text-xs text-[var(--text-muted)]">
              {deleteTargets.map((node) => (
                <p key={node.id}>{node.name}</p>
              ))}
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={onCloseDeleteModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm">
                {t("common.cancel", "Cancel")}
              </button>
              <button
                type="button"
                onClick={onSubmitDelete}
                disabled={actionBusy}
                data-testid="delete-submit"
                className="focus-ring rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm font-semibold text-red-500 disabled:opacity-60"
              >
                {t("common.delete", "Delete")}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {showMoveModal ? (
        <div className="modal-overlay fixed inset-0 z-40 grid place-items-center p-4" data-testid="move-modal">
          <div className="surface-card w-full max-w-md rounded-2xl p-5">
            <h2 className="text-lg font-semibold">{t("modal.move.title", "Move")}</h2>
            <p className="mt-1 text-sm text-[var(--text-muted)]">{t("modal.move.count", "{count} item(s) will be moved.", { count: moveTargets.length })}</p>
            <label className="mt-3 block text-sm text-[var(--text-main)]">
              {t("modal.move.destination", "Destination folder")}
              <select
                value={moveDestinationID ?? ""}
                onChange={(event) => {
                  const value = event.target.value;
                  onChangeMoveDestinationID(value === "" ? null : value);
                }}
                data-testid="move-destination-select"
                className="focus-ring mt-1 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 outline-none"
              >
                <option value="">{rootLabel}</option>
                {allFoldersForMove.map((folder) => (
                  <option key={folder.id} value={folder.id}>
                    {folder.name}
                  </option>
                ))}
              </select>
            </label>
            {moveValidationMessage ? <p className="mt-2 text-xs text-red-500">{moveValidationMessage}</p> : null}
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={onCloseMoveModal} className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm">
                {t("common.cancel", "Cancel")}
              </button>
              <button
                type="button"
                onClick={onSubmitMove}
                disabled={actionBusy || Boolean(moveValidationMessage)}
                data-testid="move-submit"
                className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
              >
                {t("common.move", "Move")}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
