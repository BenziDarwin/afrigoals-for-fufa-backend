"use client";

import { useCallback, useEffect, useState } from "react";
import { formatDate, formatPrice } from "@/lib/utils";
import { refundRequestLabel, type CustomerRefundRequest } from "@/lib/orders/refund-requests";

type Request = CustomerRefundRequest & {phone:string;orderStatus:string};
export function AdminRefundRequests<T>({onApproved}:{onApproved:(refund:T)=>void}) {
 const [items,setItems]=useState<Request[]>([]);
 const [filter,setFilter]=useState("requested");
 const [selected,setSelected]=useState<Request|null>(null);
 const [amount,setAmount]=useState("");const [kind,setKind]=useState("cancellation");const [note,setNote]=useState("");
 const [busy,setBusy]=useState(false);const [error,setError]=useState("");const [message,setMessage]=useState("");
 const load=useCallback(async()=>{
  try{const res=await fetch(`/api/admin/refund-requests?status=${encodeURIComponent(filter)}`,{cache:"no-store"});const data=await res.json();if(!res.ok)throw new Error(data.error||"Could not load customer requests");setItems(data);}catch(e){setError(e instanceof Error?e.message:"Could not load customer requests");}
 },[filter]);
 useEffect(()=>{void load();},[load]);
 async function decide(action:"approve"|"decline") {
  if(!selected)return;setBusy(true);setError("");setMessage("");
  try {
   const res=await fetch(`/api/admin/refund-requests/${encodeURIComponent(selected.id)}/${action}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({amount:Number(amount),kind,reviewNote:note})});
   const data=await res.json();if(!res.ok)throw new Error(data.error||"Could not save review");
   setSelected(null);setNote("");await load();
   if(action==="approve"){setMessage("Request approved. Review the prepared refund below and confirm to send the money.");onApproved(data);}else{setMessage("Request declined. The customer can see your response on their order.");}
  }catch(e){setError(e instanceof Error?e.message:"Could not save review");}finally{setBusy(false);}
 }
 const field="mt-1 w-full rounded-md border bg-background px-3 py-2";
 return <section className="space-y-4 rounded-xl border p-5">
  <div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-lg font-semibold">Customer refund requests</h2><div className="flex gap-3"><select aria-label="Filter refund requests" value={filter} onChange={e=>setFilter(e.target.value)} className="rounded-md border bg-background px-3 py-2"><option value="requested">Awaiting review</option><option value="approved">Approved</option><option value="declined">Declined</option><option value="">All requests</option></select><button disabled={busy} onClick={()=>void load()} className="underline">Refresh</button></div></div>
  {error&&<p role="alert" className="text-sm text-red-600">{error}</p>}{message&&<p role="status" className="text-sm">{message}</p>}
  {items.length===0&&<p className="text-sm text-muted-foreground">No matching customer requests.</p>}
  <p className="text-xs text-muted-foreground">Up to 500 matching requests, oldest first.</p>
  {items.map(item=><article key={item.id} className="space-y-2 rounded-lg border p-4 text-sm"><div className="flex flex-wrap justify-between gap-2"><strong>{item.orderNumber} · {formatPrice(item.amount,"ugx")}</strong><span>{refundRequestLabel(item)}</span></div><p>{item.customerName} · {item.customerEmail} · {item.phone}</p><p>Order stage: {item.orderStatus} · {formatDate(item.createdAt,"datetime")}</p><p className="break-all">Original IOTEC transaction: {item.originalTransactionId}</p>{item.receiptReference&&<p>Customer receipt: {item.receiptReference}</p>}<p className="whitespace-pre-wrap">Reason: {item.reason}</p><p className="whitespace-pre-wrap text-muted-foreground">{item.details}</p>{item.reviewNote&&<p>Admin response: {item.reviewNote}</p>}{item.status==="requested"&&<button disabled={busy} className="font-medium underline" onClick={()=>{setSelected(item);setAmount(String(item.amount));setKind(item.kind);setNote("");setError("");}}>Review request</button>}</article>)}
  {selected&&<div className="space-y-4 rounded-lg border-2 border-primary p-4"><h3 className="font-semibold">Review {selected.orderNumber}</h3><p className="text-sm">Approve up to {formatPrice(selected.amount,"ugx")}. The original payment and exact payer number will be verified with IOTEC.</p><div className="grid gap-4 sm:grid-cols-2"><label className="text-sm">Approved amount (UGX)<input type="number" min={500} max={selected.amount} step={1} className={field} value={amount} onChange={e=>setAmount(e.target.value)}/></label><label className="text-sm">Refund type<select className={field} value={kind} onChange={e=>setKind(e.target.value)}><option value="cancellation">Cancellation before delivery</option><option value="return_settlement">Return / complaint after delivery</option></select></label></div><label className="block text-sm">Response and agreed settlement — visible to the customer<textarea maxLength={4000} className={field} placeholder="Explain the approval, agreed amount and return terms, or why you are declining." value={note} onChange={e=>setNote(e.target.value)}/></label><div className="flex flex-wrap gap-3"><button disabled={busy||note.trim().length<5||!Number.isInteger(Number(amount))||Number(amount)<500||Number(amount)>selected.amount} className="rounded-md bg-primary px-4 py-2 text-primary-foreground disabled:opacity-50" onClick={()=>void decide("approve")}>Approve and prepare refund</button><button disabled={busy||note.trim().length<5} className="rounded-md border px-4 py-2 disabled:opacity-50" onClick={()=>void decide("decline")}>Decline request</button><button disabled={busy} className="underline" onClick={()=>setSelected(null)}>Close review</button></div></div>}
 </section>;
}
