import { NextResponse } from "next/server";
import { authedFetch } from "@/lib/api/proxy";
async function forward(request:Request,ctx:{params:Promise<{id:string}>},method:string){
 const {id}=await ctx.params;
 const response=await authedFetch(`/orders/${encodeURIComponent(id)}/refund-requests`,method==="GET"?{}:{method,headers:{"Content-Type":"application/json"},body:await request.text()});
 return new NextResponse(await response.text(),{status:response.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}});
}
export async function GET(request:Request,ctx:{params:Promise<{id:string}>}){return forward(request,ctx,"GET");}
export async function POST(request:Request,ctx:{params:Promise<{id:string}>}){return forward(request,ctx,"POST");}
