/** Central appearance registry. Theme CSS lives in `src/styles.css` and must use these ids. */

export const APPEARANCE_STORAGE_KEY = "fluxera-appearance";

export const themes = [
  { id: "fluxera", label: "Dark", themeColor: "#0b1220", default: true },
  { id: "fluxera-light", label: "Light", themeColor: "#f7fafc" },
] as const;

export const radii = [
  { id: "default", label: "Default" },
  { id: "round", label: "Round" },
  { id: "sharp", label: "Sharp" },
] as const;

export const densities = [
  { id: "compact", label: "Compact" },
  { id: "default", label: "Default" },
  { id: "comfortable", label: "Comfortable" },
] as const;

export type ThemeId = (typeof themes)[number]["id"];
export type RadiusId = (typeof radii)[number]["id"];
export type DensityId = (typeof densities)[number]["id"];

export type Appearance = {
  theme: ThemeId;
  radius: RadiusId;
  density: DensityId;
};

export const defaultAppearance: Appearance = {
  theme: "fluxera",
  radius: "default",
  density: "default",
};

export function isThemeId(value: unknown): value is ThemeId {
  return themes.some((theme) => theme.id === value);
}

export function isRadiusId(value: unknown): value is RadiusId {
  return radii.some((radius) => radius.id === value);
}

export function isDensityId(value: unknown): value is DensityId {
  return densities.some((density) => density.id === value);
}

export function themeById(id: ThemeId) {
  return themes.find((theme) => theme.id === id) ?? themes[0];
}
