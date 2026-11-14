import { NextResponse } from "next/server";
import { authedFetch } from "@/lib/api/proxy";
export const maxDuration = 120;
async function proxy(request:Request,method:string) {
 const url=new URL(request.url);
 const response=await authedFetch(`/admin/refunds${url.search}`,method==="GET"?{}:{method,headers:{"Content-Type":"application/json"},body:await request.text()});
 return new NextResponse(await response.text(),{status:response.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}});
}
export async function GET(request:Request){return proxy(request,"GET");}
export async function POST(request:Request){return proxy(request,"POST");}
