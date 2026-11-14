from pathlib import Path
root=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
p=root/'lib/ai/tools/get-my-orders.ts';s=p.read_text(encoding='utf-8');start=s.index('export function createGetMyOrdersTool(')
s=s[:start]+'''export async function getMyOrders(status = ""): Promise<GetMyOrdersResult> {
  const response = await authedFetch("/orders/my");
  if (response.status === 401) return {found:false,message:"Sign in to view your orders.",orders:[],totalOrders:0,isAuthenticated:false};
  if (!response.ok) return {found:false,message:"Order history is temporarily unavailable.",orders:[],totalOrders:0,isAuthenticated:true};
  const data: ApiOrder[] = await response.json();
  const orders = data.filter(order=>!status || order.status===status).slice(0,20).map(order=>({
    id:order.id,orderNumber:order.orderNumber??null,total:order.total??null,totalFormatted:formatPrice(order.total??0),
    status:order.status??null,statusDisplay:String(order.status||"pending").replaceAll("_"," "),
    itemCount:(order.items??[]).reduce((sum,item)=>sum+item.quantity,0),itemNames:(order.items??[]).map(item=>item.productName),
    itemImages:(order.items??[]).map(item=>item.imageUrl).filter((url):url is string=>!!url),
    createdAt:order.createdAt??null,orderUrl:`/orders/${encodeURIComponent(order.id)}`,
  }));
  return {found:orders.length>0,message:orders.length?"Your matching orders.":"No matching orders found.",orders,totalOrders:data.length,isAuthenticated:true};
}
export function createGetMyOrdersTool(_userId:string|null){
  return tool({description:"Read the signed-in customer's own orders. No account ID may be supplied.",inputSchema:schema,execute:async({status})=>getMyOrders(status)});
}
'''
s='import { authedFetch } from "@/lib/api/proxy";\nimport type { ApiOrder } from "@/lib/api/types";\nimport { formatPrice } from "@/lib/utils";\n'+s;put('lib/ai/tools/get-my-orders.ts',s)
put('lib/ai/tools/search-products.ts','''import{tool}from"ai";import{z}from"zod";
import{apiGet}from"@/lib/api/client";import type{ApiProduct}from"@/lib/api/types";
import{formatPrice}from"@/lib/utils";import{getStockStatus,getStockMessage}from"@/lib/constants/stock";
const schema=z.object({query:z.string().optional().default(""),category:z.string().optional().default(""),team:z.string().optional().default(""),kitType:z.string().optional().default(""),size:z.string().optional().default(""),minPrice:z.number().nonnegative().optional().default(0),maxPrice:z.number().nonnegative().optional().default(0)});
type Filters=z.infer<typeof schema>;
type Product=ApiProduct&{variants?:{size?:string;stock?:number}[]};
export async function searchProducts(filters:Partial<Filters>){
 const f=schema.parse(filters);const params=new URLSearchParams();
 const query=[f.query,f.team,f.kitType].filter(Boolean).join(" ");if(query)params.set("q",query);
 if(f.category)params.set("category",f.category==="footwear"?"shoes":f.category==="fan-gear"?"accessories":f.category);
 try{
  const rows=await apiGet<Product[]>(`/products?${params}`);
  const selected=rows.filter(p=>(!f.minPrice||p.price>=f.minPrice)&&(!f.maxPrice||p.price<=f.maxPrice)&&(!f.size||(p.variants??[]).some(v=>v.size?.trim().toLowerCase()===f.size.trim().toLowerCase())));
  const products=selected.slice(0,12).map(p=>{
   const stock=f.size?(p.variants??[]).filter(v=>v.size?.trim().toLowerCase()===f.size.trim().toLowerCase()).reduce((sum,v)=>sum+(v.stock??0),0):p.stock;
   return {id:p.id,name:p.name,slug:p.slug,description:p.description??null,price:p.price,priceFormatted:formatPrice(p.price),category:p.category??null,categorySlug:p.category??null,material:null,color:null,dimensions:null,stockCount:stock,stockStatus:getStockStatus(stock),stockMessage:getStockMessage(stock),featured:p.isFeatured??false,assemblyRequired:false,imageUrl:p.images?.[0]??null,productUrl:`/products/${encodeURIComponent(p.slug)}`};
  });
  return {found:products.length>0,message:products.length?`Found ${products.length} matching products.`:"No products match these filters.",products,totalResults:selected.length,filters:f};
 }catch{return {found:false,message:"The product catalog is temporarily unavailable. Please try again.",products:[],filters:f}}
}
export const searchProductsTool=tool({description:"Search live merchandise prices and variant stock by category, team, kit, size and budget.",inputSchema:schema,execute:searchProducts});
''')
p=root/'lib/ai/shopping-agent.ts';s=p.read_text(encoding='utf-8').replace('if (getMyOrdersTool) {','if (isAuthenticated) {').replace('gateway("anthropic/claude-sonnet-4.5")','gateway(process.env.SHOPPING_AI_MODEL || "anthropic/claude-sonnet-4.5")');put('lib/ai/shopping-agent.ts',s)
put('app/api/chat/route.ts','''import{createAgentUIStreamResponse,type UIMessage}from"ai";
import{createShoppingAgent}from"@/lib/ai/shopping-agent";
import{authedFetch}from"@/lib/api/proxy";
export async function POST(request:Request){
 let body:unknown;try{body=await request.json()}catch{return Response.json({error:"Invalid chat request"},{status:400})}
 const messages=(body as {messages?:UIMessage[]})?.messages;
 if(!Array.isArray(messages)||!messages.length||messages.length>40||JSON.stringify(messages).length>100000)return Response.json({error:"Chat history is missing or too large"},{status:400});
 if(!process.env.AI_GATEWAY_API_KEY)return Response.json({error:"The shopping assistant is not configured yet."},{status:503});
 let userId:string|null=null;
 try{const me=await authedFetch("/me");if(me.ok){const user=await me.json();userId=user.id??null}}catch{}
 const agent=createShoppingAgent({userId});
 return createAgentUIStreamResponse({agent,messages});
}
''')
# Ignore late quotes for an earlier location, including a restored checkout draft.
p=root/'app/(app)/checkout/CheckoutClient.tsx';s=p.read_text(encoding='utf-8').replace('useMemo, useState, useTransition','useMemo, useRef, useState, useTransition')
s=s.replace('  const [isCalculating, startTransition]', '  const quoteVersion = useRef(0);\n  const [isCalculating, startTransition]',1)
s=s.replace('''          startTransition(async () => {
            const result = await getDeliveryQuote({ lat, lng });''','''          const version = ++quoteVersion.current;
          startTransition(async () => {
            const result = await getDeliveryQuote({ lat, lng });
            if (version !== quoteVersion.current) return;''',1)
