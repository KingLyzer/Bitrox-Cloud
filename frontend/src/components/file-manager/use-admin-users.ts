"use client";

import { useCallback, useState } from "react";
import { createAdminUser, listAdminUsers, updateAdminUser, updateAdminUserPassword, type AdminUser } from "@/lib/api";
import { normalizeErrorMessage, type Flash } from "@/components/file-manager/helpers";

type Options = {
  canAccessAdmin: boolean;
  runWithRefresh: <T>(operation: () => Promise<T>) => Promise<T>;
  showFlash: (tone: Flash["tone"], message: string) => void;
};

export function useAdminUsers({ canAccessAdmin, runWithRefresh, showFlash }: Options) {
  const [adminUsers, setAdminUsers] = useState<AdminUser[]>([]);
  const [adminLoading, setAdminLoading] = useState(false);
  const [adminBusy, setAdminBusy] = useState(false);

  const loadAdminUsers = useCallback(async () => {
    if (!canAccessAdmin) {
      setAdminUsers([]);
      return;
    }
    setAdminLoading(true);
    try {
      const users = await runWithRefresh(() => listAdminUsers());
      setAdminUsers(users);
    } catch (error) {
      showFlash("error", normalizeErrorMessage(error));
    } finally {
      setAdminLoading(false);
    }
  }, [canAccessAdmin, runWithRefresh, showFlash]);

  const createUserFromAdmin = useCallback(
    async (payload: {
      email: string;
      displayName: string;
      role: "owner" | "admin" | "user";
      password: string;
      quotaBytes: number | null;
      isActive: boolean;
    }) => {
      setAdminBusy(true);
      try {
        await runWithRefresh(() => createAdminUser(payload));
        await loadAdminUsers();
        showFlash("success", "Kullanici olusturuldu.");
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setAdminBusy(false);
      }
    },
    [loadAdminUsers, runWithRefresh, showFlash]
  );

  const updateUserFromAdmin = useCallback(
    async (payload: {
      userID: string;
      displayName: string;
      role: "owner" | "admin" | "user";
      isActive: boolean;
      quotaBytes?: number;
      useDefaultQuota?: boolean;
    }) => {
      setAdminBusy(true);
      try {
        await runWithRefresh(() => updateAdminUser(payload));
        await loadAdminUsers();
        showFlash("success", "Kullanici guncellendi.");
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setAdminBusy(false);
      }
    },
    [loadAdminUsers, runWithRefresh, showFlash]
  );

  const updateUserPasswordFromAdmin = useCallback(
    async (userID: string, newPassword: string) => {
      setAdminBusy(true);
      try {
        await runWithRefresh(() => updateAdminUserPassword(userID, newPassword));
        await loadAdminUsers();
        showFlash("success", "Sifre guncellendi.");
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setAdminBusy(false);
      }
    },
    [loadAdminUsers, runWithRefresh, showFlash]
  );

  return {
    adminUsers,
    adminLoading,
    adminBusy,
    loadAdminUsers,
    createUserFromAdmin,
    updateUserFromAdmin,
    updateUserPasswordFromAdmin
  };
}

