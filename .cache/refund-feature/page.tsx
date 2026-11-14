"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { formatDate, formatPrice } from "@/lib/utils";

type Refund = {
 id: string; orderId: string; orderNumber: string; customerEmail: string; customerName: string;
 originalTransactionId: string; maskedPhone: string; amount: number; currency: string;
 kind: string; reason: string; settlementNotes: string; status: string;
 providerReference: string; failureReason: string; createdAt: string; verifiedAt?: string;
};
const initial = {transactionId:"",phone:"",customerName:"",amount:"",kind:"cancellation",reason:"",settlementNotes:""};

export default function RefundsPage() {
 const [rows,setRows]=useState<Refund[]>([]);
 const [form,setForm]=useState(initial);
 const [review,setReview]=useState<Refund|null>(null);
 const [confirmed,setConfirmed]=useState(false);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [message,setMessage]=useState("");
 const [search,setSearch]=useState("");
 const load=useCallback(async()=>{
  try {
   const res=await fetch(`/api/admin/refunds?search=${encodeURIComponent(search)}`,{cache:"no-store"});
   const data=await res.json();
   if(!res.ok)throw new Error(data.error||"Could not load refunds");
   setRows(data);
  } catch(e){setError(e instanceof Error?e.message:"Could not load refunds");}
 },[search]);
 useEffect(()=>{void load();},[load]);
 async function prepare(e:React.FormEvent) {
  e.preventDefault();setBusy(true);setError("");setMessage("");
  try {
   const res=await fetch("/api/admin/refunds",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({...form,amount:Number(form.amount)})});
   const data=await res.json();if(!res.ok)throw new Error(data.error||"Could not prepare refund");
   setReview(data);setConfirmed(false);setForm(initial);await load();
  }catch(e){setError(e instanceof Error?e.message:"Could not prepare refund");}finally{setBusy(false);}
 }
 async function act(row:Refund,action:"submit"|"reconcile"|"reject") {
  setBusy(true);setError("");setMessage("");
  try {
   const res=await fetch(`/api/admin/refunds/${encodeURIComponent(row.id)}/${action}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({confirm:action==="submit"&&confirmed})});
   const data=await res.json();if(!res.ok)throw new Error(data.error||"Refund action failed");
   setMessage(action==="reject"?"Draft rejected. No money was sent.":data.status==="succeeded"?"IOTEC confirmed the refund was paid.":`Refund status: ${data.status}. ${data.failureReason||"Use Recheck to confirm the final outcome."}`);
   setReview(null);setConfirmed(false);await load();
  }catch(e){setError(e instanceof Error?e.message:"Refund action failed");await load();}finally{setBusy(false);}
 }
 const field="mt-1 w-full rounded-lg border bg-background px-3 py-2";
 return <div className="space-y-6">
  <div><h1 className="text-2xl font-bold">Refunds and returns</h1><p className="mt-2 text-sm text-muted-foreground">Refund verified IOTEC payments to the original payer. Track cancellations, product complaints and agreed settlements.</p></div>
  <Link href="/admin/reconciliation" className="inline-block text-sm underline">View payment and refund reconciliation</Link>
  {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-red-700">{error}</p>}
  {message&&<p role="status" className="rounded-lg border p-3">{message}</p>}
  <form onSubmit={prepare} className="space-y-4 rounded-xl border p-5">
   <h2 className="text-lg font-semibold">Prepare a refund</h2>
   <div className="grid gap-4 md:grid-cols-2">
    <label>Original IOTEC transaction ID<input required className={field} value={form.transactionId} onChange={e=>setForm({...form,transactionId:e.target.value})}/></label>
    <label>Customer name<input required minLength={2} maxLength={150} className={field} value={form.customerName} onChange={e=>setForm({...form,customerName:e.target.value})}/></label>
    <label>Original payer phone number<input required type="tel" placeholder="0771234567" className={field} value={form.phone} onChange={e=>setForm({...form,phone:e.target.value})}/></label>
    <label>Refund amount (UGX)<input required type="number" min={500} max={1000000000} step={1} className={field} value={form.amount} onChange={e=>setForm({...form,amount:e.target.value})}/></label>
    <label>Refund type<select className={field} value={form.kind} onChange={e=>setForm({...form,kind:e.target.value})}><option value="cancellation">Cancellation before delivery</option><option value="return_settlement">Return or complaint after delivery</option></select></label>
    <label>Reason<input required minLength={5} maxLength={2000} className={field} value={form.reason} onChange={e=>setForm({...form,reason:e.target.value})}/></label>
   </div>
   <label className="block">Agreed settlement and return details<textarea required={form.kind==="return_settlement"} minLength={form.kind==="return_settlement"?10:undefined} maxLength={4000} className={field} placeholder="Record the customer agreement, returned goods, amount and any deductions." value={form.settlementNotes} onChange={e=>setForm({...form,settlementNotes:e.target.value})}/></label>
   <p className="text-sm text-muted-foreground">Full and partial refunds are supported. IOTEC requires at least UGX 500 per payout. The original payment and exact payer number are checked before a draft is created. Preparing a draft reserves the amount without sending money.</p>
   <button disabled={busy} className="rounded-lg bg-primary px-4 py-2 text-primary-foreground disabled:opacity-50">{busy?"Working…":"Verify payment and prepare refund"}</button>
  </form>
  {review&&<section aria-label="Review refund" className="space-y-4 rounded-xl border-2 border-amber-400 p-5">
   <h2 className="text-lg font-semibold">Review before sending money</h2>
   <dl className="grid gap-2 text-sm"><div>Order: <strong>{review.orderNumber}</strong></div><div>Customer: {review.customerName} · {review.customerEmail}</div><div>Mobile money: {review.maskedPhone}</div><div>Original transaction: {review.originalTransactionId}</div><div>Refund: <strong>{formatPrice(review.amount,"ugx")}</strong></div><div>Reason: {review.reason}</div><div>Settlement: {review.settlementNotes||"—"}</div></dl>
   <p className="text-sm">Sending a cancellation refund cancels the order and restores its reserved stock. After-delivery settlements keep the delivery history; inspect returned stock separately. IOTEC may charge a payout fee.</p>
   <label className="flex items-start gap-2"><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/><span>I have verified the customer agreement and approve sending this amount to the original payer.</span></label>
   <div className="flex flex-wrap gap-3"><button disabled={busy||!confirmed} onClick={()=>void act(review,"submit")} className="rounded-lg bg-primary px-4 py-2 text-primary-foreground disabled:opacity-50">Confirm and send refund</button><button disabled={busy} onClick={()=>{setReview(null);setConfirmed(false);}} className="rounded-lg border px-4 py-2">Close review</button></div>
  </section>}
  <section className="space-y-3"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-lg font-semibold">Refund register</h2><div className="flex gap-2"><input aria-label="Search refunds" placeholder="Order, email or transaction ID" className="rounded-lg border bg-background px-3 py-2" value={search} onChange={e=>setSearch(e.target.value)}/><button disabled={busy} className="rounded-lg border px-3 py-2" onClick={()=>void load()}>Refresh</button></div></div>
   <p className="text-sm text-muted-foreground">Latest 500 matching records. Unknown or processing refunds must be rechecked, never resubmitted.</p>
   <div className="overflow-x-auto rounded-xl border"><table className="w-full text-left text-sm"><thead className="bg-muted"><tr>{["Order / customer","Payment / refund transaction","Amount / status","Reason / agreement","Actions"].map(x=><th key={x} className="p-3">{x}</th>)}</tr></thead><tbody>
   {rows.length===0&&<tr><td colSpan={5} className="p-4 text-muted-foreground">No refunds found.</td></tr>}
   {rows.map(row=><tr key={row.id} className="border-t align-top"><td className="space-y-1 p-3"><Link className="underline" href={`/orders/${row.orderId}`}>{row.orderNumber}</Link><p>{row.customerName}</p><p>{row.customerEmail}</p><p>{row.maskedPhone}</p></td><td className="max-w-64 break-all p-3"><p>Payment: {row.originalTransactionId}</p><p>Refund: {row.providerReference||"Awaiting submission or confirmation"}</p><p className="mt-2 text-xs text-muted-foreground">Case: {row.id}</p><p className="text-xs text-muted-foreground">{formatDate(row.createdAt,"datetime")}</p></td><td className="p-3"><strong>{formatPrice(row.amount,"ugx")}</strong><p className={row.status==="succeeded"?"font-medium text-green-700":"font-medium"}>{row.status==="succeeded"?"Refund paid":row.status}</p><p className="mt-2 max-w-60 text-xs">{row.failureReason}</p></td><td className="max-w-72 whitespace-pre-wrap p-3"><p>{row.kind==="cancellation"?"Cancellation":"Return / complaint"}</p><p>{row.reason}</p><p className="mt-2 text-muted-foreground">{row.settlementNotes}</p></td><td className="space-y-2 p-3">{row.status==="draft"?<><button disabled={busy} className="block underline" onClick={()=>{setReview(row);setConfirmed(false);}}>Review and send</button><button disabled={busy} className="block underline" onClick={()=>void act(row,"reject")}>Reject draft</button></>:row.status!=="rejected"&&row.status!=="succeeded"?<button disabled={busy} className="underline" onClick={()=>void act(row,"reconcile")}>Recheck with IOTEC</button>:null}</td></tr>)}
   </tbody></table></div>
  </section>
 </div>;
}
