import { densities, radii, themeById, themes } from "../theme";
import { useAppearance } from "../theme/AppearanceProvider";
import { MoonIcon, SunIcon } from "./icons";

export function AppearanceController({
  align = "start",
  placement = "top",
}: {
  align?: "start" | "end";
  placement?: "top" | "bottom";
}) {
  const { appearance, setTheme, setRadius, setDensity } = useAppearance();
  const current = themeById(appearance.theme);

  return (
    <div className={`dropdown ${placement === "top" ? "dropdown-top" : "dropdown-bottom"} ${align === "end" ? "dropdown-end" : "dropdown-start"} w-full`}>
      <div tabIndex={0} role="button" className="btn btn-ghost btn-block justify-between">
        Appearance
        <span className="badge badge-soft">{current.label}</span>
      </div>
      <div tabIndex={0} className="dropdown-content bg-base-100 rounded-box z-50 mb-2 w-64 p-3 shadow">
        <fieldset className="fieldset">
          <legend className="fieldset-legend">Theme</legend>
          <div className="flex flex-col gap-1">
            {themes.map((theme) => (
              <input
                key={theme.id}
                type="radio"
                name="fluxera-theme"
                className="theme-controller btn btn-sm btn-block btn-ghost justify-start"
                aria-label={theme.label}
                value={theme.id}
                checked={appearance.theme === theme.id}
                onChange={() => setTheme(theme.id)}
              />
            ))}
          </div>
        </fieldset>
        <fieldset className="fieldset">
          <legend className="fieldset-legend">Corners</legend>
          <div className="join w-full">
            {radii.map((radius) => (
              <input
                key={radius.id}
                type="radio"
                name="fluxera-radius"
                className="join-item btn btn-sm flex-1"
                aria-label={radius.label}
                checked={appearance.radius === radius.id}
                onChange={() => setRadius(radius.id)}
              />
            ))}
          </div>
        </fieldset>
        <fieldset className="fieldset">
          <legend className="fieldset-legend">Density</legend>
          <div className="join w-full">
            {densities.map((density) => (
              <input
                key={density.id}
                type="radio"
                name="fluxera-density"
                className="join-item btn btn-sm flex-1"
                aria-label={density.label}
                checked={appearance.density === density.id}
                onChange={() => setDensity(density.id)}
              />
            ))}
          </div>
        </fieldset>
      </div>
    </div>
  );
}

export function ThemeSwapButton() {
  const { appearance, setTheme } = useAppearance();
  const light = appearance.theme === "fluxera-light";
  return (
    <label className="swap swap-rotate" title={light ? "Switch to dark theme" : "Switch to light theme"}>
      <input
        type="checkbox"
        checked={light}
        onChange={() => setTheme(light ? "fluxera" : "fluxera-light")}
        aria-label="Toggle color theme"
      />
      <SunIcon className="swap-on size-5" />
      <MoonIcon className="swap-off size-5" />
    </label>
  );
}
