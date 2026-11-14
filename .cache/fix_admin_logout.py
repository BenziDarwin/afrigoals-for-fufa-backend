from pathlib import Path
web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:90]);p.write_text(s.replace(a,b),encoding='utf-8')
# Both entry points render the exact same form and completion flow.
p=web/'app/(app)/signin/page.tsx';s=p.read_text(encoding='utf-8')
s=s.replace('import { Input } from "@/components/ui/input";','import { Input } from "@/components/ui/input";\nimport { verifyAdminSession } from "@/lib/auth/admin-signin";\nimport { notifyAuthChange } from "@/lib/auth/events";')
s=s.replace('if (!value.startsWith("/") || value.startsWith("//")) return "/";','if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\\\")) return "/";')
s=s.replace('function SignInInner() {','function SignInInner({ adminOnly = false }: { adminOnly?: boolean }) {')
s=s.replace('const next = safeNext(searchParams.get("next"));','const requestedNext = safeNext(searchParams.get("next"));\n  const next = adminOnly ? ((requestedNext === "/admin" || requestedNext.startsWith("/admin/")) && requestedNext.split("?")[0] !== "/admin/signin" ? requestedNext : "/admin") : requestedNext;')
a='  function notifyAuthChange() {\n    window.dispatchEvent(new Event("auth-changed"));\n  }\n\n';assert a in s;s=s.replace(a,'')
s=s.replace('function completeAuth(data: AuthResponse) {','async function completeAuth(data: AuthResponse) {')
s=s.replace('    notifyAuthChange();','''    if (adminOnly) {
      const sessionError = await verifyAdminSession();
      if (sessionError) { setError(sessionError); return; }
    }
    notifyAuthChange("signed_in");''')
s=s.replace('router.replace("/admin");','router.replace(adminOnly ? next : "/admin");')
s=s.replace('        completeAuth(data);','        await completeAuth(data);').replace('      completeAuth(data);','      await completeAuth(data);')
s=s.replace('        Sign in\n','        {adminOnly ? "Admin sign in" : "Sign in"}\n')
s=s.replace('export default function SignInPage() {','export default function SignInPage({ adminOnly = false }: { adminOnly?: boolean }) {')
s=s.replace('<SignInInner />','<SignInInner adminOnly={adminOnly} />')
target=web/'components/auth/SignInPage.tsx';target.parent.mkdir(parents=True,exist_ok=True);target.write_text(s,encoding='utf-8')
p.write_text('"use client";\nimport SignInPage from "@/components/auth/SignInPage";\nexport default function Page() { return <SignInPage />; }\n',encoding='utf-8')
(web/'app/(admin)/admin/signin/page.tsx').write_text('"use client";\nimport SignInPage from "@/components/auth/SignInPage";\nexport default function Page() { return <SignInPage adminOnly />; }\n',encoding='utf-8')
(web/'lib/auth/events.ts').write_text('''export type AuthChange = "signed_in" | "signed_out";
const STORAGE_KEY = "afrigoals_auth_change";

export function notifyAuthChange(change: AuthChange) {
 window.dispatchEvent(new CustomEvent("auth-changed", { detail: change }));
 try { window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ change, nonce: Date.now() + Math.random() })); } catch {}
}

export function subscribeAuthChanges(listener: (change?: AuthChange) => void) {
 const local = (event: Event) => listener((event as CustomEvent<AuthChange>).detail);
 const remote = (event: StorageEvent) => {
  if (event.key !== STORAGE_KEY || !event.newValue) return;
  try { const {change} = JSON.parse(event.newValue); if (change === "signed_in" || change === "signed_out") listener(change); } catch {}
 };
 window.addEventListener("auth-changed", local);
 window.addEventListener("storage", remote);
 return () => {window.removeEventListener("auth-changed",local);window.removeEventListener("storage",remote);};
}

export async function logoutSession(fetcher: typeof fetch = fetch) {
 const response = await fetcher("/api/auth/logout", {method:"POST",credentials:"same-origin",cache:"no-store"});
 if (!response.ok) throw new Error("Could not sign out. Please try again.");
 try { window.localStorage.removeItem("auth_user"); } catch {}
 notifyAuthChange("signed_out");
}
''',encoding='utf-8')
p=web/'lib/auth/client.ts'
edit(p,'import { useCallback, useEffect, useRef, useState } from "react";','import { useCallback, useEffect, useRef, useState } from "react";\nimport { logoutSession, subscribeAuthChanges } from "@/lib/auth/events";')
s=p.read_text();start=s.index('    try {',s.index('  const signOut ='));end=s.index('\n  }, []);',start)
s=s[:start]+'''    await logoutSession();
    setState({ status: "signed_out", user: null });'''+s[end:]
