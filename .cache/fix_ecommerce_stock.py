from pathlib import Path
import json

root = Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(name, text):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding='utf-8')

path = root / 'lib/catalog/query.ts'
text = path.read_text(encoding='utf-8')
old = '''  let products: ApiProduct[] = [];

  try {
    products = await apiGet<ApiProduct[]>(`/products?${qs}`);
  } catch {
    products = [];
  }
'''
assert old in text
text = text.replace(old, '''  // An unavailable inventory service is not an empty catalog.
  const products = await apiGet<ApiProduct[]>(`/products?${qs}`);
''', 1)
put('lib/catalog/query.ts', text)

oldhook = (root / 'lib/hooks/useCartStock.ts').read_text(encoding='utf-8')
start = oldhook.index('export interface StockInfo')
end = oldhook.index('interface UseCartStockReturn')
types = oldhook[start:end]
put('lib/cart/stock.ts', '''import type { CatalogProduct } from "@/lib/catalog/types";
import { getCartItemKey, type CartItem } from "@/lib/store/cart-store";

''' + types + '''function normalize(value?: string | null) {
  return String(value || "").trim().toLowerCase();
}

export function findVariantStock(product: CatalogProduct, item: CartItem): number {
  const variants = product.variants ?? [];
  if (!variants.length) return Math.max(0, Number(product.stock) || 0);
  const key = normalize(item.variantKey);
  const size = normalize(item.selectedSize);
  const color = normalize(item.selectedColor);
  // Resolve an exact ID first. A key-only selection must not match row one.
  const exact = key ? variants.find(v => normalize(v._key || v.id) === key) : undefined;
  const selected = exact ?? ((size || color) ? variants.find(v =>
    (!size || normalize(v.size) === size) &&
    (!color || normalize(v.color || v.colour) === color),
  ) : undefined);
  return Math.max(0, Number(selected?.stock) || 0);
}

export function buildStockMap(products: CatalogProduct[], items: CartItem[]): StockMap {
  return new Map(items.map(item => {
    const product = products.find(p => p._id === item.productId);
    const currentStock = product ? findVariantStock(product, item) : 0;
    return [getCartItemKey(item), {
      productId: item.productId,
      variantKey: item.variantKey ?? null,
      selectedSize: item.selectedSize ?? null,
      selectedColor: item.selectedColor ?? null,
      currentStock,
      isOutOfStock: currentStock <= 0,
      exceedsStock: item.quantity > currentStock,
      availableQuantity: Math.min(item.quantity, currentStock),
    }];
  }));
}
''')
put('lib/hooks/useCartStock.ts', '''"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { CatalogProduct } from "@/lib/catalog/types";
import type { CartItem } from "@/lib/store/cart-store";
import { buildStockMap, type StockMap } from "@/lib/cart/stock";
export type { StockInfo, StockMap } from "@/lib/cart/stock";

export function useCartStock(items: CartItem[]) {
  const [stockMap, setStockMap] = useState<StockMap>(new Map());
  const [isLoading, setIsLoading] = useState(items.length > 0);
  const [error, setError] = useState<string | null>(null);
  const activeRequest = useRef<AbortController | null>(null);

  const refetch = useCallback(async () => {
    activeRequest.current?.abort();
    const controller = new AbortController();
    activeRequest.current = controller;
    setStockMap(new Map());
    setError(null);
    setIsLoading(items.length > 0);
    if (!items.length) return;

    try {
      const response = await fetch("/api/cart/stock", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: [...new Set(items.map(item => item.productId))] }),
        cache: "no-store",
        signal: controller.signal,
      });
      if (!response.ok) throw new Error(`Stock request failed: ${response.status}`);
      const products: unknown = await response.json();
      if (!Array.isArray(products)) throw new Error("Invalid stock response");
      if (!controller.signal.aborted) {
        setStockMap(buildStockMap(products as CatalogProduct[], items));
      }
    } catch (cause) {
      if (!controller.signal.aborted) {
        console.error("Failed to verify stock:", cause);
        setError("We couldn't verify stock right now. Please try again.");
      }
    } finally {
      if (!controller.signal.aborted) setIsLoading(false);
    }
  }, [items]);

  useEffect(() => {
    void refetch();
    const refresh = () => { if (document.visibilityState === "visible") void refetch(); };
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      activeRequest.current?.abort();
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [refetch]);

  const hasStockIssues = [...stockMap.values()].some(info => info.isOutOfStock || info.exceedsStock);
  return { stockMap, isLoading, error, hasStockIssues, refetch };
}
''')
put('components/app/StockVerificationNotice.tsx', '''"use client";

export function StockVerificationNotice({ error, retry }: { error: string | null; retry: () => void }) {
  if (!error) return null;
  return (
    <div role="alert" className="m-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
      <p>{error}</p>
      <button type="button" onClick={retry} className="mt-2 font-semibold underline underline-offset-2">Retry stock check</button>
    </div>
  );
}
''')
for name in ('app/(app)/checkout/CheckoutClient.tsx', 'components/app/CartPageClient.tsx', 'components/app/CartSheet.tsx'):
    text = (root / name).read_text(encoding='utf-8')
    text = text.replace('import { useCartStock }', 'import { StockVerificationNotice } from "@/components/app/StockVerificationNotice";\nimport { useCartStock }', 1)
    text = text.replace('const { stockMap, isLoading, hasStockIssues }', 'const { stockMap, isLoading, hasStockIssues, error: stockError, refetch: retryStock }', 1)
    marker = '{hasStockIssues && !isLoading'
    idx = text.index(marker)
    text = text[:idx] + '<StockVerificationNotice error={stockError} retry={retryStock} />\n              ' + text[idx:]
    if name.startswith('app/'):
        text = text.replace('const isCheckoutDisabled =\n    hasStockIssues ||', 'const isCheckoutDisabled =\n    Boolean(stockError) ||\n    hasStockIssues ||', 1)
    put(name, text)

