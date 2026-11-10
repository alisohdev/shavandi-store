// Deterministic public API used only by the Playwright webServer configuration.
const product = {id:"live-1",slug:"live-shirt",name:"پیراهن کاتالوگ",description:"توضیح محصول منتشرشده",category_id:"cat-1",regular_price_toman:150000,sale_price_toman:null,variants:[{id:"variant-1",size:"M",color:"آبی",price:{regular_toman:175000,sale_toman:160000}}],images:[{id:"image-1",asset_ref:"/images/cotton-top.webp",sort_order:0,alt_text:"تصویر پیراهن"}]};
Bun.serve({hostname:"127.0.0.1",port:8089,fetch(request){
 const url=new URL(request.url);
 if(url.pathname==="/api/v1/categories") return Response.json({categories:[{id:"cat-1",name:"پوشاک زنده",slug:"live"},{id:"cat-2",name:"بدون محصول",slug:"empty"},{id:"cat-3",name:"خطای سرویس",slug:"unavailable"}]});
 if(url.pathname==="/api/v1/products") {
  if(url.searchParams.get("category")==="unavailable") return Response.json({}, {status:503});
  if(url.searchParams.get("category")==="empty" || url.searchParams.has("cursor")) return Response.json({products:[]});
  return Response.json({products:[product],next_cursor:"next-page"});
 }
 if(url.pathname==="/api/v1/products/live-shirt") return Response.json({product});
 return Response.json({error:{code:"not_found"}},{status:404});
}});

