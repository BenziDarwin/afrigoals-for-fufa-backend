import { NextResponse } from "next/server";
import { authedFetch } from "@/lib/api/proxy";
export const maxDuration = 120;
export async function POST(request:Request,ctx:{params:Promise<{id:string;action:string}>}) {
 const {id,action}=await ctx.params;
 if(!["submit","reconcile","reject"].includes(action))return NextResponse.json({error:"Unknown refund action"},{status:404});
 const response=await authedFetch(`/admin/refunds/${encodeURIComponent(id)}/${action}`,{method:"POST",headers:{"Content-Type":"application/json"},body:await request.text()});
 return new NextResponse(await response.text(),{status:response.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}});
}