put('tests/cart-stock.test.ts', '''import "./setup-server-only";
import assert from "node:assert/strict";
import { afterEach, mock, test } from "node:test";
import { POST } from "../app/api/cart/stock/route";
import { buildStockMap, findVariantStock } from "../lib/cart/stock";
import { getCartItemKey, type CartItem } from "../lib/store/cart-store";
import type { CatalogProduct } from "../lib/catalog/types";
import { mockFetch, json } from "./helpers";

afterEach(() => mock.restoreAll());
const item: CartItem = { productId: "p1", name: "Uganda Cranes", price: 90000, quantity: 1, selectedSize: "M" };
const product: CatalogProduct = { _id: "p1", name: item.name, slug: "uganda", currency: "UGX", price: 90000, stock: 30, variants: [
  { _key: "small", size: "S", stock: 10 },
  { _key: "medium", size: "M", stock: 20 },
] };

test("size M remains available when the live inventory contains 20", () => {
  const stock = buildStockMap([product], [item]).get(getCartItemKey(item));
  assert.equal(stock?.currentStock, 20);
  assert.equal(stock?.isOutOfStock, false);
  assert.equal(stock?.exceedsStock, false);
});
test("a key-only selection resolves its exact variant instead of the first row", () => {
  assert.equal(findVariantStock(product, { ...item, selectedSize: null, variantKey: "medium" }), 20);
  assert.equal(findVariantStock(product, { ...item, selectedSize: null, variantKey: "missing" }), 0);
});
test("old generated keys can match the selected size after catalog changes", () => {
  assert.equal(findVariantStock(product, { ...item, variantKey: "old-key", selectedSize: " m " }), 20);
});
test("real zero stock and insufficient quantities still block checkout", () => {
  const empty = { ...product, variants: [{ size: "M", stock: 0 }] };
  assert.equal(buildStockMap([empty], [item]).get(getCartItemKey(item))?.isOutOfStock, true);
  const many = { ...item, quantity: 21 };
  assert.equal(buildStockMap([product], [many]).get(getCartItemKey(many))?.exceedsStock, true);
});
test("inventory outages are errors, never a successful empty stock list", async () => {
  mockFetch(() => json({ error: "Inventory unavailable" }, 503));
  const response = await POST(new Request("http://localhost/api/cart/stock", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ids: ["p1"] }),
  }));
  assert.equal(response.status, 500);
  assert.deepEqual(await response.json(), { error: "Failed to load cart stock" });
});
test("stock route preserves live variant quantities", async () => {
  mockFetch(() => json([{ ...product, id: "p1" }]));
  const response = await POST(new Request("http://localhost/api/cart/stock", {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ids: ["p1"] }),
  }));
  assert.equal(response.status, 200);
  const products = await response.json();
  assert.equal(findVariantStock(products[0], item), 20);
});
''')
path = root / 'package.json'
package = json.loads(path.read_text(encoding='utf-8'))
package['scripts']['test'] += ' tests/cart-stock.test.ts'
put('package.json', json.dumps(package, indent=2) + '\n')
print('Fixed inventory error propagation, stock matching and retry UI; added regression tests.')
