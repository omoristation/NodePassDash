import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Card, CardBody, CardHeader, Chip, Spinner } from "@heroui/react";
import { Icon } from "@iconify/react/dist/offline";
import { useTranslation } from "react-i18next";

import { buildApiUrl } from "@/lib/utils";

interface UpdateCheckResult {
  current: string;
  latest?: string;
  hasUpdate: boolean;
  name?: string;
  notes?: string;
  publishedAt?: string;
  releaseUrl?: string;
  asset?: string;
  supported: boolean;
  message?: string;
}

interface UpdateProgress {
  running: boolean;
  phase: string;
  message?: string;
  current: string;
  latest?: string;
  previous?: string;
}

interface EndpointUpdateCardProps {
  endpointId: number | null;
  currentVersion?: string;
}

/**
 * 主控（NodePass master）版本更新卡片。
 *
 * 检查与更新都通过面板后端转发到主控的 /update/* 接口，浏览器不接触主控
 * 的 API Key。更新替换二进制后主控会自行重启（phase 经过 "restarting"
 * 再到 "done"），重启会同时重启该主控管理的全部实例，这里持续轮询直到
 * 主控以新版本恢复。
 */
export function EndpointUpdateCard({
  endpointId,
  currentVersion,
}: EndpointUpdateCardProps) {
  const { t } = useTranslation("endpoints");

  const [checking, setChecking] = useState(false);
  const [applying, setApplying] = useState(false);
  const [result, setResult] = useState<UpdateCheckResult | null>(null);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => stopPolling, [stopPolling]);

  const handleCheck = useCallback(async () => {
    if (!endpointId) return;
    setChecking(true);
    setError("");
    try {
      const res = await fetch(
        buildApiUrl(`/api/endpoints/${endpointId}/update/check`),
        { cache: "no-store" },
      );
      const json = await res.json().catch(() => ({}));

      if (!res.ok || !json.success) {
        setError(json.error || `HTTP ${res.status}`);
        return;
      }
      setResult(json.data);
      setProgress(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setChecking(false);
    }
  }, [endpointId]);

  const startPolling = useCallback(() => {
    if (!endpointId) return;
    stopPolling();

    const deadline = Date.now() + 10 * 60 * 1000;

    pollRef.current = setInterval(async () => {
      try {
        const res = await fetch(
          buildApiUrl(`/api/endpoints/${endpointId}/update/status`),
          { cache: "no-store" },
        );
        const json = await res.json().catch(() => ({}));

        if (res.ok && json.success && json.data) {
          const status = json.data as UpdateProgress;
          setProgress(status);
          // "restarting" 期间主控正在退出（请求可能短暂失败），继续轮询，
          // 直到它以新版本回来并报告 "done"。
          if (!status.running && status.phase !== "restarting") {
            stopPolling();
            setApplying(false);
            if (status.phase === "error") {
              setError(status.message || t("details.update.failed"));
            } else if (status.phase === "done") {
              // 更新已完成并重启：当前版本换成主控报告的新版本，清除待更新
              // 状态，避免卡片继续显示“发现新版本”。
              setResult((prev) =>
                prev
                  ? {
                      ...prev,
                      current: status.current || prev.current,
                      latest: status.latest || prev.latest,
                      hasUpdate: false,
                      message: undefined,
                    }
                  : prev,
              );
            }
          }
        }
      } catch {
        // 网络抖动时保持轮询
      }

      if (Date.now() > deadline) {
        stopPolling();
        setApplying(false);
        setError(t("details.update.timeout"));
      }
    }, 2000);
  }, [endpointId, stopPolling, t]);

  const handleApply = useCallback(async () => {
    if (!endpointId) return;
    setApplying(true);
    setError("");
    setProgress(null);
    try {
      const res = await fetch(
        buildApiUrl(`/api/endpoints/${endpointId}/update/apply`),
        { method: "POST" },
      );
      const json = await res.json().catch(() => ({}));

      if (!res.ok || !json.success) {
        setError(json.error || `HTTP ${res.status}`);
        setApplying(false);
        return;
      }
      startPolling();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setApplying(false);
    }
  }, [endpointId, startPolling]);

  // 进入页面时读取一次主控的更新状态：如果此前已经触发过更新（或刚完成
  // 尚未重启），这里接着展示，不必重新点检查。
  useEffect(() => {
    if (!endpointId) return;
    let cancelled = false;

    (async () => {
      try {
        const res = await fetch(
          buildApiUrl(`/api/endpoints/${endpointId}/update/status`),
          { cache: "no-store" },
        );
        const json = await res.json().catch(() => ({}));

        if (cancelled || !res.ok || !json.success || !json.data) return;
        const status = json.data as UpdateProgress;
        if (status.running || status.phase === "restarting") {
          setProgress(status);
          setApplying(true);
          startPolling();
        } else if (status.phase === "done" || status.phase === "error") {
          setProgress(status);
          if (status.phase === "error") {
            setError(status.message || t("details.update.failed"));
          }
        }
      } catch {
        // 主控不可达时保持静默，用户仍可手动点检查
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [endpointId, startPolling, t]);

  const running = applying || progress?.running === true;

  const phaseText = (() => {
    if (!progress) return "";
    switch (progress.phase) {
      case "checking":
        return t("details.update.checking");
      case "downloading":
        return t("details.update.downloading");
      case "installing":
        return t("details.update.installing");
      case "restarting":
        return t("details.update.restarting");
      case "done":
        return t("details.update.done");
      default:
        return progress.message || progress.phase;
    }
  })();

  return (
    <Card className="p-2">
      <CardHeader className="flex items-center justify-between gap-3">
        <div className="flex flex-col flex-1">
          <h3 className="text-lg font-semibold">{t("details.update.title")}</h3>
          <p className="text-sm text-default-500">
            {t("details.update.description")}
          </p>
        </div>
        <Button
          isLoading={checking}
          isDisabled={!endpointId || running}
          size="sm"
          startContent={
            !checking ? <Icon icon="solar:refresh-bold" width={18} /> : null
          }
          variant="bordered"
          onPress={handleCheck}
        >
          {checking ? t("details.update.checking") : t("details.update.check")}
        </Button>
      </CardHeader>
      <CardBody className="space-y-3">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm text-default-500">
            {t("details.update.current")}:
          </span>
          <Chip color="secondary" size="sm" variant="flat">
            {progress?.current || result?.current || currentVersion || "—"}
          </Chip>
          {result?.latest && (
            <Chip
              color={result.hasUpdate ? "warning" : "success"}
              size="sm"
              variant="flat"
            >
              {t("details.update.latest")}: {result.latest}
            </Chip>
          )}
        </div>

        {result && !result.hasUpdate && !result.message && (
          <p className="flex items-center gap-1 text-sm text-success">
            <Icon icon="solar:check-circle-bold" width={16} />
            {t("details.update.upToDate")}
          </p>
        )}

        {result?.message && (
          <p className="text-sm text-warning">{result.message}</p>
        )}

        {result?.hasUpdate && (
          <div className="space-y-2">
            <p className="text-sm font-medium text-warning">
              {t("details.update.available", { version: result.latest })}
            </p>
            {!result.supported && (
              <p className="text-xs text-danger">
                {t("details.update.unsupported")}
              </p>
            )}
            <div className="flex items-center gap-3 flex-wrap">
              <Button
                color="primary"
                isDisabled={!result.supported || running}
                isLoading={running}
                size="sm"
                startContent={
                  !running ? <Icon icon="solar:download-bold" width={18} /> : null
                }
                onPress={handleApply}
              >
                {running
                  ? t("details.update.updating")
                  : t("details.update.update")}
              </Button>
              {result.releaseUrl && (
                <a
                  className="flex items-center gap-1 text-xs text-default-500 hover:text-default-700"
                  href={result.releaseUrl}
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {t("details.update.releaseNotes")}
                  <Icon icon="lucide:external-link" width={12} />
                </a>
              )}
            </div>
          </div>
        )}

        {progress && (
          <div className="rounded-lg border border-default-200 p-3 space-y-1">
            <div className="flex items-center gap-2 text-sm">
              {running && <Spinner size="sm" />}
              <span>{phaseText}</span>
            </div>
          </div>
        )}

        {error && <p className="text-sm text-danger">{error}</p>}
      </CardBody>
    </Card>
  );
}
