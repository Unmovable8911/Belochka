import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from "react"

type Theme = "light" | "dark" | "system"
type ResolvedTheme = "light" | "dark"

interface ThemeContextValue {
  theme: Theme
  resolvedTheme: ResolvedTheme
  setTheme: (theme: string) => void
  themes: (Theme | string)[]
  systemTheme: ResolvedTheme
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

const STORAGE_KEY = "theme"
const THEME_CLASS = "dark"

function getSystemTheme(): ResolvedTheme {
  if (typeof window === "undefined") return "dark"
  if (typeof window.matchMedia !== "function") return "dark"
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
  } catch {
    return "dark"
  }
}

function getStoredTheme(): Theme {
  if (typeof window === "undefined") return "system"
  if (typeof localStorage === "undefined") return "system"
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored
    }
  } catch {
    // localStorage unavailable — use system
  }
  return "system"
}

function resolveTheme(theme: Theme): ResolvedTheme {
  if (theme === "system") return getSystemTheme()
  return theme
}

function applyTheme(resolved: ResolvedTheme) {
  const root = document.documentElement
  if (resolved === "dark") {
    root.classList.add(THEME_CLASS)
  } else {
    root.classList.remove(THEME_CLASS)
  }
  root.style.colorScheme = resolved
}

function isValidTheme(t: string): t is Theme {
  return t === "light" || t === "dark" || t === "system"
}

export function ThemeProvider({ children, defaultTheme = "system" }: {
  children: ReactNode
  defaultTheme?: Theme
}) {
  const [theme, setThemeState] = useState<Theme>(getStoredTheme)
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(getSystemTheme)
  const [mounted, setMounted] = useState(false)

  // Mark as mounted to avoid hydration mismatch
  useEffect(() => {
    setMounted(true)
  }, [])

  // Apply theme whenever resolved theme changes
  const resolved = resolveTheme(theme)

  useEffect(() => {
    applyTheme(resolved)
  }, [resolved])

  // Listen for system theme changes
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)")
    const handler = (e: MediaQueryListEvent | MediaQueryList) => {
      const next = e.matches ? "dark" : "light"
      setSystemTheme(next)
    }
    mq.addEventListener("change", handler)
    return () => mq.removeEventListener("change", handler)
  }, [])

  const setTheme = useCallback((next: string) => {
    if (!isValidTheme(next)) return
    setThemeState(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // localStorage unavailable — ignore
    }
  }, [])

  // Use default theme until mounted to avoid hydration mismatch
  // (the blocking script in index.html already applied the correct class)
  const displayTheme = mounted ? theme : defaultTheme
  const displayResolved = mounted ? resolved : resolveTheme(defaultTheme)

  return (
    <ThemeContext value={{
      theme: displayTheme,
      resolvedTheme: displayResolved,
      setTheme,
      themes: ["light", "dark", "system"],
      systemTheme,
    }}>
      {children}
    </ThemeContext>
  )
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext)
  if (!ctx) {
    // Fallback: return safe defaults when used outside ThemeProvider.
    // This matches next-themes behavior (returns a no-op stub).
    return {
      theme: "system",
      resolvedTheme: getSystemTheme(),
      setTheme: () => {},
      themes: ["light", "dark", "system"],
      systemTheme: getSystemTheme(),
    }
  }
  return ctx
}
