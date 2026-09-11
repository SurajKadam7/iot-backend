import { NavLink, Outlet } from "react-router-dom";
import type { ReactNode } from "react";
import type { Me } from "../types";
import { Brand } from "./Brand";
import { AppearanceController, ThemeSwapButton } from "./AppearanceController";
import { BoardIcon, DeviceIcon, LocationIcon, MenuIcon, OrgIcon, UsersIcon } from "./icons";

const DRAWER_ID = "app-drawer";

export type NavItem = { to: string; label: string; end?: boolean };

function navIcon(to: string): ReactNode {
  switch (to) {
    case "/":
      return <BoardIcon />;
    case "/devices":
      return <DeviceIcon />;
    case "/locations":
      return <LocationIcon />;
    case "/users":
      return <UsersIcon />;
    case "/internal":
      return <OrgIcon />;
    default:
      return null;
  }
}

function closeDrawer() {
  const el = document.getElementById(DRAWER_ID) as HTMLInputElement | null;
  if (el) el.checked = false;
}

export function Shell({
  me,
  subtitle,
  nav,
  roleLabel,
  onLogout,
}: {
  me: Me;
  subtitle: string;
  nav: NavItem[];
  roleLabel: string;
  onLogout: () => void;
}) {
  return (
    <div className="drawer lg:drawer-open">
      <input id={DRAWER_ID} type="checkbox" className="drawer-toggle" />
      <div className="drawer-content flex min-h-screen flex-col">
        <a
          href="#main"
          className="btn sr-only focus:not-sr-only focus:absolute focus:z-50 m-2"
        >
          Skip to content
        </a>
        <div className="navbar bg-base-200 lg:hidden">
          <div className="navbar-start">
            <label htmlFor={DRAWER_ID} className="btn btn-square btn-ghost drawer-button" aria-label="Open menu">
              <MenuIcon />
            </label>
          </div>
          <div className="navbar-center">
            <Brand compact />
          </div>
          <div className="navbar-end">
            <ThemeSwapButton />
          </div>
        </div>
        <main id="main" className="mx-auto w-full max-w-[90rem] flex-1 p-4 sm:p-6 lg:p-8">
          <Outlet context={me} />
        </main>
      </div>
      <div className="drawer-side z-40">
        <label htmlFor={DRAWER_ID} aria-label="close sidebar" className="drawer-overlay"></label>
        <aside className="flex min-h-full w-72 flex-col bg-base-200">
          <div className="px-4 pt-6 pb-2">
            <Brand subtitle={subtitle} />
          </div>
          <ul className="menu w-full flex-1 px-2">
            {nav.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) => (isActive ? "menu-active" : undefined)}
                  onClick={closeDrawer}
                >
                  {navIcon(item.to)}
                  {item.label}
                </NavLink>
              </li>
            ))}
          </ul>
          <div className="mt-auto flex flex-col gap-3 p-4">
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-sm" title={me.email}>
                {me.email}
              </span>
              <span className="badge badge-soft">{roleLabel}</span>
            </div>
            <AppearanceController />
            <button type="button" className="btn btn-ghost btn-block" onClick={onLogout}>
              Sign out
            </button>
          </div>
        </aside>
      </div>
    </div>
  );
}
