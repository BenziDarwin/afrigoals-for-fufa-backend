from pathlib import Path
web=Path('F:/Afrigoals/Afrigoals_ecommerce')
p=web/'lib/auth/events.ts';s=p.read_text();s=s.replace('export function notifyAuthChange(change: AuthChange) {','export function notifyAuthChange(change: AuthChange) {\n if (typeof window === "undefined") return;');p.write_text(s,encoding='utf-8')
p=web/'tests/auth-session.test.ts'
with p.open('a',encoding='utf-8') as f:f.write('''
test("logout expires the active session cookie and forbids response caching", async () => {
 const {POST}=await import("../app/api/auth/logout/route");
 const res=await POST();
 assert.equal(res.status,200);
 assert.match(res.headers.get("set-cookie")??"",/afrigoals_token=;/);
 assert.match(res.headers.get("set-cookie")??"",/Max-Age=0/i);
 assert.match(res.headers.get("cache-control")??"",/no-store/);
});

test("a remembered device without an active session cannot open admin pages", async()=>{
 const {NextRequest}=await import("next/server");
 const {default:middleware}=await import("../middleware");
 const calls=mockFetch(()=>json({role:"admin"}));
 const res=await middleware(new NextRequest("http://localhost/admin/delivery-pricing",{headers:{cookie:"afrigoals_trusted_device=remembered"}}));
 assert.equal(new URL(res.headers.get("location")!).pathname,"/admin/signin");
 assert.equal(calls.length,0);
});

test("an expired admin session is redirected to sign-in", async()=>{
 const {NextRequest}=await import("next/server");
 const {default:middleware}=await import("../middleware");
 mockFetch(()=>json({error:"not authenticated"},401));
 const res=await middleware(new NextRequest("http://localhost/admin/delivery-pricing",{headers:{cookie:"afrigoals_token=expired"}}));
 assert.equal(new URL(res.headers.get("location")!).pathname,"/admin/signin");
});

test("logout immediately broadcasts signed-out state to all subscribers and other tabs",async()=>{
 const {subscribeAuthChanges,logoutSession}=await import("../lib/auth/events");
 const target=new EventTarget();
 const stored=new Map<string,string>([["auth_user","cached-admin"]]);
 const original=Object.getOwnPropertyDescriptor(globalThis,"window");
 Object.defineProperty(globalThis,"window",{configurable:true,value:{
  addEventListener:target.addEventListener.bind(target),removeEventListener:target.removeEventListener.bind(target),dispatchEvent:target.dispatchEvent.bind(target),
  localStorage:{setItem:(key:string,value:string)=>stored.set(key,value),removeItem:(key:string)=>stored.delete(key)},
 }});
 const first:unknown[]=[];const second:unknown[]=[];
 const stopA=subscribeAuthChanges(x=>first.push(x));const stopB=subscribeAuthChanges(x=>second.push(x));
 try {
  mockFetch(()=>json({ok:true}));
  await logoutSession();
  assert.deepEqual(first,["signed_out"]);assert.deepEqual(second,["signed_out"]);
  assert.equal(stored.has("auth_user"),false);
  const value=stored.get("afrigoals_auth_change")!;
  assert.equal(JSON.parse(value).change,"signed_out");
  const remote=new Event("storage");
  Object.defineProperties(remote,{key:{value:"afrigoals_auth_change"},newValue:{value}});
  target.dispatchEvent(remote);
  assert.deepEqual(first,["signed_out","signed_out"]);
  assert.deepEqual(second,["signed_out","signed_out"]);
 }finally{stopA();stopB();if(original)Object.defineProperty(globalThis,"window",original);else Reflect.deleteProperty(globalThis,"window");}
});

test("failed logout is reported instead of pretending the cookie was cleared",async()=>{
 const {logoutSession}=await import("../lib/auth/events");
 mockFetch(()=>json({error:"temporary failure"},503));
 await assert.rejects(logoutSession(),/Could not sign out/);
});
''')
