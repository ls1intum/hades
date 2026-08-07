import { NavLink, Outlet } from "react-router-dom";
import { Activity, LayoutDashboard, ListChecks, LogOut, Wifi, WifiOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-provider";
import { useAuth } from "@/context/auth";
import { useStream } from "@/hooks/useStream";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/", label: "Overview", icon: LayoutDashboard, end: true },
  { to: "/jobs", label: "Jobs", icon: ListChecks, end: false },
];

export function Layout() {
  const { username, logout } = useAuth();
  const connected = useStream(true);

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-4 px-4">
          <div className="flex items-center gap-2 font-semibold">
            <Activity className="size-5" />
            <span>Hades</span>
          </div>
          <nav className="flex items-center gap-1">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    "inline-flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                    isActive
                      ? "bg-secondary text-secondary-foreground"
                      : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                  )
                }
              >
                <item.icon className="size-4" />
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="ml-auto flex items-center gap-3">
            <span
              className="flex items-center gap-1.5 text-xs text-muted-foreground"
              title={connected ? "Live updates connected" : "Reconnecting..."}
            >
              {connected ? (
                <Wifi className="size-4 text-[var(--color-success)]" />
              ) : (
                <WifiOff className="size-4 text-muted-foreground" />
              )}
              <span className="hidden sm:inline">
                {connected ? "Live" : "Offline"}
              </span>
            </span>
            <ThemeToggle />
            <span className="hidden text-sm text-muted-foreground sm:inline">
              {username}
            </span>
            <Button variant="ghost" size="icon" onClick={logout} aria-label="Log out">
              <LogOut />
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
