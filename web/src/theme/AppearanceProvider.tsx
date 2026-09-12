import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { applyAppearance, readAppearance } from "./apply";
import type { Appearance, DensityId, RadiusId, ThemeId } from "./config";

type AppearanceContextValue = {
  appearance: Appearance;
  setTheme: (theme: ThemeId) => void;
  setRadius: (radius: RadiusId) => void;
  setDensity: (density: DensityId) => void;
};

const AppearanceContext = createContext<AppearanceContextValue | null>(null);

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [appearance, setAppearance] = useState<Appearance>(readAppearance);

  useEffect(() => {
    applyAppearance(appearance);
  }, [appearance]);

  const value = useMemo<AppearanceContextValue>(
    () => ({
      appearance,
      setTheme: (theme) => setAppearance((current) => ({ ...current, theme })),
      setRadius: (radius) => setAppearance((current) => ({ ...current, radius })),
      setDensity: (density) => setAppearance((current) => ({ ...current, density })),
    }),
    [appearance],
  );

  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance(): AppearanceContextValue {
  const value = useContext(AppearanceContext);
  if (!value) {
    throw new Error("useAppearance must be used within AppearanceProvider");
  }
  return value;
}
