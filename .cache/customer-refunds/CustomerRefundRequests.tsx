"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { formatDate, formatPrice } from "@/lib/utils";
import { refundRequestLabel, type CustomerRefundRequest } from "@/lib/orders/refund-requests";

type Payment = {transactionId:string;maskedPhone:string;available:number};
type RequestData = {items:CustomerRefundRequest[];payments:Payment[];delivered:boolean;hasOpenRequest:boolean};

export function CustomerRefundRequests({orderId}:{orderId:string}) {
 const router=useRouter();
 const [data,setData]=useState<RequestData|null>(null);
 const [hidden,setHidden]=useState(false);
 const [error,setError]=useState("");
 const [message,setMessage]=useState("");
 const [busy,setBusy]=useState(false);
 const [form,setForm]=useState({transactionId:"",customerName:"",phone:"",amount:"",reason:"",details:"",receiptReference:""});
 const load=useCallback(async()=>{
  try {
   const res=await fetch(`/api/orders/${encodeURIComponent(orderId)}/refund-requests`,{cache:"no-store"});
   if(res.status===404||res.status===403){setHidden(true);return;}
   const body=await res.json();if(!res.ok)throw new Error(body.error||"Could not load refund requests");
   setHidden(false);setData(body);
   setForm(old=>old.transactionId?old:{...old,transactionId:body.payments?.[0]?.transactionId||""});
  }catch(e){setError(e instanceof Error?e.message:"Could not load refund requests");}
 },[orderId]);
 useEffect(()=>{void load();},[load]);
 async function submit(e:React.FormEvent) {
  e.preventDefault();setBusy(true);setError("");setMessage("");
  try {
   const res=await fetch(`/api/orders/${encodeURIComponent(orderId)}/refund-requests`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({...form,amount:Number(form.amount)})});
   const body=await res.json();if(!res.ok)throw new Error(body.error||"Could not submit your request");
   setMessage("Your refund request was sent for review. You can track the decision and payment below.");
   setForm(old=>({...old,phone:"",amount:"",reason:"",details:"",receiptReference:""}));
   await load();router.refresh();
  }catch(e){setError(e instanceof Error?e.message:"Could not submit your request");}finally{setBusy(false);}
 }
 if(hidden)return null;
 if(!data&&!error)return <p className="text-sm text-muted-foreground">Loading refund options…</p>;
 if(data&&data.payments.length===0&&data.items.length===0&&!error)return null;
 const selected=data?.payments.find(p=>p.transactionId===form.transactionId);
 const field="mt-1 w-full rounded-md border bg-background px-3 py-2";
 return <section className="space-y-4 rounded-lg border bg-background p-6" id="refund-requests">
  <div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-lg font-semibold">Refunds and returns</h2><button className="text-sm underline" type="button" disabled={busy} onClick={()=>{setError("");void load();router.refresh();}}>Refresh status</button></div>
  {error&&<p role="alert" className="text-sm text-red-600">{error}</p>}
  {message&&<p role="status" className="rounded-md border p-3 text-sm">{message}</p>}
  {data?.items.map(item=><article key={item.id} className="space-y-2 rounded-lg border p-4 text-sm"><div className="flex flex-wrap justify-between gap-2"><strong>{refundRequestLabel(item)}</strong><span>{formatDate(item.createdAt,"datetime")}</span></div><p>Requested: {formatPrice(item.amount,"ugx")}{item.approvedAmount>0&&` · Approved: ${formatPrice(item.approvedAmount,"ugx")}`}</p><p>{item.reason}</p>{item.details&&<p className="whitespace-pre-wrap text-muted-foreground">{item.details}</p>}<p>Original payer: {item.maskedPhone}</p>{item.receiptReference&&<p>Your receipt reference: {item.receiptReference}</p>}{item.reviewNote&&<p className="whitespace-pre-wrap rounded-md bg-muted p-3">Admin response: {item.reviewNote}</p>}{item.refundTransactionId&&<p className="break-all">Refund transaction: {item.refundTransactionId}</p>}<p className="break-all text-xs text-muted-foreground">Request: {item.id}</p></article>)}
  {data?.hasOpenRequest?<p className="text-sm text-muted-foreground">An active refund request is already being handled for this order. Please wait for the review or refund confirmation.</p>:data&&data.payments.length>0?<form onSubmit={submit} className="space-y-4">
   <h3 className="font-semibold">Request a refund</h3><p className="text-sm text-muted-foreground">{data.delivered?"Tell us about the product issue and the return or settlement you are requesting.":"Request cancellation and a full or partial refund before delivery."} Submitting a request sends it for review; it does not cancel the order or send money automatically.</p>
   <label className="block text-sm">Original IOTEC transaction<select required className={field} value={form.transactionId} onChange={e=>setForm({...form,transactionId:e.target.value,amount:""})}>{data.payments.map(payment=><option value={payment.transactionId} key={payment.transactionId}>{payment.transactionId} · up to {formatPrice(payment.available,"ugx")}</option>)}</select></label>
   <div className="grid gap-4 sm:grid-cols-2"><label className="text-sm">Your name<input required minLength={2} maxLength={150} autoComplete="name" className={field} value={form.customerName} onChange={e=>setForm({...form,customerName:e.target.value})}/></label><label className="text-sm">Original payer phone number<input required type="tel" autoComplete="tel" placeholder="0771234567" className={field} value={form.phone} onChange={e=>setForm({...form,phone:e.target.value})}/><span className="text-xs text-muted-foreground">The refund can only go back to the original payer{selected?.maskedPhone?` (${selected.maskedPhone})`:""}.</span></label><label className="text-sm">Amount requested (UGX)<input required type="number" min={500} max={selected?.available} step={1} className={field} value={form.amount} onChange={e=>setForm({...form,amount:e.target.value})}/></label><label className="text-sm">Mobile money receipt reference (optional)<input maxLength={150} placeholder="Reference from your payment message" className={field} value={form.receiptReference} onChange={e=>setForm({...form,receiptReference:e.target.value})}/></label></div>
   <label className="block text-sm">Reason for requesting a refund<input required minLength={5} maxLength={2000} className={field} value={form.reason} onChange={e=>setForm({...form,reason:e.target.value})}/></label>
   <label className="block text-sm">Product and return details<textarea required={data.delivered} minLength={data.delivered?10:undefined} maxLength={4000} className={field} placeholder="Which item is affected? Describe the issue, product condition and the resolution you would like." value={form.details} onChange={e=>setForm({...form,details:e.target.value})}/></label>
   <button disabled={busy} className="rounded-md bg-primary px-4 py-2 text-primary-foreground disabled:opacity-50">{busy?"Submitting…":"Send refund request"}</button>
  </form>:data?.items.length?<p className="text-sm text-muted-foreground">There is no remaining payment balance currently available for a new request.</p>:null}
 </section>;
}
