from pathlib import Path
web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(p,a,b):
 s=p.read_text(encoding='utf-8');assert a in s,(str(p),a[:80]);p.write_text(s.replace(a,b),encoding='utf-8')
(web/'lib/auth/admin-signin.ts').write_text('''// Confirm the server session rather than trusting the login response role.
export async function verifyAdminSession(fetcher: typeof fetch = fetch): Promise<string | null> {
 const response = await fetcher("/api/auth/me", { cache: "no-store" });
 const user = await response.json().catch(() => null);
 if (!response.ok) return "Could not verify your session. Please sign in again.";
 if (user?.role !== "admin") {
  await fetcher("/api/auth/logout", { method: "POST" });
  return "This account is not an admin.";
 }
 return null;
}
''',encoding='utf-8')
p=web/'app/(admin)/admin/signin/page.tsx'
edit(p,'import { Input } from "@/components/ui/input";','import { Input } from "@/components/ui/input";\nimport { verifyAdminSession } from "@/lib/auth/admin-signin";')
edit(p,'if (!value.startsWith("/admin")) return "/admin";','if (value !== "/admin" && !value.startsWith("/admin/")) return "/admin";\n  if (value.includes("\\\\") || value.split("?")[0] === "/admin/signin") return "/admin";')
edit(p,'  async function onPasswordSubmit','''  async function finishSignIn() {
    const sessionError = await verifyAdminSession();
    if (sessionError) { setError(sessionError); return; }
    router.replace(next);
    router.refresh();
  }

  async function onPasswordSubmit''')
edit(p,'      if (!(data as any)?.requires2fa','''      if (data?.token && data?.user) {
        setPassword("");
        await finishSignIn();
        return;
      }

      if (!(data as any)?.requires2fa''')
start='      const meRes = await fetch("/api/auth/me", { cache: "no-store" });'
s=p.read_text();a=s.index(start);b=s.index('      router.refresh();',a)+len('      router.refresh();');s=s[:a]+'      await finishSignIn();'+s[b:];p.write_text(s,encoding='utf-8')
edit(p,'Admin access requires password and email verification code.','Sign in with your admin password. A new or unverified browser also requires an email verification code.')
# Invalid upstream success bodies should be reported as API errors, not fake code challenges.
p=web/'app/api/auth/login/route.ts'
edit(p,'  return NextResponse.json({\n    requires2fa:','''  if (!(data as any)?.requires2fa || typeof (data as any)?.challengeId !== "string" || !(data as any).challengeId.trim()) {
    return NextResponse.json({ error: "The sign-in service returned an incomplete response. Please try again." }, { status: 502 });
  }

  return NextResponse.json({
    requires2fa:''')
p=web/'tests/auth-session.test.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
test("trusted admin login forwards its device token and establishes a session", async () => {
 setRequestCookies({afrigoals_trusted_device:"trusted-browser"});
 const { TRUSTED_DEVICE_COOKIE_NAME }=await import("../lib/auth/constants");
 setRequestCookies({[TRUSTED_DEVICE_COOKIE_NAME]:"trusted-browser"});
 const { POST }=await import("../app/api/auth/login/route");
 const calls=mockFetch(()=>json({token:"admin-session",user:{id:"a1",role:"admin"}}));
 const res=await POST(jsonRequest({email:"admin@example.com",password:"secret"}));
 assert.equal(headerValue(calls[0].init,"X-Trusted-Device"),"trusted-browser");
 assert.match(res.headers.get("set-cookie")??"",/afrigoals_token=admin-session/);
 assert.equal(res.status,200);
});

test("admin sign-in verifies the established session before proceeding", async()=>{
 const {verifyAdminSession}=await import("../lib/auth/admin-signin");
 const calls=mockFetch(()=>json({id:"a1",role:"admin"}));
 assert.equal(await verifyAdminSession(),null);
 assert.equal(calls[0].url,"/api/auth/me");
 assert.equal(calls.length,1);
});

test("admin sign-in rejects customer sessions and clears their login", async()=>{
 const {verifyAdminSession}=await import("../lib/auth/admin-signin");
 const calls=mockFetch(()=>json({id:"u1",role:"user"}));
 assert.equal(await verifyAdminSession(),"This account is not an admin.");
 assert.equal(calls[1].url,"/api/auth/logout");
 assert.equal(calls[1].init?.method,"POST");
});

test("malformed login success does not pretend a verification code was sent", async()=>{
 setRequestCookies({});
 const {POST}=await import("../app/api/auth/login/route");
 mockFetch(()=>json({}));
 const res=await POST(jsonRequest({email:"admin@example.com",password:"secret"}));
 assert.equal(res.status,502);
 assert.equal(res.headers.get("set-cookie"),null);
});
''')
