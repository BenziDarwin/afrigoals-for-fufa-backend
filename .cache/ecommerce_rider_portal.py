from pathlib import Path
root=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
put('lib/api/rider-proxy.ts','''import "server-only";
import{cookies}from"next/headers";
import{getBackendApiBaseUrl}from"@/lib/api/backend";
export const RIDER_COOKIE="afrigoals_rider_token";
export async function riderFetch(path:string,init:RequestInit={}){
 const token=(await cookies()).get(RIDER_COOKIE)?.value;
 if(!token)return new Response(JSON.stringify({error:"Rider sign-in required"}),{status:401,headers:{"Content-Type":"application/json"}});
 return fetch(`${getBackendApiBaseUrl()}${path}`,{...init,headers:{...init.headers,Authorization:`Bearer ${token}`,Accept:"application/json"},cache:"no-store"});
}
''')
put('app/api/rider/login/route.ts','''import{NextResponse}from"next/server";
import{getBackendApiBaseUrl}from"@/lib/api/backend";
import{RIDER_COOKIE}from"@/lib/api/rider-proxy";
export async function POST(request:Request){
 let input;try{input=await request.json()}catch{return NextResponse.json({error:"Invalid JSON"},{status:400})}
 const response=await fetch(`${getBackendApiBaseUrl()}/riders/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email:input.email,password:input.password}),cache:"no-store"});
 const data=await response.json();if(!response.ok)return NextResponse.json({error:data.error},{status:response.status});
 if(!data.token)return NextResponse.json({error:"Rider session was not returned"},{status:502});
 const result=NextResponse.json({rider:data.rider});result.cookies.set(RIDER_COOKIE,data.token,{httpOnly:true,sameSite:"lax",secure:process.env.NODE_ENV==="production",path:"/",maxAge:7*86400});return result;
}
export async function DELETE(){const response=NextResponse.json({success:true});response.cookies.set(RIDER_COOKIE,"",{httpOnly:true,path:"/",maxAge:0});return response}
''')
put('app/api/rider/orders/route.ts','''import{NextResponse}from"next/server";
import{riderFetch}from"@/lib/api/rider-proxy";
export async function GET(){const r=await riderFetch("/rider/orders");return new NextResponse(await r.text(),{status:r.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}})}
''')
put('app/api/rider/orders/[id]/accept/route.ts','''import{NextResponse}from"next/server";
import{riderFetch}from"@/lib/api/rider-proxy";
export async function POST(_request:Request,{params}:{params:Promise<{id:string}>}){const{id}=await params;const r=await riderFetch(`/rider/orders/${encodeURIComponent(id)}/accept`,{method:"POST"});return new NextResponse(await r.text(),{status:r.status,headers:{"Content-Type":"application/json"}})}
''')
put('app/(app)/rider/page.tsx','''"use client";
import{useEffect,useState}from"react";
import{Button}from"@/components/ui/button";
import{formatPrice}from"@/lib/utils";
type Order={id:string;orderNumber:string;status:string;deliveryStatus?:string;deliveryAddress:string;deliveryFee:number;deliveryServiceFee?:number;riderPayout?:number};
export default function RiderPage(){
 const[orders,setOrders]=useState<Order[]>([]);const[signedIn,setSignedIn]=useState(false);const[email,setEmail]=useState("");const[password,setPassword]=useState("");const[error,setError]=useState("");const[busy,setBusy]=useState(false);
 async function load(){const r=await fetch("/api/rider/orders",{cache:"no-store"});if(r.status===401){setSignedIn(false);return}const data=await r.json();if(!r.ok)throw Error(data.error);setOrders(data);setSignedIn(true)}
 useEffect(()=>{load().catch(e=>setError(e.message))},[]);
 async function login(e:React.FormEvent){e.preventDefault();setBusy(true);setError("");try{const r=await fetch("/api/rider/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email,password})});const data=await r.json();if(!r.ok)throw Error(data.error);setPassword("");await load()}catch(e){setError(e instanceof Error?e.message:"Could not sign in")}finally{setBusy(false)}}
 async function accept(id:string){setBusy(true);setError("");try{const r=await fetch(`/api/rider/orders/${encodeURIComponent(id)}/accept`,{method:"POST"});if(!r.ok){const data=await r.json();throw Error(data.error)}await load()}catch(e){setError(e instanceof Error?e.message:"Could not accept")}finally{setBusy(false)}}
 const earned=orders.filter(o=>o.status==="delivered"||o.deliveryStatus==="delivered").reduce((total,o)=>total+(o.riderPayout??0),0);
 return <main className="mx-auto max-w-5xl space-y-6 p-6"><h1 className="text-2xl font-semibold">Rider dashboard</h1>{error&&<p role="alert" className="text-red-600">{error}</p>}{!signedIn?<form onSubmit={login} className="max-w-md space-y-4"><p>Sign in with an approved rider account.</p><label htmlFor="rider-email" className="block">Email<input id="rider-email" type="email" required value={email} onChange={e=>setEmail(e.target.value)} className="block w-full rounded border p-2"/></label><label htmlFor="rider-password" className="block">Password<input id="rider-password" type="password" autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)} className="block w-full rounded border p-2"/></label><Button disabled={busy}>Sign in</Button></form>:<><div className="flex flex-wrap items-center justify-between gap-4"><p className="text-xl font-semibold">Earned on completed deliveries: {formatPrice(earned)}</p><Button variant="outline" onClick={()=>fetch("/api/rider/login",{method:"DELETE"}).then(()=>{setSignedIn(false);setOrders([])})}>Sign out</Button></div><p>Your payout is the delivery fee minus the Afrigoals commission. Pending deliveries are not counted as earned.</p>{orders.length===0&&<p>No assigned deliveries.</p>}{orders.map(o=><article key={o.id} className="space-y-2 rounded-xl border p-5"><h2 className="font-semibold">{o.orderNumber}</h2><p>{o.deliveryAddress}</p><p>Status: {o.status} · {o.deliveryStatus}</p><p>Delivery fee: {formatPrice(o.deliveryFee)} · Commission: {formatPrice(o.deliveryServiceFee??0)}</p><p className="font-semibold">Your payout: {formatPrice(o.riderPayout??0)}</p>{o.deliveryStatus==="assigned"&&<Button disabled={busy} onClick={()=>void accept(o.id)}>Accept delivery</Button>}</article>)}</>}</main>
}
''')
put('app/api/admin/riders/route.ts','''import{NextResponse}from"next/server";import{authedFetch}from"@/lib/api/proxy";
export async function GET(){const r=await authedFetch("/admin/riders");return new NextResponse(await r.text(),{status:r.status,headers:{"Content-Type":"application/json"}})}
''')
put('app/api/admin/riders/[id]/status/route.ts','''import{NextResponse}from"next/server";import{authedFetch}from"@/lib/api/proxy";
export async function PATCH(request:Request,{params}:{params:Promise<{id:string}>}){const{id}=await params;const r=await authedFetch(`/admin/riders/${encodeURIComponent(id)}/status`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:await request.text()});return new NextResponse(await r.text(),{status:r.status,headers:{"Content-Type":"application/json"}})}
''')
put('app/(admin)/admin/riders/page.tsx','''"use client";
import{useEffect,useState}from"react";import{Button}from"@/components/ui/button";
export default function RidersPage(){const[riders,setRiders]=useState<{id:string;name:string;email:string;status:string}[]>([]);const[error,setError]=useState("");const[busy,setBusy]=useState(false);
async function load(){const r=await fetch("/api/admin/riders",{cache:"no-store"});const data=await r.json();if(!r.ok)throw Error(data.error);setRiders(data)}useEffect(()=>{load().catch(e=>setError(e.message))},[]);
async function review(id:string,status:string){setBusy(true);try{const r=await fetch(`/api/admin/riders/${encodeURIComponent(id)}/status`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({status})});const data=await r.json();if(!r.ok)throw Error(data.error);await load()}catch(e){setError(e instanceof Error?e.message:"Could not update rider")}finally{setBusy(false)}}
return <main className="space-y-6 p-6"><h1 className="text-2xl font-semibold">Rider accounts</h1>{error&&<p role="alert">{error}</p>}{riders.map(r=><article key={r.id} className="space-y-3 rounded border p-4"><h2>{r.name} · {r.email}</h2><p>{r.status}</p><div className="flex flex-wrap gap-3">{["approved","rejected","suspended"].map(status=><Button key={status} disabled={busy||r.status===status} variant="outline" onClick={()=>void review(r.id,status)}>{status}</Button>)}</div></article>)}</main>}
''')
p=root/'app/(admin)/admin/layout.tsx';s=p.read_text(encoding='utf-8').replace('const navItems = [','const navItems = [\n {label:"Rider accounts",href:"/admin/riders",icon:Package},',1);p.write_text(s,encoding='utf-8')
print('Added rider dashboard, isolated rider sessions and admin rider review screens.')
