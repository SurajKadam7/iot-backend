import { APP_NAME, LOGO_SRC } from "../brand";

export function Brand({
  subtitle,
  size = "default",
}: {
  subtitle?: string;
  size?: "default" | "large";
}) {
  return (
    <div className={`brand${size === "large" ? " login-brand" : ""}`}>
      <img
        className={`brand-logo${size === "large" ? " large" : ""}`}
        src={LOGO_SRC}
        alt={APP_NAME || "Logo"}
      />
      {subtitle ? (
        <div className="brand-org" title={subtitle}>
          {subtitle}
        </div>
      ) : null}
    </div>
  );
}