s=s.replace('''    startTransition(async () => {
      const result = await getDeliveryQuote({ lat, lng });''','''    const version = ++quoteVersion.current;
    startTransition(async () => {
      const result = await getDeliveryQuote({ lat, lng });
      if (version !== quoteVersion.current) return;''',1);put('app/(app)/checkout/CheckoutClient.tsx',s)

put('tests/shopping-tools.test.ts','''import"./setup-server-only";
import assert from"node:assert/strict";import{afterEach,mock,test}from"node:test";
import{getMyOrders}from"../lib/ai/tools/get-my-orders";import{searchProducts}from"../lib/ai/tools/search-products";import{AUTH_COOKIE_NAME}from"../lib/auth/constants";import{mockFetch,json,setRequestCookies}from"./helpers";
process.env.AFRIGOALS_API_URL="https://api.afrigoals-store.com";afterEach(()=>mock.restoreAll());
test("shopping order tool does not fetch orders for a guest",async()=>{setRequestCookies({});const calls=mockFetch(()=>json([]));const result=await getMyOrders();assert.equal(result.isAuthenticated,false);assert.equal(calls.length,0)});
test("shopping order tool uses the customer's session and strips delivery details",async()=>{setRequestCookies({[AUTH_COOKIE_NAME]:"owner-token"});const calls=mockFetch(()=>json([{id:"own",orderNumber:"ORD1",total:800,status:"pending_payment",createdAt:"2026-11-07",deliveryAddress:"private address",email:"private@email",items:[]}]));const result=await getMyOrders();assert.equal(calls[0].url,"https://api.afrigoals-store.com/api/v1/orders/my");assert.equal(result.orders[0].id,"own");assert.equal(JSON.stringify(result).includes("private address"),false)});
test("shopping product tool uses the correct API prefix and selected size stock",async()=>{const calls=mockFetch(()=>json([{id:"p1",slug:"kit",name:"Kit",price:500,stock:40,images:[],variants:[{size:"M",stock:20},{size:"S",stock:20}]}]));const result=await searchProducts({size:"M",maxPrice:800});assert.equal(result.products[0].stockCount,20);assert.ok(calls[0].url.includes("/api/v1/products"))});
''')
import json
p=root/'package.json';data=json.loads(p.read_text(encoding='utf-8'));data['scripts']['test']+=' tests/shopping-tools.test.ts';put('package.json',json.dumps(data,indent=2)+'\n')
print('Connected the shopping assistant to scoped real orders and live product filters; prevented stale delivery quotes.')
