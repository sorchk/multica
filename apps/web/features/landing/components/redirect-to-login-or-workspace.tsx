"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { workspaceListOptions } from "@multica/core/workspace";
import { resolvePostAuthDestination, useHasOnboarded } from "@multica/core/paths";
import { paths } from "@multica/core/paths";

export function RedirectToLoginOrWorkspace() {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const hasOnboarded = useHasOnboarded();

  const { data: list = [], isFetched } = useQuery({
    ...workspaceListOptions(),
    enabled: !!user,
  });

  useEffect(() => {
    if (isLoading) return;

    if (!user) {
      router.replace(paths.login());
      return;
    }

    if (!isFetched) return;
    router.replace(resolvePostAuthDestination(list, hasOnboarded));
  }, [isLoading, user, isFetched, list, hasOnboarded, router]);

  return null;
}