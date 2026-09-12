import {
  APPEARANCE_STORAGE_KEY,
  defaultAppearance,
  isDensityId,
  isRadiusId,
  isThemeId,
  themeById,
  type Appearance,
} from "./config";

export function readAppearance(): Appearance {
  if (typeof window === "undefined") return defaultAppearance;
  try {
    const raw = localStorage.getItem(APPEARANCE_STORAGE_KEY);
    if (!raw) return defaultAppearance;
    if (isThemeId(raw)) return { ...defaultAppearance, theme: raw };
    const parsed = JSON.parse(raw) as Partial<Appearance>;
    return {
      theme: isThemeId(parsed.theme) ? parsed.theme : defaultAppearance.theme,
      radius: isRadiusId(parsed.radius) ? parsed.radius : defaultAppearance.radius,
      density: isDensityId(parsed.density) ? parsed.density : defaultAppearance.density,
    };
  } catch {
    return defaultAppearance;
  }
}

export function applyAppearance(appearance: Appearance): void {
  const root = document.documentElement;
  root.setAttribute("data-theme", appearance.theme);
  root.setAttribute("data-radius", appearance.radius);
  root.setAttribute("data-density", appearance.density);
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", themeById(appearance.theme).themeColor);
  try {
    localStorage.setItem(APPEARANCE_STORAGE_KEY, JSON.stringify(appearance));
  } catch {
    /* ignore quota / private mode */
  }
}
