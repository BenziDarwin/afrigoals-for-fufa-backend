from pathlib import Path
import re

root = Path('F:/Afrigoals/Afrigoals_ecommerce_api')
path = root / 'routes/orders.go'
text = path.read_text(encoding='utf-8')
start = text.index('func createOrderFromFlatDelivery(')
end = text.index('\ntype orderVariantSelection', start)
helper = text[start:end]
helper = helper.replace('models.OrderStatusEvent, error) {', 'models.OrderStatusEvent, *apiFailure) {', 1)
pattern = r'c\.Status\(([^\n]+?)\)\.JSON\(fiber\.Map\{\s*"error":\s*([^\n]+?),?\s*\}\)'
helper, count = re.subn(pattern, lambda m: '&apiFailure{status: ' + m[1] + ', msg: ' + m[2].rstrip(',') + '}', helper)
assert count == 15, f'Unexpected failure branch count: {count}'
helper = helper.replace('tx.Rollback()\n\t\t}', 'tx.Rollback()\n\t\t\tpanic(r)\n\t\t}', 1)
text = text[:start] + helper + text[end:]
old = 'if errResp != nil {\n\t\t\treturn errResp\n\t\t}'
assert text.count(old) == 2
text = text.replace(old, 'if errResp != nil {\n\t\t\treturn c.Status(errResp.status).JSON(fiber.Map{"error": errResp.msg})\n\t\t}')
path.write_text(text, encoding='utf-8')
print(f'Updated {count} order failure branches and both checkout handlers')