a='''    function handleAuthChanged() {
      refresh();
    }''';assert a in s;s=s.replace(a,'''    const unsubscribe = subscribeAuthChanges((change) => {
      if (change === "signed_out") {
        ++requestVersion.current;
        setState({ status: "signed_out", user: null });
      } else { refresh(); }
    });
    function handlePageShow(event: PageTransitionEvent) {
      if (event.persisted) { setState({status:"loading",user:null}); refresh(); }
    }''')
s=s.replace('    window.addEventListener("auth-changed", handleAuthChanged);','    window.addEventListener("pageshow", handlePageShow);')
s=s.replace('      window.removeEventListener("auth-changed", handleAuthChanged);','      unsubscribe();\n      window.removeEventListener("pageshow", handlePageShow);')
p.write_text(s,encoding='utf-8')
# A failed admin role check must also invalidate other mounted auth hooks.
p=web/'lib/auth/admin-signin.ts'
edit(p,'// Confirm the server session','import { logoutSession } from "@/lib/auth/events";\n\n// Confirm the server session')
edit(p,'await fetcher("/api/auth/logout", { method: "POST" });','await logoutSession(fetcher);')
p=web/'app/(admin)/admin/layout.tsx'
edit(p,'import { useState } from "react";','import { useEffect, useState } from "react";')
edit(p,'const { signOut } = useAuthState();','const { signOut, status, user } = useAuthState();\n  const isSignInPage = pathname === "/admin/signin";\n  const canAccess = status === "signed_in" && user?.role === "admin";')
edit(p,'  const [isSigningOut, setIsSigningOut] = useState(false);','''  const [isSigningOut, setIsSigningOut] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  useEffect(() => {
    if (!isSignInPage && status !== "loading" && !canAccess) {
      router.replace(`/admin/signin?next=${encodeURIComponent(pathname)}`);
    }
  }, [isSignInPage, status, canAccess, pathname, router]);''')
edit(p,'    setIsSigningOut(true);','    setIsSigningOut(true);\n    setLogoutError("");')
edit(p,'      router.replace("/admin/signin");\n      router.refresh();','      window.location.replace("/admin/signin");\n    } catch (error) {\n      setLogoutError(error instanceof Error ? error.message : "Could not sign out");')
edit(p,'  return (\n    <div className="flex min-h-screen','''  if (isSignInPage) return <main className="min-h-screen bg-zinc-50 dark:bg-zinc-950">{children}</main>;
  if (!canAccess || isSigningOut) return <main className="p-8" aria-busy="true">{isSigningOut ? "Signing out…" : "Checking admin access…"}</main>;

  return (
    <div className="flex min-h-screen''')
edit(p,'            {pathname !== "/admin/signin" ? (','            {logoutError && <p role="alert" className="px-3 text-sm text-red-600">{logoutError}</p>}\n            {pathname !== "/admin/signin" ? (')
# Prevent caching the active-session response and explicit logout result.
edit(web/'app/api/auth/logout/route.ts','NextResponse.json({ ok: true })','NextResponse.json({ ok: true }, { headers: { "Cache-Control": "no-store, private" } })')
edit(web/'app/api/auth/me/route.ts','return NextResponse.json(data);','return NextResponse.json(data, { headers: { "Cache-Control": "no-store, private" } });')
