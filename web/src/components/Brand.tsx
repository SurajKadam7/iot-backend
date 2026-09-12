import { APP_NAME, LOGO_SRC } from "../brand";

export function Brand({
  subtitle,
  size = "default",
  compact = false,
}: {
  subtitle?: string;
  size?: "default" | "large";
  compact?: boolean;
}) {
  const dim = compact ? "size-10 p-1" : size === "large" ? "size-36 p-4" : "size-20 p-2.5";
  return (
    <div className="flex min-w-0 flex-col items-center gap-2.5">
      <img
        className={`${dim} block shrink-0 rounded-full bg-white object-contain`}
        src={LOGO_SRC}
        alt={APP_NAME || "Fluxera"}
      />
      {subtitle ? (
        <div className="w-full truncate text-center text-sm text-base-content/60" title={subtitle}>
          {subtitle}
        </div>
      ) : null}
    </div>
  );
}
