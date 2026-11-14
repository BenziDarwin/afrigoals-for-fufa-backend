import { NextResponse } from "next/server";
import { authedFetch } from "@/lib/api/proxy";
export async function GET(request:Request){
 const response=await authedFetch(`/admin/refund-requests${new URL(request.url).search}`);
 return new NextResponse(await response.text(),{status:response.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}});
}
